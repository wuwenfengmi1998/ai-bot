package builtin

import (
	"encoding/json"
	"fmt"
	"time"
)

type timeTool struct {
	enabled bool
	prompt  string
}

func NewTimeTool() *timeTool {
	return &timeTool{
		enabled: true,
		prompt:  "获取当前日期和时间，可选指定时区（如 Asia/Shanghai，默认本地时区）",
	}
}

func (t *timeTool) Name() string        { return "get_current_time" }
func (t *timeTool) Description() string { return t.prompt }
func (t *timeTool) Enabled() bool       { return t.enabled }
func (t *timeTool) DefaultConfig() map[string]any {
	return map[string]any{"enabled": true, "prompt": t.prompt}
}

func (t *timeTool) Configure(cfg map[string]any) error {
	var err error
	if t.enabled, err = parseEnabled(cfg); err != nil {
		return err
	}
	if p, ok := cfg["prompt"].(string); ok && p != "" {
		t.prompt = p
	}
	return nil
}

func (t *timeTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"timezone": map[string]any{"type": "string", "description": "IANA 时区名，如 Asia/Shanghai"},
		},
	}
}

func (t *timeTool) Execute(args json.RawMessage) (string, error) {
	var p struct {
		Timezone string `json:"timezone"`
	}
	_ = json.Unmarshal(args, &p)
	loc := time.Local
	if p.Timezone != "" {
		l, err := time.LoadLocation(p.Timezone)
		if err != nil {
			return "", err
		}
		loc = l
	}
	now := time.Now().In(loc)
	return now.Format("2006-01-02 15:04:05 Monday MST"), nil
}

func parseEnabled(cfg map[string]any) (bool, error) {
	v, ok := cfg["enabled"]
	if !ok {
		return true, nil
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("enabled 必须是布尔值，实际为 %T", v)
	}
	return b, nil
}
