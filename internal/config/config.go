package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	configDir  = "data"
	configFile = "config.yaml"
)

type Config struct {
	BotName      string `yaml:"bot_name"`
	Port         int    `yaml:"port"`
	LogLevel     string `yaml:"log_level"`
	APIKey       string `yaml:"api_key"`
	BaseURL      string `yaml:"base_url"`
	Model        string `yaml:"model"`
	SystemPrompt string `yaml:"system_prompt"`
}

var cfg *Config

func Load() (*Config, error) {
	if cfg != nil {
		return cfg, nil
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(configDir, configFile)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := writeDefault(path); err != nil {
			return nil, err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg = &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	applyDefaults(cfg)
	return cfg, nil
}

func applyDefaults(c *Config) {
	def := &Config{
		BotName:      "ai-bot",
		Port:         8080,
		LogLevel:     "info",
		BaseURL:      "https://api.openai.com/v1",
		Model:        "gpt-4o-mini",
		SystemPrompt: "你是一个乐于助人的 AI 助手。",
	}
	if c.BotName == "" {
		c.BotName = def.BotName
	}
	if c.Port == 0 {
		c.Port = def.Port
	}
	if c.LogLevel == "" {
		c.LogLevel = def.LogLevel
	}
	if c.BaseURL == "" {
		c.BaseURL = def.BaseURL
	}
	if c.Model == "" {
		c.Model = def.Model
	}
	if c.SystemPrompt == "" {
		c.SystemPrompt = def.SystemPrompt
	}
}

func GetConfig() *Config {
	return cfg
}

func writeDefault(path string) error {
	cfg = &Config{
		BotName:      "ai-bot",
		Port:         8080,
		LogLevel:     "info",
		APIKey:       "",
		BaseURL:      "https://api.openai.com/v1",
		Model:        "gpt-4o-mini",
		SystemPrompt: "你是一个乐于助人的 AI 助手。",
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
