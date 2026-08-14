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
			got, err := builtin.CalculatorTool{}.Execute(args)
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
	if _, err := (builtin.CalculatorTool{}).Execute(args); err == nil {
		t.Error("非法表达式应返回错误")
	}
}

func TestRandom(t *testing.T) {
	args, _ := json.Marshal(map[string]any{"min": 1, "max": 10})
	for i := 0; i < 100; i++ {
		out, err := builtin.RandomTool{}.Execute(args)
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
	if _, err := (builtin.RandomTool{}).Execute(bad); err == nil {
		t.Error("min>max 应返回错误")
	}
}

func TestTimeTool(t *testing.T) {
	out, err := builtin.TimeTool{}.Execute(nil)
	if err != nil {
		t.Fatalf("Execute 出错: %v", err)
	}
	if !strings.Contains(out, "20") {
		t.Errorf("时间输出异常: %q", out)
	}
}
