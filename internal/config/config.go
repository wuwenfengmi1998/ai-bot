package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	configDir  = "data"
	configFile = "config.yaml"
)

type Provider struct {
	Name            string   `yaml:"name"`
	APIKey          string   `yaml:"api_key"`
	BaseURL         string   `yaml:"base_url"`
	Models          []string `yaml:"models"`
	Thinking        string   `yaml:"thinking"`
	ReasoningEffort string   `yaml:"reasoning_effort"`
}

type Config struct {
	BotName         string     `yaml:"bot_name"`
	Port            int        `yaml:"port"`
	LogLevel        string     `yaml:"log_level"`
	SystemPrompt    string     `yaml:"system_prompt"`
	Providers       []Provider `yaml:"providers"`
	DefaultProvider string     `yaml:"default_provider"`
	DefaultModel    string     `yaml:"default_model"`
	ToolModel       string     `yaml:"tool_model"`
	VisionModel     string     `yaml:"vision_model"`
}

type legacyConfig struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
	Model   string `yaml:"model"`
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
	if len(cfg.Providers) == 0 {
		if err := migrateLegacy(path, data); err != nil {
			return nil, err
		}
	}
	applyDefaults(cfg)
	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func GetConfig() *Config {
	return cfg
}

func migrateLegacy(path string, data []byte) error {
	legacy := &legacyConfig{}
	if err := yaml.Unmarshal(data, legacy); err != nil {
		return err
	}
	if legacy.APIKey == "" && legacy.BaseURL == "" && legacy.Model == "" {
		return errors.New("配置文件中没有 providers，请检查 data/config.yaml")
	}
	p := Provider{
		Name:    "openai",
		APIKey:  legacy.APIKey,
		BaseURL: legacy.BaseURL,
		Models:  []string{legacy.Model},
	}
	if p.BaseURL == "" {
		p.BaseURL = "https://api.openai.com/v1"
	}
	if len(p.Models) == 0 || p.Models[0] == "" {
		p.Models = []string{"gpt-4o-mini"}
	}
	cfg.Providers = []Provider{p}
	cfg.DefaultProvider = p.Name
	cfg.DefaultModel = p.Models[0]
	return writeFile(path, cfg)
}

func applyDefaults(c *Config) {
	if c.BotName == "" {
		c.BotName = "ai-bot"
	}
	if c.Port == 0 {
		c.Port = 8080
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.SystemPrompt == "" {
		c.SystemPrompt = "你是一个乐于助人的 AI 助手。"
	}
	if c.DefaultProvider == "" && len(c.Providers) > 0 {
		c.DefaultProvider = c.Providers[0].Name
	}
}

func validate(c *Config) error {
	if len(c.Providers) == 0 {
		return errors.New("至少需要一个供应商 (providers)")
	}
	names := make(map[string]bool, len(c.Providers))
	for i := range c.Providers {
		p := &c.Providers[i]
		if p.Name == "" {
			return fmt.Errorf("providers[%d] 缺少 name", i)
		}
		if names[p.Name] {
			return fmt.Errorf("供应商名称重复: %s", p.Name)
		}
		names[p.Name] = true
		if p.BaseURL == "" {
			return fmt.Errorf("供应商 %s 缺少 base_url", p.Name)
		}
		if len(p.Models) == 0 {
			return fmt.Errorf("供应商 %s 未配置 models", p.Name)
		}
		for _, m := range p.Models {
			if m == "" {
				return fmt.Errorf("供应商 %s 包含空模型名", p.Name)
			}
		}
		if p.Thinking != "" && !contains([]string{"enabled", "disabled"}, p.Thinking) {
			return fmt.Errorf("供应商 %s 的 thinking 无效: %q（可选 enabled/disabled）", p.Name, p.Thinking)
		}
		if p.ReasoningEffort != "" && !contains([]string{"low", "high", "max"}, p.ReasoningEffort) {
			return fmt.Errorf("供应商 %s 的 reasoning_effort 无效: %q（可选 low/high/max）", p.Name, p.ReasoningEffort)
		}
	}
	if _, ok := names[c.DefaultProvider]; !ok {
		return fmt.Errorf("default_provider %q 不存在", c.DefaultProvider)
	}
	if _, _, err := ResolveModel(c.DefaultModel); err != nil {
		return fmt.Errorf("default_model 无效: %w", err)
	}
	if c.ToolModel != "" {
		if _, _, err := ResolveModel(c.ToolModel); err != nil {
			return fmt.Errorf("tool_model 无效: %w", err)
		}
	}
	if c.VisionModel != "" {
		if _, _, err := ResolveModel(c.VisionModel); err != nil {
			return fmt.Errorf("vision_model 无效: %w", err)
		}
	}
	return nil
}

func FindProvider(name string) *Provider {
	for i := range cfg.Providers {
		if cfg.Providers[i].Name == name {
			return &cfg.Providers[i]
		}
	}
	return nil
}

func ResolveModel(id string) (*Provider, string, error) {
	if id == "" {
		id = cfg.DefaultModel
	}
	if providerName, modelName, ok := strings.Cut(id, "/"); ok {
		p := FindProvider(providerName)
		if p == nil {
			return nil, "", fmt.Errorf("供应商 %q 不存在", providerName)
		}
		if !contains(p.Models, modelName) {
			return nil, "", fmt.Errorf("供应商 %s 没有模型 %q", p.Name, modelName)
		}
		return p, modelName, nil
	}
	var found *Provider
	for i := range cfg.Providers {
		if contains(cfg.Providers[i].Models, id) {
			if found != nil {
				return nil, "", fmt.Errorf("模型 %q 在多个供应商中存在，请使用 provider/model 格式指定", id)
			}
			found = &cfg.Providers[i]
		}
	}
	if found == nil {
		return nil, "", fmt.Errorf("模型 %q 不存在", id)
	}
	return found, id, nil
}

func AllModels() []string {
	var out []string
	for i := range cfg.Providers {
		p := &cfg.Providers[i]
		for _, m := range p.Models {
			out = append(out, p.Name+"/"+m)
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func writeDefault(path string) error {
	cfg = &Config{
		BotName:      "ai-bot",
		Port:         8080,
		LogLevel:     "info",
		SystemPrompt: "你是一个乐于助人的 AI 助手。",
		Providers: []Provider{
			{
				Name:    "openai",
				APIKey:  "",
				BaseURL: "https://api.openai.com/v1",
				Models:  []string{"gpt-4o-mini", "gpt-4o"},
			},
			{
				Name:    "deepseek",
				APIKey:  "",
				BaseURL: "https://api.deepseek.com/v1",
				Models:  []string{"deepseek-chat", "deepseek-reasoner"},
			},
		},
		DefaultProvider: "openai",
		DefaultModel:    "gpt-4o-mini",
	}
	if err := validate(cfg); err != nil {
		return err
	}
	return writeFile(path, cfg)
}

func writeFile(path string, c *Config) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
