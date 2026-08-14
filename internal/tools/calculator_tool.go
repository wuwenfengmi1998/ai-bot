package tools

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/expr-lang/expr"
)

type CalculatorTool struct{}

func (CalculatorTool) Name() string { return "calculate" }
func (CalculatorTool) Description() string {
	return "计算数学表达式，如 \"(12 + 3) * 4\"、\"2^10\"、\"sqrt(9)\""
}
func (CalculatorTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"expression": map[string]any{"type": "string", "description": "要计算的数学表达式"},
		},
		"required": []string{"expression"},
	}
}

func (CalculatorTool) Execute(args json.RawMessage) (string, error) {
	var p struct {
		Expression string `json:"expression"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	if p.Expression == "" {
		return "", fmt.Errorf("expression 不能为空")
	}
	out, err := expr.Eval(p.Expression, nil)
	if err != nil {
		return "", err
	}
	switch v := out.(type) {
	case float64:
		return strconv.FormatFloat(round(v), 'f', -1, 64), nil
	case int:
		return strconv.Itoa(v), nil
	case bool:
		return strconv.FormatBool(v), nil
	case string:
		return v, nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

func round(v float64) float64 {
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return v
	}
	return math.Round(v*1e8) / 1e8
}
