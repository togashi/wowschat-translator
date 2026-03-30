package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/kardianos/service"
	"github.com/togashi/wowschat-translator/internal/config"
	"github.com/togashi/wowschat-translator/internal/server"
	"github.com/togashi/wowschat-translator/internal/translator"
)

var svcConfig = &service.Config{
	Name:        "WoWSChatTranslator",
	DisplayName: "WoWS Chat Translator",
	Description: "World of Warships in-game chat translation service via DeepL",
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
		configFile   = flag.String("config", "", "path to config file (default: config.yaml next to executable)")
		apiKey       = flag.String("api-key", "", "DeepL API key")
		targetLang   = flag.String("target-lang", "", "target language code (e.g. JA, EN-US)")
		outputFmt    = flag.String("output-format", "", "translated output format (e.g. ({DetectedSourceLanguage}) {TranslatedText})")
		engine       = flag.String("translation-engine", "", "translation engine: deepl or gpt")
		openAIKey    = flag.String("openai-api-key", "", "OpenAI API key for GPT translation")
		openAIModel  = flag.String("openai-model", "", "OpenAI model ID for GPT translation (e.g. gpt-5.4-mini)")
		openAITemp   = flag.String("openai-temperature", "", "OpenAI sampling temperature for GPT translation (e.g. 0.2)")
		debug        = flag.String("debug", "", "enable verbose debug logging (true/false)")
		traceLogFile = flag.String("trace-log-file", "", "path to JSONL trace log file; if set, trace logging is enabled")
	)
	flag.Parse()

	// Service management commands (install / uninstall / start / stop)
	if args := flag.Args(); len(args) > 0 {
		runServiceCommand(args[0])
		return
	}

	cfg, err := config.Load(
		*configFile,
		*apiKey,
		*targetLang,
		*outputFmt,
		*engine,
		*openAIKey,
		*openAIModel,
		*openAITemp,
		*debug,
		*traceLogFile,
	)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	var tr translator.Translator
	switch strings.ToLower(cfg.TranslationEngine) {
	case "deepl":
		if cfg.DeepLAPIKey == "" {
			log.Fatal("DeepL API key is required for translation_engine=deepl.\n" +
				"  Set via --api-key flag, WOWSCHAT_API_KEY env var, or deepl_api_key (api_key fallback) in config.yaml")
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
			cfg.OpenAITemperature,
			cfg.OutputFormat,
			cfg.Passthrough,
			cfg.Glossary,
			cfg.Debug,
		)
	default:
		log.Fatalf("unsupported translation_engine %q (valid: deepl, gpt)", cfg.TranslationEngine)
	}

	log.Printf("target language: %s", cfg.TargetLang)
	log.Printf("translation engine: %s", cfg.TranslationEngine)
	if cfg.Debug {
		log.Printf("debug logging: enabled")
	}

	if cfg.TraceLogFile != "" {
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

	srv := server.New(tr, cfg.TargetLang)
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
