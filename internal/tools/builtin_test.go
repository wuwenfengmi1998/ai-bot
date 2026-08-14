package tools_test

import (
	"encoding/json"
	"strings"
	"testing"

	"myaibot/internal/tools/builtin"
)

func TestCalculator(t *testing.T) {
	cases := []struct {
		name string
		expr string
		want string
	}{
		{"四则运算", "(12 + 3) * 4", "60"},
		{"幂运算", "2^10", "1024"},
		{"浮点", "7 / 2", "3.5"},
		{"小数精度", "1 / 3", "0.33333333"},
		{"布尔", "2 > 1", "true"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args, _ := json.Marshal(map[string]string{"expression": c.expr})
			got, err := builtin.NewCalculatorTool().Execute(args)
			if err != nil {
				t.Fatalf("Execute 出错: %v", err)
			}
			if got != c.want {
				t.Errorf("Execute(%q) = %q, want %q", c.expr, got, c.want)
			}
		})
	}
}

func TestCalculatorInvalid(t *testing.T) {
	args, _ := json.Marshal(map[string]string{"expression": "1 +"})
	if _, err := builtin.NewCalculatorTool().Execute(args); err == nil {
		t.Error("非法表达式应返回错误")
	}
}

func TestRandom(t *testing.T) {
	args, _ := json.Marshal(map[string]any{"min": 1, "max": 10})
	for i := 0; i < 100; i++ {
		out, err := builtin.NewRandomTool().Execute(args)
		if err != nil {
			t.Fatalf("Execute 出错: %v", err)
		}
		var v int
		if err := json.Unmarshal([]byte(out), &v); err != nil {
			t.Fatalf("结果 %q 解析失败: %v", out, err)
		}
		if v < 1 || v > 10 {
			t.Fatalf("结果 %q 不在 [1,10] 内", out)
		}
	}
	bad, _ := json.Marshal(map[string]any{"min": 10, "max": 1})
	if _, err := builtin.NewRandomTool().Execute(bad); err == nil {
		t.Error("min>max 应返回错误")
	}
}

func TestTimeTool(t *testing.T) {
	out, err := builtin.NewTimeTool().Execute(nil)
	if err != nil {
		t.Fatalf("Execute 出错: %v", err)
	}
	if !strings.Contains(out, "20") {
		t.Errorf("时间输出异常: %q", out)
	}
}

func TestConfigurePrompt(t *testing.T) {
	tool := builtin.NewTimeTool()
	err := tool.Configure(map[string]any{"enabled": true, "prompt": "自定义提示词"})
	if err != nil {
		t.Fatalf("Configure 出错: %v", err)
	}
	if tool.Description() != "自定义提示词" {
		t.Errorf("Description = %q, want 自定义提示词", tool.Description())
	}
	if !tool.Enabled() {
		t.Error("enabled 应为 true")
	}
}

func TestConfigureDisable(t *testing.T) {
	tool := builtin.NewCalculatorTool()
	if err := tool.Configure(map[string]any{"enabled": false}); err != nil {
		t.Fatalf("Configure 出错: %v", err)
	}
	if tool.Enabled() {
		t.Error("enabled 应为 false")
	}
}

func TestConfigureInvalid(t *testing.T) {
	tool := builtin.NewRandomTool()
	if err := tool.Configure(map[string]any{"enabled": "yes"}); err == nil {
		t.Error("enabled 非布尔值应报错")
	}
}

func TestDefaultConfig(t *testing.T) {
	tool := builtin.NewTimeTool()
	cfg := tool.DefaultConfig()
	if cfg["enabled"] != true {
		t.Errorf("默认 enabled 应为 true, got %v", cfg["enabled"])
	}
	if p, ok := cfg["prompt"].(string); !ok || p == "" {
		t.Errorf("默认 prompt 缺失: %v", cfg["prompt"])
	}
}
