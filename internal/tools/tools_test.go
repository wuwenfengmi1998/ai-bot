package tools

import (
	"encoding/json"
	"strings"
	"testing"
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
			got, err := CalculatorTool{}.Execute(args)
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
	if _, err := (CalculatorTool{}).Execute(args); err == nil {
		t.Error("非法表达式应返回错误")
	}
}

func TestRandom(t *testing.T) {
	args, _ := json.Marshal(map[string]any{"min": 1, "max": 10})
	for i := 0; i < 100; i++ {
		out, err := RandomTool{}.Execute(args)
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
	if _, err := (RandomTool{}).Execute(bad); err == nil {
		t.Error("min>max 应返回错误")
	}
}

func TestTimeTool(t *testing.T) {
	out, err := TimeTool{}.Execute(nil)
	if err != nil {
		t.Fatalf("Execute 出错: %v", err)
	}
	if !strings.Contains(out, "20") {
		t.Errorf("时间输出异常: %q", out)
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry(TimeTool{}, CalculatorTool{}, RandomTool{})
	if _, ok := r.Get("get_current_time"); !ok {
		t.Error("get_current_time 未注册")
	}
	if _, ok := r.Get("nonexistent"); ok {
		t.Error("未知工具不应存在")
	}
	if len(r.ParamList()) != 3 {
		t.Errorf("ParamList 数量 = %d, want 3", len(r.ParamList()))
	}
	if _, err := r.Execute("nonexistent", nil); err == nil {
		t.Error("执行未知工具应返回错误")
	}
	list := r.List()
	if len(list) != 3 {
		t.Fatalf("List 数量 = %d, want 3", len(list))
	}
	wantOrder := []string{"calculate", "get_current_time", "random_number"}
	for i, want := range wantOrder {
		if !strings.HasPrefix(list[i], want+" - ") {
			t.Errorf("List[%d] = %q, want 前缀 %q", i, list[i], want+" - ")
		}
	}
}
