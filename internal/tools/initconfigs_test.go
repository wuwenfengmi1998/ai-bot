package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type stubConfigurable struct {
	configured map[string]any
	fail       bool
	enabled    bool
}

func (s *stubConfigurable) Name() string        { return "db" }
func (s *stubConfigurable) Description() string { return "测试数据库工具" }
func (s *stubConfigurable) Parameters() map[string]any {
	return map[string]any{"type": "object"}
}
func (s *stubConfigurable) Execute(args json.RawMessage) (string, error) {
	return "ok", nil
}
func (s *stubConfigurable) Enabled() bool { return s.enabled }
func (s *stubConfigurable) DefaultConfig() map[string]any {
	return map[string]any{"enabled": true, "password": "请填写"}
}
func (s *stubConfigurable) Configure(cfg map[string]any) error {
	if s.fail {
		return fmt.Errorf("密码为空")
	}
	if v, ok := cfg["enabled"].(bool); ok {
		s.enabled = v
	}
	s.configured = cfg
	return nil
}

func TestInitConfigsMissing(t *testing.T) {
	t.Chdir(t.TempDir())
	stub := &stubConfigurable{}
	err := NewRegistry(stub).InitConfigs()
	if err == nil || !strings.Contains(err.Error(), "db") {
		t.Fatalf("缺少配置应报错, got %v", err)
	}
	data, rerr := os.ReadFile(filepath.Join("data", "tools", "db.yaml"))
	if rerr != nil {
		t.Fatalf("默认配置未生成: %v", rerr)
	}
	if !strings.Contains(string(data), "请填写") {
		t.Errorf("默认模板内容异常: %s", data)
	}
	if stub.configured != nil {
		t.Error("缺少配置时不应调用 Configure")
	}
}

func TestInitConfigsOK(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join("data", "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("data", "tools", "db.yaml")
	if err := os.WriteFile(path, []byte("host: localhost\npassword: secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stub := &stubConfigurable{enabled: true}
	if err := NewRegistry(stub).InitConfigs(); err != nil {
		t.Fatalf("InitConfigs 出错: %v", err)
	}
	if stub.configured == nil || stub.configured["host"] != "localhost" || stub.configured["password"] != "secret" {
		t.Errorf("Configure 未收到配置: %v", stub.configured)
	}
}

func TestInitConfigsInvalid(t *testing.T) {
	t.Chdir(t.TempDir())
	os.MkdirAll(filepath.Join("data", "tools"), 0o755)
	os.WriteFile(filepath.Join("data", "tools", "db.yaml"), []byte("host: localhost\n"), 0o644)
	stub := &stubConfigurable{fail: true}
	err := NewRegistry(stub).InitConfigs()
	if err == nil || !strings.Contains(err.Error(), "密码为空") {
		t.Fatalf("非法配置应报错, got %v", err)
	}
}

func TestInitConfigsSkipsPlain(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := NewRegistry(stubTool{}).InitConfigs(); err != nil {
		t.Fatalf("普通工具不应报错: %v", err)
	}
}

func TestInitConfigsDisables(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join("data", "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("data", "tools", "db.yaml")
	if err := os.WriteFile(path, []byte("enabled: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stub := &stubConfigurable{enabled: true}
	r := NewRegistry(stub)
	if err := r.InitConfigs(); err != nil {
		t.Fatalf("InitConfigs 出错: %v", err)
	}
	if stub.Enabled() {
		t.Fatal("工具应被禁用")
	}
	if _, ok := r.Get("db"); ok {
		t.Error("禁用的工具应被移出注册表")
	}
}
