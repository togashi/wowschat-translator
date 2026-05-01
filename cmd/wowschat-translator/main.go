package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/kardianos/service"
	"github.com/togashi/wowschat-translator/internal/config"
	"github.com/togashi/wowschat-translator/internal/server"
	"github.com/togashi/wowschat-translator/internal/translator"
	"gopkg.in/yaml.v3"
)

var svcConfig = &service.Config{
	Name:        "WoWSChatTranslatorService",
	DisplayName: "WoWS Chat Translator Service",
	Description: "World of Warships in-game chat translation service",
}

type program struct {
	srv *server.Server
}

func (p *program) Start(_ service.Service) error {
	go func() {
		if err := p.srv.Start(); err != nil {
			log.Printf("server error: %v", err)
		}
	}()
	return nil
}

func (p *program) Stop(_ service.Service) error {
	return p.srv.Shutdown(context.Background())
}

func main() {
	var (
		configFile      = flag.String("config", "", "path to config file (default search: user config dir then current directory)")
		apiKey          = flag.String("api-key", "", "DeepL API key")
		targetLang      = flag.String("target-lang", "", "target language code (e.g. JA, EN-US)")
		outputFmt       = flag.String("output-format", "", "translated output format (e.g. ({DetectedSourceLanguage}) {TranslatedText})")
		engine          = flag.String("translation-engine", "", "translation engine: deepl, gpt, claude, or gemini")
		openAIKey       = flag.String("openai-api-key", "", "OpenAI API key for GPT translation")
		openAIModel     = flag.String("openai-model", "", "OpenAI model ID for GPT translation (e.g. gpt-5.4-mini)")
		openAIPrompt    = flag.String("openai-prompt-file", "", "optional file path for GPT system prompt override")
		openAITemp      = flag.String("openai-temperature", "", "OpenAI sampling temperature for GPT translation (e.g. 0.2)")
		anthropicKey    = flag.String("anthropic-api-key", "", "Anthropic API key for Claude translation")
		anthropicModel  = flag.String("anthropic-model", "", "Anthropic model ID for Claude translation (e.g. claude-haiku-4-5-20251001)")
		anthropicPrompt = flag.String("anthropic-prompt-file", "", "optional file path for Claude system prompt override")
		anthropicTemp   = flag.String("anthropic-temperature", "", "Anthropic sampling temperature for Claude translation (e.g. 0.2)")
		geminiKey       = flag.String("gemini-api-key", "", "Google AI API key for Gemini translation")
		geminiModel     = flag.String("gemini-model", "", "Gemini model ID for Gemini translation (e.g. gemini-2.5-flash)")
		geminiPrompt    = flag.String("gemini-prompt-file", "", "optional file path for Gemini system prompt override")
		geminiTemp      = flag.String("gemini-temperature", "", "Gemini sampling temperature for Gemini translation (e.g. 0.2)")
		debug           = flag.String("debug", "", "enable verbose debug logging (true/false)")
		traceLogFile    = flag.String("trace-log-file", "", "path to JSONL trace log file; if set, trace logging is enabled")
		initConfig      = flag.Bool("init-config", false, "create default config.yaml and exit")
		dumpConfig      = flag.Bool("dump-config", false, "dump loaded and resolved config as YAML (with masked API keys), then exit")
	)
	flag.Parse()

	if *initConfig {
		path, created, err := config.EnsureDefaultConfig(*configFile)
		if err != nil {
			log.Fatalf("init config: %v", err)
		}
		if created {
			log.Printf("created default config: %s", path)
		} else {
			log.Printf("config already exists: %s", path)
		}
		return
	}

	// Service management commands (install / uninstall / start / stop)
	if args := flag.Args(); len(args) > 0 {
		runServiceCommand(args[0])
		return
	}

	if path, created, err := config.EnsureDefaultConfig(*configFile); err != nil {
		log.Printf("warning: could not create default config: %v", err)
	} else if created {
		log.Printf("created default config: %s", path)
		log.Printf("edit API key settings in config.yaml and run again")
	}

	resolvedConfigPath := config.ResolveConfigPath(*configFile)

	cfg, err := config.Load(
		*configFile,
		*apiKey,
		*targetLang,
		*outputFmt,
		*engine,
		*openAIKey,
		*openAIModel,
		*openAIPrompt,
		*openAITemp,
		*anthropicKey,
		*anthropicModel,
		*anthropicPrompt,
		*anthropicTemp,
		*geminiKey,
		*geminiModel,
		*geminiPrompt,
		*geminiTemp,
		*debug,
		*traceLogFile,
	)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if *dumpConfig {
		if absPath, absErr := filepath.Abs(resolvedConfigPath); absErr == nil {
			resolvedConfigPath = absPath
		}
		if _, err := fmt.Fprintf(os.Stdout, "loaded_config_file: %s\n", resolvedConfigPath); err != nil {
			log.Fatalf("dump config path: %v", err)
		}

		dump := maskedConfigForDump(cfg)
		enc := yaml.NewEncoder(os.Stdout)
		enc.SetIndent(2)
		if err := enc.Encode(dump); err != nil {
			log.Fatalf("dump config: %v", err)
		}
		if err := enc.Close(); err != nil {
			log.Fatalf("dump config close: %v", err)
		}
		return
	}

	var tr translator.Translator
	switch strings.ToLower(cfg.TranslationEngine) {
	case "deepl":
		if cfg.DeepLAPIKey == "" {
			log.Fatal("DeepL API key is required for translation_engine=deepl.\n" +
				"  Set via --api-key flag, WOWSCHAT_API_KEY env var, or deepl_api_key in config.yaml")
		}
		tr = translator.NewDeepLTranslator(cfg.DeepLAPIKey, cfg.OutputFormat, cfg.Debug)
	case "gpt":
		if cfg.OpenAIAPIKey == "" {
			log.Fatal("OpenAI API key is required for translation_engine=gpt.\n" +
				"  Set via --openai-api-key flag, WOWSCHAT_OPENAI_API_KEY env var, or openai_api_key in config.yaml")
		}
		tr = translator.NewGPTTranslator(
			cfg.OpenAIAPIKey,
			cfg.OpenAIModel,
			cfg.OpenAIPromptFile,
			cfg.OpenAITemperature,
			cfg.OutputFormat,
			cfg.Passthrough,
			cfg.Glossary,
			cfg.Expand,
			cfg.Debug,
		)
	case "claude":
		if cfg.AnthropicAPIKey == "" {
			log.Fatal("Anthropic API key is required for translation_engine=claude.\n" +
				"  Set via --anthropic-api-key flag, WOWSCHAT_ANTHROPIC_API_KEY env var, or anthropic_api_key in config.yaml")
		}
		tr = translator.NewClaudeTranslator(
			cfg.AnthropicAPIKey,
			cfg.AnthropicModel,
			cfg.AnthropicPromptFile,
			cfg.AnthropicTemperature,
			cfg.OutputFormat,
			cfg.Passthrough,
			cfg.Glossary,
			cfg.Expand,
			cfg.Debug,
		)
	case "gemini":
		if cfg.GeminiAPIKey == "" {
			log.Fatal("Google AI API key is required for translation_engine=gemini.\n" +
				"  Set via --gemini-api-key flag, WOWSCHAT_GEMINI_API_KEY env var, or gemini_api_key in config.yaml")
		}
		tr = translator.NewGeminiTranslator(
			cfg.GeminiAPIKey,
			cfg.GeminiModel,
			cfg.GeminiPromptFile,
			cfg.GeminiTemperature,
			cfg.OutputFormat,
			cfg.Passthrough,
			cfg.Glossary,
			cfg.Expand,
			cfg.Debug,
		)
	default:
		log.Fatalf("unsupported translation_engine %q (valid: deepl, gpt, claude, gemini)", cfg.TranslationEngine)
	}

	log.Printf("target language: %s", cfg.TargetLang)
	log.Printf("translation engine: %s", cfg.TranslationEngine)
	if cfg.Debug {
		log.Printf("debug logging: enabled")
	}

	if cfg.TraceLogFile != "" {
		if dir := filepath.Dir(cfg.TraceLogFile); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				log.Fatalf("trace log dir: %v", err)
			}
		}
		traceFile, err := os.OpenFile(cfg.TraceLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			log.Fatalf("trace log file: %v", err)
		}
		defer traceFile.Close()

		encoder := json.NewEncoder(traceFile)
		var mu sync.Mutex
		if traceSinkSetter, ok := tr.(translator.TraceSinkSetter); ok {
			traceSinkSetter.SetTraceSink(func(event translator.TranslatorTraceEvent) {
				mu.Lock()
				defer mu.Unlock()
				if err := encoder.Encode(event); err != nil {
					log.Printf("trace log write error: %v", err)
				}
			})
			log.Printf("trace logging: %s", cfg.TraceLogFile)
		} else {
			log.Printf("trace logging not supported by selected translator")
		}
	}

	srv := server.New(tr, cfg.TargetLang, cfg.ListenPort, cfg.EndpointPath)
	prg := &program{srv: srv}

	svc, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatalf("service init: %v", err)
	}

	if service.Interactive() {
		// Running interactively (not as a service)
		go func() {
			if err := srv.Start(); err != nil {
				log.Printf("server stopped: %v", err)
			}
		}()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit

		log.Println("shutting down...")
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Printf("shutdown: %v", err)
		}
	} else {
		// Running as a Windows service
		if err := svc.Run(); err != nil {
			log.Fatalf("service run: %v", err)
		}
	}
}

func maskedConfigForDump(cfg *config.Config) config.Config {
	masked := *cfg
	masked.DeepLAPIKey = maskSecret(masked.DeepLAPIKey)
	masked.OpenAIAPIKey = maskSecret(masked.OpenAIAPIKey)
	masked.AnthropicAPIKey = maskSecret(masked.AnthropicAPIKey)
	masked.GeminiAPIKey = maskSecret(masked.GeminiAPIKey)
	return masked
}

func maskSecret(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	return "***"
}

func runServiceCommand(cmd string) {
	svc, err := service.New(&program{}, svcConfig)
	if err != nil {
		log.Fatalf("service: %v", err)
	}

	switch cmd {
	case "install":
		err = svc.Install()
	case "uninstall":
		err = svc.Uninstall()
	case "start":
		err = svc.Start()
	case "stop":
		err = svc.Stop()
	default:
		log.Fatalf("unknown command %q  (valid: install / uninstall / start / stop)", cmd)
	}

	if err != nil {
		log.Fatalf("%s: %v", cmd, err)
	}
	log.Printf("service %s: OK", cmd)
}
