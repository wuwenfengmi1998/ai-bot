package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
	"myaibot/internal/config"
)

const maxHistory = 20

type Bot struct {
	clients      map[string]*openai.Client
	cfg          *config.Config
	provider     *config.Provider
	model        string
	history      []openai.ChatCompletionMessageParamUnion
	systemPrompt string
}

func New(cfg *config.Config) *Bot {
	b := &Bot{
		clients:      make(map[string]*openai.Client),
		cfg:          cfg,
		systemPrompt: cfg.SystemPrompt,
	}
	b.provider = config.FindProvider(cfg.DefaultProvider)
	b.model = cfg.DefaultModel
	return b
}

func (b *Bot) client() *openai.Client {
	if c, ok := b.clients[b.provider.Name]; ok {
		return c
	}
	c := openai.NewClient(
		option.WithAPIKey(b.provider.APIKey),
		option.WithBaseURL(b.provider.BaseURL),
	)
	b.clients[b.provider.Name] = &c
	return &c
}

func (b *Bot) Models() []string {
	return config.AllModels()
}

func (b *Bot) Current() (string, string) {
	return b.provider.Name, b.model
}

func (b *Bot) SwitchModel(id string) error {
	p, modelName, err := config.ResolveModel(id)
	if err != nil {
		return err
	}
	b.provider = p
	b.model = modelName
	return nil
}

func (b *Bot) SetThinking(v string) error {
	if v != "enabled" && v != "disabled" {
		return fmt.Errorf("无效值: %s（可选 enabled/disabled）", v)
	}
	b.provider.Thinking = v
	return nil
}

func (b *Bot) SetEffort(v string) error {
	if v != "low" && v != "high" && v != "max" {
		return fmt.Errorf("无效值: %s（可选 low/high/max）", v)
	}
	b.provider.ReasoningEffort = v
	return nil
}

func (b *Bot) ThinkingConfig() (string, string) {
	return b.provider.Thinking, b.provider.ReasoningEffort
}

func (b *Bot) ContextDump() string {
	var sb strings.Builder
	sb.WriteString("[系统] " + b.systemPrompt + "\n")
	for i, msg := range b.history {
		role, content := "", ""
		switch {
		case msg.OfUser != nil:
			role, content = "用户", msg.OfUser.Content.OfString.Value
		case msg.OfAssistant != nil:
			role, content = "机器人", msg.OfAssistant.Content.OfString.Value
		case msg.OfSystem != nil:
			role, content = "系统", msg.OfSystem.Content.OfString.Value
		}
		if content == "" {
			content = "[多模态内容]"
		}
		sb.WriteString(fmt.Sprintf("[%d] %s: %s\n", i+1, role, content))
	}
	return sb.String()
}

func (b *Bot) Chat(ctx context.Context, userMsg string, onReasoning, onContent func(string)) (string, error) {
	if b.provider.APIKey == "" {
		return "", fmt.Errorf("供应商 %s 未配置 api_key，请编辑 data/config.yaml", b.provider.Name)
	}
	history := make([]openai.ChatCompletionMessageParamUnion, 0, len(b.history)+2)
	history = append(history, openai.SystemMessage(b.systemPrompt))
	history = append(history, b.history...)
	history = append(history, openai.UserMessage(userMsg))

	params := openai.ChatCompletionNewParams{
		Model:    b.model,
		Messages: history,
	}
	if p := b.provider; p.ReasoningEffort != "" && p.Thinking != "disabled" {
		params.ReasoningEffort = shared.ReasoningEffort(p.ReasoningEffort)
	}
	if b.provider.Thinking != "" {
		params.SetExtraFields(map[string]any{
			"thinking": map[string]string{"type": b.provider.Thinking},
		})
	}
	stream := b.client().Chat.Completions.NewStreaming(ctx, params)
	answer := ""
	for stream.Next() {
		for _, choice := range stream.Current().Choices {
			if rc, ok := choice.Delta.JSON.ExtraFields["reasoning_content"]; ok && rc.Valid() {
				var s string
				if json.Unmarshal([]byte(rc.Raw()), &s) == nil && s != "" {
					onReasoning(s)
				}
			}
			if choice.Delta.Content != "" {
				answer += choice.Delta.Content
				onContent(choice.Delta.Content)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return "", fmt.Errorf("调用 AI 接口失败: %w", err)
	}
	b.history = append(b.history, openai.UserMessage(userMsg), openai.AssistantMessage(answer))
	if len(b.history) > maxHistory {
		b.history = b.history[len(b.history)-maxHistory:]
	}
	return answer, nil
}
