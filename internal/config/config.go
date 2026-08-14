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
	configDir     = "data"
	configFile    = "config.yaml"
	toolConfigDir = "data/tools"
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
	BotName         string         `yaml:"bot_name"`
	Port            int            `yaml:"port"`
	LogLevel        string         `yaml:"log_level"`
	SystemPrompt    string         `yaml:"system_prompt"`
	Providers       []Provider     `yaml:"providers"`
	DefaultProvider string         `yaml:"default_provider"`
	DefaultModel    string         `yaml:"default_model"`
	ToolModel       string         `yaml:"tool_model"`
	VisionModel     string         `yaml:"vision_model"`
	Database        DatabaseConfig `yaml:"database"`
}

type DatabaseConfig struct {
	Driver   string `yaml:"driver"`
	File     string `yaml:"file"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
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
	if c.Database.Driver == "" {
		c.Database.Driver = "sqlite3"
	}
	if c.Database.File == "" {
		c.Database.File = "data/memory.db"
	}
	if c.Database.Driver == "mysql" {
		if c.Database.Host == "" {
			c.Database.Host = "127.0.0.1"
		}
		if c.Database.Port == 0 {
			c.Database.Port = 3306
		}
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
	d := c.Database
	if !contains([]string{"sqlite3", "mysql"}, d.Driver) {
		return fmt.Errorf("database.driver 无效: %q（可选 sqlite3/mysql）", d.Driver)
	}
	if d.Driver == "mysql" && d.Name == "" {
		return errors.New("mysql 需要配置 database.name")
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
		Database: DatabaseConfig{
			Driver: "sqlite3",
			File:   "data/memory.db",
			Host:   "127.0.0.1",
			Port:   3306,
			User:   "root",
			Name:   "memory",
		},
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

// LoadToolConfig 读取 data/tools/<name>.yaml，文件不存在时返回 ok=false。
func LoadToolConfig(name string) (cfg map[string]any, ok bool, err error) {
	path := filepath.Join(toolConfigDir, name+".yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, false, fmt.Errorf("解析工具配置 %s 失败: %w", path, err)
	}
	return cfg, true, nil
}

// WriteDefaultToolConfig 生成工具默认配置模板到 data/tools/<name>.yaml。
func WriteDefaultToolConfig(name string, defaults map[string]any) error {
	if err := os.MkdirAll(toolConfigDir, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(defaults)
	if err != nil {
		return err
	}
	path := filepath.Join(toolConfigDir, name+".yaml")
	return os.WriteFile(path, data, 0o644)
}

func ToolConfigPath(name string) string {
	return filepath.Join(toolConfigDir, name+".yaml")
}
