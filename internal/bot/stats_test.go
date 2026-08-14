package bot

import (
	"testing"

	"github.com/openai/openai-go"

	"myaibot/internal/config"
)

func TestEstimateTokensEmpty(t *testing.T) {
	if n := estimateTokens(""); n != 0 {
		t.Errorf("空串应为 0, got %d", n)
	}
}

func TestEstimateTokensKnown(t *testing.T) {
	cases := []struct {
		text string
		want int64
	}{
		{"hello", 1},
		{"hello world", 2},
		{"你好", 1},
		{"你是一个乐于助人的 AI 助手。", 12},
	}
	for _, c := range cases {
		if n := estimateTokens(c.text); n != c.want {
			t.Errorf("estimateTokens(%q) = %d, want %d", c.text, n, c.want)
		}
	}
}

func TestContextStats(t *testing.T) {
	b := &Bot{
		systemPrompt: "你是一个乐于助人的 AI 助手。",
		provider: &config.Provider{
			Name: "p",
			Models: []config.ModelConfig{
				{Name: "m", ContextWindow: 1000},
			},
		},
		model: "m",
		history: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("hello"),
			openai.AssistantMessage("world"),
		},
	}
	used, total := b.ContextStats()
	if total != 1000 {
		t.Errorf("total = %d, want 1000", total)
	}
	if used != 14 { // 系统提示 12 + hello 1 + world 1
		t.Errorf("used = %d, want 14", used)
	}
}
