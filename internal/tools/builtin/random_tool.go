package builtin

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
)

type randomTool struct {
	enabled bool
	prompt  string
}

func NewRandomTool() *randomTool {
	return &randomTool{
		enabled: true,
		prompt:  "生成指定范围内的随机整数，默认 0 到 100（含端点）",
	}
}

func (t *randomTool) Name() string        { return "random_number" }
func (t *randomTool) Description() string { return t.prompt }
func (t *randomTool) Enabled() bool       { return t.enabled }
func (t *randomTool) DefaultConfig() map[string]any {
	return map[string]any{"enabled": true, "prompt": t.prompt}
}

func (t *randomTool) Configure(cfg map[string]any) error {
	var err error
	if t.enabled, err = parseEnabled(cfg); err != nil {
		return err
	}
	if p, ok := cfg["prompt"].(string); ok && p != "" {
		t.prompt = p
	}
	return nil
}

func (t *randomTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"min": map[string]any{"type": "integer", "description": "最小值，默认 0"},
			"max": map[string]any{"type": "integer", "description": "最大值，默认 100"},
		},
	}
}

func (t *randomTool) Execute(args json.RawMessage) (string, error) {
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
