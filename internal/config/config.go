package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DeepLAPIKey       string            `yaml:"deepl_api_key"`
	LegacyAPIKey      string            `yaml:"api_key"`
	TargetLang        string            `yaml:"target_lang"`
	OutputFormat      string            `yaml:"output_format"`
	Passthrough       []string          `yaml:"passthrough"`
	Glossary          map[string]string `yaml:"glossary"`
	TranslationEngine string            `yaml:"translation_engine"`
	OpenAIAPIKey      string            `yaml:"openai_api_key"`
	OpenAIModel       string            `yaml:"openai_model"`
	OpenAIPromptFile  string            `yaml:"openai_prompt_file"`
	OpenAITemperature float64           `yaml:"openai_temperature"`
	Debug             bool              `yaml:"debug"`
	TraceLogFile      string            `yaml:"trace_log_file"`
}

// Load builds Config with priority: CLI args > env vars > config file > defaults.
// configFile and other arguments are already-parsed CLI values (empty = not provided).
func Load(
	configFile,
	apiKey,
	targetLang,
	outputFormat,
	translationEngine,
	openAIAPIKey,
	openAIModel,
	openAIPromptFile,
	openAITemperature,
	debug,
	traceLogFile string,
) (*Config, error) {
	cfg := &Config{
		TargetLang:        "JA",
		OutputFormat:      "({DetectedSourceLanguage}) {TranslatedText}",
		TranslationEngine: "deepl",
		OpenAIModel:       "gpt-5.4-mini",
		OpenAITemperature: 0.2,
	}

	path := resolveConfigPath(configFile)

	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if cfg.DeepLAPIKey == "" && cfg.LegacyAPIKey != "" {
			cfg.DeepLAPIKey = cfg.LegacyAPIKey
		}
	} else if configFile != "" {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	if v := os.Getenv("WOWSCHAT_API_KEY"); v != "" {
		cfg.DeepLAPIKey = v
	}
	if v := os.Getenv("WOWSCHAT_TARGET_LANG"); v != "" {
		cfg.TargetLang = v
	}
	if v := os.Getenv("WOWSCHAT_OUTPUT_FORMAT"); v != "" {
		cfg.OutputFormat = v
	}
	if v := os.Getenv("WOWSCHAT_TRANSLATION_ENGINE"); v != "" {
		cfg.TranslationEngine = v
	}
	if v := os.Getenv("WOWSCHAT_OPENAI_API_KEY"); v != "" {
		cfg.OpenAIAPIKey = v
	}
	if v := os.Getenv("WOWSCHAT_OPENAI_MODEL"); v != "" {
		cfg.OpenAIModel = v
	}
	if v := os.Getenv("WOWSCHAT_OPENAI_PROMPT_FILE"); v != "" {
		cfg.OpenAIPromptFile = v
	}
	if v := os.Getenv("WOWSCHAT_OPENAI_TEMPERATURE"); v != "" {
		temperature, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid WOWSCHAT_OPENAI_TEMPERATURE %q: %w", v, err)
		}
		cfg.OpenAITemperature = temperature
	}
	if v := os.Getenv("WOWSCHAT_DEBUG"); v != "" {
		debugValue, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("invalid WOWSCHAT_DEBUG %q: %w", v, err)
		}
		cfg.Debug = debugValue
	}
	if v := os.Getenv("WOWSCHAT_TRACE_LOG_FILE"); v != "" {
		cfg.TraceLogFile = v
	}

	if apiKey != "" {
		cfg.DeepLAPIKey = apiKey
	}
	if targetLang != "" {
		cfg.TargetLang = targetLang
	}
	if outputFormat != "" {
		cfg.OutputFormat = outputFormat
	}
	if translationEngine != "" {
		cfg.TranslationEngine = translationEngine
	}
	if openAIAPIKey != "" {
		cfg.OpenAIAPIKey = openAIAPIKey
	}
	if openAIModel != "" {
		cfg.OpenAIModel = openAIModel
	}
	if openAIPromptFile != "" {
		cfg.OpenAIPromptFile = openAIPromptFile
	}
	if openAITemperature != "" {
		temperature, err := strconv.ParseFloat(openAITemperature, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid openai_temperature %q: %w", openAITemperature, err)
		}
		cfg.OpenAITemperature = temperature
	}
	if debug != "" {
		debugValue, err := strconv.ParseBool(debug)
		if err != nil {
			return nil, fmt.Errorf("invalid debug %q: %w", debug, err)
		}
		cfg.Debug = debugValue
	}
	if traceLogFile != "" {
		cfg.TraceLogFile = traceLogFile
	}

	cfg.TargetLang = strings.ToUpper(cfg.TargetLang)
	cfg.TranslationEngine = strings.ToLower(cfg.TranslationEngine)
	return cfg, nil
}

func resolveConfigPath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "config.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "config.yaml"
}
