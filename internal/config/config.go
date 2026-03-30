package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	APIKey       string `yaml:"api_key"`
	TargetLang   string `yaml:"target_lang"`
	OutputFormat string `yaml:"output_format"`
}

// Load builds Config with priority: CLI args > env vars > config file > defaults.
// configFile, apiKey, targetLang, outputFormat are already-parsed CLI argument values (empty = not provided).
func Load(configFile, apiKey, targetLang, outputFormat string) (*Config, error) {
	cfg := &Config{
		TargetLang:   "JA",
		OutputFormat: "({DetectedSourceLanguage}) {TranslatedText}",
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
		cfg.APIKey = v
	}
	if v := os.Getenv("WOWSCHAT_TARGET_LANG"); v != "" {
		cfg.TargetLang = v
	}
	if v := os.Getenv("WOWSCHAT_OUTPUT_FORMAT"); v != "" {
		cfg.OutputFormat = v
	}

	if apiKey != "" {
		cfg.APIKey = apiKey
	}
	if targetLang != "" {
		cfg.TargetLang = targetLang
	}
	if outputFormat != "" {
		cfg.OutputFormat = outputFormat
	}

	cfg.TargetLang = strings.ToUpper(cfg.TargetLang)
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
