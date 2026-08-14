package config

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestModelConfigUnmarshalStringList(t *testing.T) {
	var p struct {
		Models []ModelConfig `yaml:"models"`
	}
	err := yaml.Unmarshal([]byte("models:\n    - deepseek-v4-flash\n    - deepseek-v4-pro\n"), &p)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(p.Models) != 2 || p.Models[0].Name != "deepseek-v4-flash" || p.Models[1].Name != "deepseek-v4-pro" {
		t.Errorf("字符串列表解析异常: %+v", p.Models)
	}
}

func TestModelConfigUnmarshalObjectList(t *testing.T) {
	var p struct {
		Models []ModelConfig `yaml:"models"`
	}
	err := yaml.Unmarshal([]byte("models:\n    - name: deepseek-v4-flash\n      context_window: 1048576\n    - name: deepseek-v4-pro\n"), &p)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(p.Models) != 2 {
		t.Fatalf("数量 = %d, want 2", len(p.Models))
	}
	if p.Models[0].Name != "deepseek-v4-flash" || p.Models[0].ContextWindow != 1048576 {
		t.Errorf("对象列表解析异常: %+v", p.Models[0])
	}
	if p.Models[1].Name != "deepseek-v4-pro" || p.Models[1].ContextWindow != 0 {
		t.Errorf("缺省 context_window 应为 0: %+v", p.Models[1])
	}
}

func TestValidateContextWindow(t *testing.T) {
	base := &Config{
		DefaultProvider: "p",
		DefaultModel:    "m",
		Database:        DatabaseConfig{Driver: "sqlite3"},
		Providers: []Provider{{
			Name: "p", BaseURL: "x",
			Models: []ModelConfig{{Name: "m", ContextWindow: -1}},
		}},
	}
	cfg = base
	if err := validate(base); err == nil {
		t.Error("负 context_window 应报错")
	}
	base.Providers[0].Models[0].ContextWindow = 0
	if err := validate(base); err != nil {
		t.Errorf("context_window 0 不应报错: %v", err)
	}
}

func TestValidateAutoFetch(t *testing.T) {
	c := &Config{
		DefaultProvider: "p",
		DefaultModel:    "m",
		Database:        DatabaseConfig{Driver: "sqlite3"},
		Providers: []Provider{{
			Name: "p", BaseURL: "x",
			AutoFetchModels: true,
		}},
	}
	cfg = c
	if err := validate(c); err != nil {
		t.Errorf("auto_fetch 空 models 不应报错: %v", err)
	}
	c.Providers[0].AutoFetchModels = false
	if err := validate(c); err == nil {
		t.Error("非 auto_fetch 空 models 应报错")
	}
}

func TestValidateAutoFetchDefaultModel(t *testing.T) {
	c := &Config{
		DefaultProvider: "p",
		DefaultModel:    "future-model",
		Database:        DatabaseConfig{Driver: "sqlite3"},
		Providers: []Provider{{
			Name: "p", BaseURL: "x",
			AutoFetchModels: true,
		}},
	}
	cfg = c
	if err := validate(c); err != nil {
		t.Errorf("auto_fetch 时 default_model 应跳过存在性校验: %v", err)
	}
}

func TestFetchModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("请求路径 = %q, want /models", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"object":"list","data":[
			{"id":"deepseek-v4-pro","object":"model","owned_by":"deepseek"},
			{"id":"deepseek-v4-flash","object":"model","owned_by":"deepseek"}
		]}`))
	}))
	defer srv.Close()

	p := &Provider{
		Name:    "deepseek",
		APIKey:  "sk-test",
		BaseURL: srv.URL,
		Models:  []ModelConfig{{Name: "deepseek-v4-flash", ContextWindow: 1048576}},
	}
	if err := FetchModels(context.Background(), p); err != nil {
		t.Fatalf("FetchModels 出错: %v", err)
	}
	if len(p.Models) != 2 {
		t.Fatalf("模型数量 = %d, want 2", len(p.Models))
	}
	if p.Models[0].Name != "deepseek-v4-flash" || p.Models[0].ContextWindow != 1048576 {
		t.Errorf("已有模型应保留 context_window: %+v", p.Models[0])
	}
	if p.Models[1].Name != "deepseek-v4-pro" || p.Models[1].ContextWindow != 0 {
		t.Errorf("新模型 context_window 应为 0: %+v", p.Models[1])
	}
}

func TestFetchModelsNoAPIKey(t *testing.T) {
	if err := FetchModels(context.Background(), &Provider{Name: "p"}); err == nil {
		t.Error("无 api_key 应报错")
	}
}

func TestModelsEqual(t *testing.T) {
	a := []ModelConfig{{Name: "x", ContextWindow: 1}}
	b := []ModelConfig{{Name: "x", ContextWindow: 1}}
	if !ModelsEqual(a, b) {
		t.Error("相同列表应相等")
	}
	b[0].ContextWindow = 2
	if ModelsEqual(a, b) {
		t.Error("不同列表应不相等")
	}
}
