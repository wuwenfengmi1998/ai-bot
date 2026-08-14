package tools

import (
	"encoding/json"
	"time"
)

type TimeTool struct{}

func (TimeTool) Name() string { return "get_current_time" }
func (TimeTool) Description() string {
	return "获取当前日期和时间，可选指定时区（如 Asia/Shanghai，默认本地时区）"
}
func (TimeTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"timezone": map[string]any{"type": "string", "description": "IANA 时区名，如 Asia/Shanghai"},
		},
	}
}

func (TimeTool) Execute(args json.RawMessage) (string, error) {
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
