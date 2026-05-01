package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DeepLAPIKey          string            `yaml:"deepl_api_key"`
	TargetLang           string            `yaml:"target_lang"`
	OutputFormat         string            `yaml:"output_format"`
	Passthrough          []string          `yaml:"passthrough"`
	Glossary             map[string]string `yaml:"glossary"`
	Expand               map[string]string `yaml:"expand"`
	ListenPort           int               `yaml:"listen_port"`
	EndpointPath         string            `yaml:"endpoint_path"`
	TranslationEngine    string            `yaml:"translation_engine"`
	OpenAIAPIKey         string            `yaml:"openai_api_key"`
	OpenAIModel          string            `yaml:"openai_model"`
	OpenAIPromptFile     string            `yaml:"openai_prompt_file"`
	OpenAITemperature    float64           `yaml:"openai_temperature"`
	AnthropicAPIKey      string            `yaml:"anthropic_api_key"`
	AnthropicModel       string            `yaml:"anthropic_model"`
	AnthropicPromptFile  string            `yaml:"anthropic_prompt_file"`
	AnthropicTemperature float64           `yaml:"anthropic_temperature"`
	GeminiAPIKey         string            `yaml:"gemini_api_key"`
	GeminiModel          string            `yaml:"gemini_model"`
	GeminiPromptFile     string            `yaml:"gemini_prompt_file"`
	GeminiTemperature    float64           `yaml:"gemini_temperature"`
	Debug                bool              `yaml:"debug"`
	TraceLogFile         string            `yaml:"trace_log_file"`
}

//go:embed default_config.yaml
var embeddedDefaultConfigYAML string

// EnsureDefaultConfig creates a default config file when it does not exist.
// It returns the resolved path and whether a new file was created.
func EnsureDefaultConfig(configFile string) (string, bool, error) {
	paths := configSearchPaths(configFile)

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, false, nil
		} else if !os.IsNotExist(err) {
			return path, false, fmt.Errorf("stat %s: %w", path, err)
		}
	}

	path := paths[0]

	content := strings.TrimSpace(embeddedDefaultConfigYAML)
	if content == "" {
		return path, false, fmt.Errorf("embedded default config is empty")
	}
	content += "\n"

	var lastErr error
	for _, candidate := range paths {
		if err := os.MkdirAll(filepath.Dir(candidate), 0o755); err != nil {
			lastErr = fmt.Errorf("create config dir %s: %w", filepath.Dir(candidate), err)
			if strings.TrimSpace(configFile) != "" {
				return candidate, false, lastErr
			}
			continue
		}
		if err := os.WriteFile(candidate, []byte(content), 0o644); err == nil {
			return candidate, true, nil
		} else {
			lastErr = fmt.Errorf("write %s: %w", candidate, err)
			if strings.TrimSpace(configFile) != "" {
				return candidate, false, lastErr
			}
		}
	}

	if lastErr != nil {
		return path, false, lastErr
	}
	return path, false, fmt.Errorf("failed to create default config")
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
	geminiAPIKey,
	geminiModel,
	geminiPromptFile,
	geminiTemperature,
	debug,
	traceLogFile string,
) (*Config, error) {
	cfg := &Config{
		TargetLang:           "JA",
		OutputFormat:         "({DetectedSourceLanguage}) {TranslatedText}",
		TranslationEngine:    "deepl",
		ListenPort:           5000,
		EndpointPath:         "/wowschat/",
		OpenAIModel:          "gpt-5.4-mini",
		OpenAITemperature:    0.2,
		AnthropicModel:       "claude-haiku-4-5-20251001",
		AnthropicTemperature: 0.2,
		GeminiModel:          "gemini-2.5-flash",
		GeminiTemperature:    0.2,
	}

	path := resolveConfigPath(configFile)

	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
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
	if v := os.Getenv("WOWSCHAT_GEMINI_API_KEY"); v != "" {
		cfg.GeminiAPIKey = v
	}
	if v := os.Getenv("WOWSCHAT_GEMINI_MODEL"); v != "" {
		cfg.GeminiModel = v
	}
	if v := os.Getenv("WOWSCHAT_GEMINI_PROMPT_FILE"); v != "" {
		cfg.GeminiPromptFile = v
	}
	if v := os.Getenv("WOWSCHAT_GEMINI_TEMPERATURE"); v != "" {
		temperature, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid WOWSCHAT_GEMINI_TEMPERATURE %q: %w", v, err)
		}
		cfg.GeminiTemperature = temperature
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
	if geminiAPIKey != "" {
		cfg.GeminiAPIKey = geminiAPIKey
	}
	if geminiModel != "" {
		cfg.GeminiModel = geminiModel
	}
	if geminiPromptFile != "" {
		cfg.GeminiPromptFile = geminiPromptFile
	}
	if geminiTemperature != "" {
		temperature, err := strconv.ParseFloat(geminiTemperature, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid gemini_temperature %q: %w", geminiTemperature, err)
		}
		cfg.GeminiTemperature = temperature
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
	resolvePromptFilePaths(cfg)
	cfg.TraceLogFile = resolveTraceLogFilePath(cfg.TraceLogFile)

	cfg.TargetLang = strings.ToUpper(cfg.TargetLang)
	cfg.TranslationEngine = strings.ToLower(cfg.TranslationEngine)
	return cfg, nil
}

func resolvePromptFilePaths(cfg *Config) {
	cfg.OpenAIPromptFile = resolvePromptFilePath(cfg.OpenAIPromptFile)
	cfg.AnthropicPromptFile = resolvePromptFilePath(cfg.AnthropicPromptFile)
	cfg.GeminiPromptFile = resolvePromptFilePath(cfg.GeminiPromptFile)
}

func resolvePromptFilePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) {
		return path
	}

	if base := configBaseDir(); base != "" {
		return filepath.Join(base, path)
	}

	return path
}

func resolveTraceLogFilePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) {
		return path
	}

	if base := traceLogBaseDir(); base != "" {
		return filepath.Join(base, path)
	}

	return path
}

func traceLogBaseDir() string {
	switch runtime.GOOS {
	case "linux":
		home, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(home) == "" {
			return ""
		}
		return filepath.Join(home, ".local", "state", "wowschat-translator")
	case "windows":
		localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		if localAppData == "" {
			cacheDir, err := os.UserCacheDir()
			if err == nil {
				localAppData = strings.TrimSpace(cacheDir)
			}
		}
		if localAppData == "" {
			return ""
		}
		return filepath.Join(localAppData, "wowschat-translator")
	default:
		return ""
	}
}

func resolveConfigPath(explicit string) string {
	paths := configSearchPaths(explicit)
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return paths[0]
}

// ResolveConfigPath returns the config path selected by the default search rules
// (or an explicit path if provided).
func ResolveConfigPath(explicit string) string {
	return resolveConfigPath(explicit)
}

func configSearchPaths(explicit string) []string {
	if strings.TrimSpace(explicit) != "" {
		return []string{explicit}
	}
	return defaultConfigCandidates()
}

func defaultConfigCandidates() []string {
	seen := make(map[string]struct{})
	paths := make([]string, 0, 2)

	add := func(path string) {
		if path == "" {
			return
		}
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			return
		}
		seen[clean] = struct{}{}
		paths = append(paths, clean)
	}

	if base := configBaseDir(); base != "" {
		add(filepath.Join(base, "config.yaml"))
	}
	if wd, err := os.Getwd(); err == nil {
		add(filepath.Join(wd, "config.yaml"))
	}
	if len(paths) == 0 {
		add("config.yaml")
	}

	return paths
}

func configBaseDir() string {
	cfgDir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(cfgDir) == "" {
		return ""
	}
	return filepath.Join(cfgDir, "wowschat-translator")
}
