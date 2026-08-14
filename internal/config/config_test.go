package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyDatabaseDefaults(t *testing.T) {
	c := &Config{Providers: []Provider{{Name: "p", BaseURL: "x", Models: []ModelConfig{{Name: "m"}}}}}
	changed := applyDefaults(c)
	if !changed {
		t.Error("缺失字段应返回 changed=true")
	}
	if c.Database.Driver != "sqlite3" {
		t.Errorf("默认 driver = %q, want sqlite3", c.Database.Driver)
	}
	if c.Database.File != "data/memory.db" {
		t.Errorf("默认 file = %q, want data/memory.db", c.Database.File)
	}
}

func TestApplyDefaultsNoChange(t *testing.T) {
	c := &Config{
		BotName:         "x",
		LogLevel:        "debug",
		SystemPrompt:    "sp",
		DefaultProvider: "p",
		Providers:       []Provider{{Name: "p", BaseURL: "x", Models: []ModelConfig{{Name: "m"}}}},
		Database:        DatabaseConfig{Driver: "mysql", File: "f", Host: "h", Port: 3307, Name: "n"},
	}
	if applyDefaults(c) {
		t.Error("完整配置不应返回 changed")
	}
}

func TestApplyMySQLDefaults(t *testing.T) {
	c := &Config{
		Providers: []Provider{{Name: "p", BaseURL: "x", Models: []ModelConfig{{Name: "m"}}}},
		Database:  DatabaseConfig{Driver: "mysql", Name: "memory"},
	}
	changed := applyDefaults(c)
	if !changed {
		t.Error("mysql 缺 host/port 应返回 changed")
	}
	if c.Database.Host != "127.0.0.1" {
		t.Errorf("默认 host = %q, want 127.0.0.1", c.Database.Host)
	}
	if c.Database.Port != 3306 {
		t.Errorf("默认 port = %d, want 3306", c.Database.Port)
	}
}

func TestLoadWritesBackDatabase(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg = nil
	path := filepath.Join("data", "config.yaml")
	old := "bot_name: test-bot\ndefault_provider: deepseek\ndefault_model: deepseek-v4-flash\nproviders:\n    - name: deepseek\n      base_url: https://api.deepseek.com\n      models:\n        - deepseek-v4-flash\n"
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load()
	if err != nil {
		t.Fatalf("Load 出错: %v", err)
	}
	if c.Database.Driver != "sqlite3" {
		t.Errorf("默认 driver = %q, want sqlite3", c.Database.Driver)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取写回后的配置失败: %v", err)
	}
	if !strings.Contains(string(data), "database:") || !strings.Contains(string(data), "sqlite3") {
		t.Errorf("写回的文件缺少 database 段:\n%s", data)
	}
	if !strings.Contains(string(data), "test-bot") {
		t.Errorf("写回不应覆盖已有字段:\n%s", data)
	}
}

func TestLoadNoRewriteWhenComplete(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg = nil
	path := filepath.Join("data", "config.yaml")
	full := "bot_name: test-bot\ndefault_provider: deepseek\ndefault_model: deepseek-v4-flash\nproviders:\n    - name: deepseek\n      base_url: https://api.deepseek.com\n      models:\n        - deepseek-v4-flash\n"
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(full), 0o644); err != nil {
		t.Fatal(err)
	}
	// 完整配置（含 database 段）
	complete := full + "database:\n    driver: sqlite3\n    file: data/memory.db\n"
	if err := os.WriteFile(path, []byte(complete), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err != nil {
		t.Fatalf("Load 出错: %v", err)
	}
	info, _ := os.Stat(path)
	before := info.ModTime()
	if _, err := Load(); err != nil {
		t.Fatalf("第二次 Load 出错: %v", err)
	}
	info, _ = os.Stat(path)
	if !info.ModTime().Equal(before) {
		t.Error("完整配置不应重复写回文件")
	}
}

func TestLoadSystemPromptCreatesDefault(t *testing.T) {
	t.Chdir(t.TempDir())
	prompt, err := LoadSystemPrompt()
	if err != nil {
		t.Fatalf("LoadSystemPrompt 出错: %v", err)
	}
	if prompt != defaultSystemPrompt {
		t.Errorf("默认提示词 = %q, want %q", prompt, defaultSystemPrompt)
	}
	data, err := os.ReadFile(systemPromptFile)
	if err != nil {
		t.Fatalf("默认文件未创建: %v", err)
	}
	if string(data) != defaultSystemPrompt {
		t.Errorf("文件内容 = %q", string(data))
	}
}

func TestLoadSystemPromptReadsExisting(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(systemPromptFile, []byte("自定义提示词\n多行"), 0o644); err != nil {
		t.Fatal(err)
	}
	prompt, err := LoadSystemPrompt()
	if err != nil {
		t.Fatalf("LoadSystemPrompt 出错: %v", err)
	}
	if prompt != "自定义提示词\n多行" {
		t.Errorf("应返回文件内容: %q", prompt)
	}
}

func TestLoadWritesBackExcludesSystemPrompt(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg = nil
	path := filepath.Join("data", "config.yaml")
	old := "bot_name: test-bot\nsystem_prompt: 旧值\ndefault_provider: deepseek\ndefault_model: deepseek-v4-flash\nproviders:\n    - name: deepseek\n      base_url: https://api.deepseek.com\n      models:\n        - deepseek-v4-flash\n"
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err != nil {
		t.Fatalf("Load 出错: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "system_prompt") {
		t.Errorf("写回的配置不应包含 system_prompt:\n%s", data)
	}
}

func TestValidateDatabase(t *testing.T) {
	c := &Config{
		DefaultProvider: "p",
		DefaultModel:    "m",
		Providers:       []Provider{{Name: "p", BaseURL: "x", Models: []ModelConfig{{Name: "m"}}}},
		Database:        DatabaseConfig{Driver: "oracle"},
	}
	cfg = c
	if err := validate(c); err == nil {
		t.Error("非法驱动应报错")
	}
	c.Database = DatabaseConfig{Driver: "mysql"}
	if err := validate(c); err == nil {
		t.Error("mysql 缺 name 应报错")
	}
	c.Database = DatabaseConfig{Driver: "mysql", Name: "memory"}
	if err := validate(c); err != nil {
		t.Errorf("合法 mysql 配置不应报错: %v", err)
	}
	c.Database = DatabaseConfig{Driver: "sqlite3"}
	if err := validate(c); err != nil {
		t.Errorf("合法 sqlite3 配置不应报错: %v", err)
	}
}
