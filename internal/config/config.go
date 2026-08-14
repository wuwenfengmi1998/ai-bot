package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"gopkg.in/yaml.v3"
)

const (
	configDir     = "data"
	configFile    = "config.yaml"
	toolConfigDir = "data/tools"
)

type ModelConfig struct {
	Name          string `yaml:"name"`
	ContextWindow int64  `yaml:"context_window"`
}

// UnmarshalYAML 兼容两种格式：
//
//	models: [deepseek-v4-flash, deepseek-v4-pro]   # 字符串列表（旧格式）
//	models:
//	  - name: deepseek-v4-flash
//	    context_window: 1048576                    # 对象列表
func (m *ModelConfig) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		m.Name = node.Value
		return nil
	case yaml.MappingNode:
		type raw ModelConfig
		var r raw
		if err := node.Decode(&r); err != nil {
			return err
		}
		*m = ModelConfig(r)
		return nil
	default:
		return fmt.Errorf("模型配置必须是字符串或对象")
	}
}

type Provider struct {
	Name            string        `yaml:"name"`
	APIKey          string        `yaml:"api_key"`
	BaseURL         string        `yaml:"base_url"`
	Models          []ModelConfig `yaml:"models"`
	AutoFetchModels bool          `yaml:"auto_fetch_models"`
	Thinking        string        `yaml:"thinking"`
	ReasoningEffort string        `yaml:"reasoning_effort"`
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
	changed := applyDefaults(cfg)
	if err := validate(cfg); err != nil {
		return nil, err
	}
	if changed {
		if err := writeFile(path, cfg); err != nil {
			return nil, err
		}
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
		Models:  []ModelConfig{{Name: legacy.Model}},
	}
	if p.BaseURL == "" {
		p.BaseURL = "https://api.openai.com/v1"
	}
	if len(p.Models) == 0 || p.Models[0].Name == "" {
		p.Models = []ModelConfig{{Name: "gpt-4o-mini"}}
	}
	cfg.Providers = []Provider{p}
	cfg.DefaultProvider = p.Name
	cfg.DefaultModel = p.Models[0].Name
	return writeFile(path, cfg)
}

func applyDefaults(c *Config) (changed bool) {
	if c.BotName == "" {
		c.BotName = "ai-bot"
		changed = true
	}
	if c.Port == 0 {
		c.Port = 8080
		changed = true
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
		changed = true
	}
	if c.SystemPrompt == "" {
		c.SystemPrompt = "你是一个乐于助人的 AI 助手。"
		changed = true
	}
	if c.DefaultProvider == "" && len(c.Providers) > 0 {
		c.DefaultProvider = c.Providers[0].Name
		changed = true
	}
	if c.Database.Driver == "" {
		c.Database.Driver = "sqlite3"
		changed = true
	}
	if c.Database.File == "" {
		c.Database.File = "data/memory.db"
		changed = true
	}
	if c.Database.Driver == "mysql" {
		if c.Database.Host == "" {
			c.Database.Host = "127.0.0.1"
			changed = true
		}
		if c.Database.Port == 0 {
			c.Database.Port = 3306
			changed = true
		}
	}
	return changed
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
		if len(p.Models) == 0 && !p.AutoFetchModels {
			return fmt.Errorf("供应商 %s 未配置 models", p.Name)
		}
		for _, m := range p.Models {
			if m.Name == "" {
				return fmt.Errorf("供应商 %s 包含空模型名", p.Name)
			}
			if m.ContextWindow < 0 {
				return fmt.Errorf("供应商 %s 的模型 %s context_window 无效: %d（不能为负数）", p.Name, m.Name, m.ContextWindow)
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
	if err := validateModelRef("default_model", c.DefaultModel, c); err != nil {
		return err
	}
	if c.ToolModel != "" {
		if err := validateModelRef("tool_model", c.ToolModel, c); err != nil {
			return err
		}
	}
	if c.VisionModel != "" {
		if err := validateModelRef("vision_model", c.VisionModel, c); err != nil {
			return err
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

// validateModelRef 校验模型引用；若引用指向启用了 auto_fetch_models 的供应商，
// 则跳过存在性校验（模型列表将在启动时从 API 拉取）。
func validateModelRef(field, id string, c *Config) error {
	if _, _, err := ResolveModelIn(c, id); err == nil {
		return nil
	}
	providerName, _, hasProvider := strings.Cut(id, "/")
	if !hasProvider {
		providerName = c.DefaultProvider
	}
	if p := FindProviderIn(c, providerName); p != nil && p.AutoFetchModels {
		return nil
	}
	return fmt.Errorf("%s 无效: 模型 %q 不存在", field, id)
}

func FindProvider(name string) *Provider {
	return FindProviderIn(cfg, name)
}

func FindProviderIn(c *Config, name string) *Provider {
	for i := range c.Providers {
		if c.Providers[i].Name == name {
			return &c.Providers[i]
		}
	}
	return nil
}

func FindModel(p *Provider, name string) *ModelConfig {
	for i := range p.Models {
		if p.Models[i].Name == name {
			return &p.Models[i]
		}
	}
	return nil
}

func ResolveModel(id string) (*Provider, string, error) {
	return ResolveModelIn(cfg, id)
}

func ResolveModelIn(c *Config, id string) (*Provider, string, error) {
	if id == "" {
		id = c.DefaultModel
	}
	if providerName, modelName, ok := strings.Cut(id, "/"); ok {
		p := FindProviderIn(c, providerName)
		if p == nil {
			return nil, "", fmt.Errorf("供应商 %q 不存在", providerName)
		}
		if FindModel(p, modelName) == nil {
			return nil, "", fmt.Errorf("供应商 %s 没有模型 %q", p.Name, modelName)
		}
		return p, modelName, nil
	}
	var found *Provider
	for i := range c.Providers {
		if FindModel(&c.Providers[i], id) != nil {
			if found != nil {
				return nil, "", fmt.Errorf("模型 %q 在多个供应商中存在，请使用 provider/model 格式指定", id)
			}
			found = &c.Providers[i]
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
			out = append(out, p.Name+"/"+m.Name)
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
				Models: []ModelConfig{
					{Name: "gpt-4o-mini", ContextWindow: 128000},
					{Name: "gpt-4o", ContextWindow: 128000},
				},
			},
			{
				Name:    "deepseek",
				APIKey:  "",
				BaseURL: "https://api.deepseek.com/v1",
				Models: []ModelConfig{
					{Name: "deepseek-v4-flash", ContextWindow: 1048576},
					{Name: "deepseek-v4-pro", ContextWindow: 1048576},
				},
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

// Save 将配置写回 data/config.yaml。
func Save(c *Config) error {
	path := filepath.Join(configDir, configFile)
	return writeFile(path, c)
}

// ModelsEqual 比较两个模型的名称与上下文窗口是否完全一致。
func ModelsEqual(a, b []ModelConfig) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].ContextWindow != b[i].ContextWindow {
			return false
		}
	}
	return true
}

// FetchModels 从供应商 API 拉取模型列表并合并进 p.Models。
// 已配置模型的 context_window 保留，新模型默认 0。
func FetchModels(ctx context.Context, p *Provider) error {
	if p.APIKey == "" {
		return errors.New("未配置 api_key")
	}
	client := openai.NewClient(
		option.WithAPIKey(p.APIKey),
		option.WithBaseURL(p.BaseURL),
	)
	page, err := client.Models.List(ctx)
	if err != nil {
		return fmt.Errorf("请求模型列表失败: %w", err)
	}
	ids := make([]string, 0, len(page.Data))
	seen := make(map[string]bool, len(page.Data))
	for _, m := range page.Data {
		if m.ID == "" || seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		ids = append(ids, m.ID)
	}
	sort.Strings(ids)

	existing := make(map[string]int64, len(p.Models))
	for _, m := range p.Models {
		existing[m.Name] = m.ContextWindow
	}
	merged := make([]ModelConfig, 0, len(ids))
	for _, id := range ids {
		merged = append(merged, ModelConfig{Name: id, ContextWindow: existing[id]})
	}
	p.Models = merged
	return nil
}
