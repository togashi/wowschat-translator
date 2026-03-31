package config

import (
	_ "embed"
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
	Expand            map[string]string `yaml:"expand"`
	TranslationEngine string            `yaml:"translation_engine"`
	OpenAIAPIKey           string            `yaml:"openai_api_key"`
	OpenAIModel            string            `yaml:"openai_model"`
	OpenAIPromptFile       string            `yaml:"openai_prompt_file"`
	OpenAITemperature      float64           `yaml:"openai_temperature"`
	AnthropicAPIKey        string            `yaml:"anthropic_api_key"`
	AnthropicModel         string            `yaml:"anthropic_model"`
	AnthropicPromptFile    string            `yaml:"anthropic_prompt_file"`
	AnthropicTemperature   float64           `yaml:"anthropic_temperature"`
	Debug                  bool              `yaml:"debug"`
	TraceLogFile           string            `yaml:"trace_log_file"`
}

//go:embed default_config.yaml
var embeddedDefaultConfigYAML string

// EnsureDefaultConfig creates a default config file when it does not exist.
// It returns the resolved path and whether a new file was created.
func EnsureDefaultConfig(configFile string) (string, bool, error) {
	path := resolveConfigPath(configFile)
	if strings.TrimSpace(configFile) == "" {
		path = defaultConfigPath()
	}

	if _, err := os.Stat(path); err == nil {
		return path, false, nil
	} else if !os.IsNotExist(err) {
		return path, false, fmt.Errorf("stat %s: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, false, fmt.Errorf("create config dir %s: %w", filepath.Dir(path), err)
	}

	content := strings.TrimSpace(embeddedDefaultConfigYAML)
	if content == "" {
		return path, false, fmt.Errorf("embedded default config is empty")
	}
	content += "\n"

	writeErr := os.WriteFile(path, []byte(content), 0o644)
	if writeErr == nil {
		return path, true, nil
	}

	if strings.TrimSpace(configFile) != "" {
		return path, false, fmt.Errorf("write %s: %w", path, writeErr)
	}

	fallback := "config.yaml"
	if filepath.Clean(fallback) == filepath.Clean(path) {
		return path, false, fmt.Errorf("write %s: %w", path, writeErr)
	}
	if _, statErr := os.Stat(fallback); statErr == nil {
		return fallback, false, nil
	} else if !os.IsNotExist(statErr) {
		return fallback, false, fmt.Errorf("stat %s: %w", fallback, statErr)
	}
	if writeErr := os.WriteFile(fallback, []byte(content), 0o644); writeErr != nil {
		return path, false, fmt.Errorf("write %s: %w", path, writeErr)
	}
	return fallback, true, nil
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
	anthropicAPIKey,
	anthropicModel,
	anthropicPromptFile,
	anthropicTemperature,
	debug,
	traceLogFile string,
) (*Config, error) {
	cfg := &Config{
		TargetLang:        "JA",
		OutputFormat:      "({DetectedSourceLanguage}) {TranslatedText}",
		TranslationEngine: "deepl",
		OpenAIModel:          "gpt-5.4-mini",
		OpenAITemperature:    0.2,
		AnthropicModel:       "claude-haiku-4-5-20251001",
		AnthropicTemperature: 0.2,
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
	if v := os.Getenv("WOWSCHAT_ANTHROPIC_API_KEY"); v != "" {
		cfg.AnthropicAPIKey = v
	}
	if v := os.Getenv("WOWSCHAT_ANTHROPIC_MODEL"); v != "" {
		cfg.AnthropicModel = v
	}
	if v := os.Getenv("WOWSCHAT_ANTHROPIC_PROMPT_FILE"); v != "" {
		cfg.AnthropicPromptFile = v
	}
	if v := os.Getenv("WOWSCHAT_ANTHROPIC_TEMPERATURE"); v != "" {
		temperature, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid WOWSCHAT_ANTHROPIC_TEMPERATURE %q: %w", v, err)
		}
		cfg.AnthropicTemperature = temperature
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
	if anthropicAPIKey != "" {
		cfg.AnthropicAPIKey = anthropicAPIKey
	}
	if anthropicModel != "" {
		cfg.AnthropicModel = anthropicModel
	}
	if anthropicPromptFile != "" {
		cfg.AnthropicPromptFile = anthropicPromptFile
	}
	if anthropicTemperature != "" {
		temperature, err := strconv.ParseFloat(anthropicTemperature, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid anthropic_temperature %q: %w", anthropicTemperature, err)
		}
		cfg.AnthropicTemperature = temperature
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
	candidate := defaultConfigPath()
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return "config.yaml"
}

func defaultConfigPath() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "config.yaml")
	}
	return "config.yaml"
}
