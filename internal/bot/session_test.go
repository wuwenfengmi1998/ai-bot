package bot

import (
	"testing"

	"github.com/openai/openai-go"

	"myaibot/internal/config"
	"myaibot/internal/store"
)

func newTestBot(t *testing.T) *Bot {
	t.Helper()
	t.Chdir(t.TempDir())
	for _, name := range []string{"get_current_time", "calculate", "random_number"} {
		if err := config.WriteDefaultToolConfig(name, map[string]any{"enabled": true, "prompt": "p"}); err != nil {
			t.Fatalf("写入工具配置失败: %v", err)
		}
	}
	cfg := &config.Config{
		BotName:         "test",
		SystemPrompt:    "测试系统提示",
		DefaultProvider: "p",
		DefaultModel:    "m",
		Providers: []config.Provider{
			{Name: "p", BaseURL: "x", Models: []config.ModelConfig{{Name: "m"}}},
		},
	}
	b, err := New(cfg)
	if err != nil {
		t.Fatalf("New 出错: %v", err)
	}
	return b
}

func TestSessionRoundtrip(t *testing.T) {
	b := newTestBot(t)
	b.history = append(b.history,
		openai.UserMessage("你好"),
		openai.AssistantMessage("你好！有什么可以帮你？"),
	)
	msgs := b.SessionMessages()
	if len(msgs) != 3 {
		t.Fatalf("SessionMessages 数量 = %d, want 3", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[0].Content != "测试系统提示" {
		t.Errorf("首条应为系统提示: %+v", msgs[0])
	}

	restored := &Bot{}
	restored.RestoreSession(&store.Session{Messages: msgs})
	if restored.systemPrompt != "测试系统提示" {
		t.Errorf("systemPrompt 未恢复: %q", restored.systemPrompt)
	}
	if len(restored.history) != 2 {
		t.Fatalf("history 数量 = %d, want 2", len(restored.history))
	}
	if u := restored.history[0].OfUser; u == nil || u.Content.OfString.Value != "你好" {
		t.Errorf("用户消息未还原: %+v", restored.history[0])
	}
	if a := restored.history[1].OfAssistant; a == nil || a.Content.OfString.Value != "你好！有什么可以帮你？" {
		t.Errorf("助手消息未还原: %+v", restored.history[1])
	}
}

func TestRestoreSessionOverridesPrompt(t *testing.T) {
	b := newTestBot(t)
	b.RestoreSession(&store.Session{
		SystemPrompt: "会话覆盖的系统提示",
		Messages:     []store.Message{{Role: "user", Content: "hi"}},
	})
	if b.systemPrompt != "会话覆盖的系统提示" {
		t.Errorf("systemPrompt 未覆盖: %q", b.systemPrompt)
	}
}

func TestRestoreTrimsHistory(t *testing.T) {
	b := newTestBot(t)
	var msgs []store.Message
	for i := 0; i < maxHistory+10; i++ {
		msgs = append(msgs, store.Message{Role: "user", Content: "m"})
	}
	b.RestoreSession(&store.Session{Messages: msgs})
	if len(b.history) != maxHistory {
		t.Errorf("history 应截断到 %d, got %d", maxHistory, len(b.history))
	}
}

func TestClearHistory(t *testing.T) {
	b := newTestBot(t)
	b.history = append(b.history, openai.UserMessage("hi"))
	b.ClearHistory()
	if len(b.history) != 0 {
		t.Errorf("history 应清空, got %d", len(b.history))
	}
	if b.systemPrompt != "测试系统提示" {
		t.Errorf("systemPrompt 应保留: %q", b.systemPrompt)
	}
}
