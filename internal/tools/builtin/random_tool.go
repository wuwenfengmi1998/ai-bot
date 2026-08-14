package builtin

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
)

type RandomTool struct{}

func (RandomTool) Name() string { return "random_number" }
func (RandomTool) Description() string {
	return "生成指定范围内的随机整数，默认 0 到 100（含端点）"
}
func (RandomTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"min": map[string]any{"type": "integer", "description": "最小值，默认 0"},
			"max": map[string]any{"type": "integer", "description": "最大值，默认 100"},
		},
	}
}

func (RandomTool) Execute(args json.RawMessage) (string, error) {
	var p struct {
		Min *int `json:"min"`
		Max *int `json:"max"`
	}
	_ = json.Unmarshal(args, &p)
	min, max := 0, 100
	if p.Min != nil {
		min = *p.Min
	}
	if p.Max != nil {
		max = *p.Max
	}
	if max < min {
		return "", fmt.Errorf("max (%d) 不能小于 min (%d)", max, min)
	}
	return fmt.Sprintf("%d", rand.IntN(max-min+1)+min), nil
}
