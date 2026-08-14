package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type stubTool struct{}

func (stubTool) Name() string        { return "stub" }
func (stubTool) Description() string { return "测试桩工具" }
func (stubTool) Parameters() map[string]any {
	return map[string]any{"type": "object"}
}
func (stubTool) Execute(args json.RawMessage) (string, error) {
	return fmt.Sprintf("stub:%s", args), nil
}

type stubTool2 struct{}

func (stubTool2) Name() string        { return "stub2" }
func (stubTool2) Description() string { return "测试桩工具2" }
func (stubTool2) Parameters() map[string]any {
	return map[string]any{"type": "object"}
}
func (stubTool2) Execute(args json.RawMessage) (string, error) {
	return "stub2", nil
}

func TestRegistry(t *testing.T) {
	r := NewRegistry(stubTool{}, stubTool2{})
	if _, ok := r.Get("stub"); !ok {
		t.Error("stub 未注册")
	}
	if _, ok := r.Get("nonexistent"); ok {
		t.Error("未知工具不应存在")
	}
	if len(r.ParamList()) != 2 {
		t.Errorf("ParamList 数量 = %d, want 2", len(r.ParamList()))
	}
	if _, err := r.Execute("nonexistent", nil); err == nil {
		t.Error("执行未知工具应返回错误")
	}
	out, err := r.Execute("stub", json.RawMessage(`{"a":1}`))
	if err != nil {
		t.Fatalf("Execute 出错: %v", err)
	}
	if out != `stub:{"a":1}` {
		t.Errorf("Execute 输出 = %q", out)
	}
	list := r.List()
	if len(list) != 2 {
		t.Fatalf("List 数量 = %d, want 2", len(list))
	}
	wantOrder := []string{"stub", "stub2"}
	for i, want := range wantOrder {
		if !strings.HasPrefix(list[i], want+" - ") {
			t.Errorf("List[%d] = %q, want 前缀 %q", i, list[i], want+" - ")
		}
	}
}
