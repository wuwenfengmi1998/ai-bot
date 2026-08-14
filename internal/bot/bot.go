package bot

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
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

func (b *Bot) Chat(ctx context.Context, userMsg string) (string, error) {
	if b.provider.APIKey == "" {
		return "", fmt.Errorf("供应商 %s 未配置 api_key，请编辑 data/config.yaml", b.provider.Name)
	}
	history := make([]openai.ChatCompletionMessageParamUnion, 0, len(b.history)+2)
	history = append(history, openai.SystemMessage(b.systemPrompt))
	history = append(history, b.history...)
	history = append(history, openai.UserMessage(userMsg))

	stream := b.client().Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model:    b.model,
		Messages: history,
	})
	answer := ""
	for stream.Next() {
		for _, delta := range stream.Current().Choices {
			answer += delta.Delta.Content
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
