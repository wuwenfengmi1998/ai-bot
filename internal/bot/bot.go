package bot

import (
	"context"
	"errors"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"myaibot/internal/config"
)

const maxHistory = 20

type Bot struct {
	client  *openai.Client
	cfg     *config.Config
	history []openai.ChatCompletionMessageParamUnion
}

func New(cfg *config.Config) *Bot {
	client := openai.NewClient(
		option.WithAPIKey(cfg.APIKey),
		option.WithBaseURL(cfg.BaseURL),
	)
	return &Bot{
		client: &client,
		cfg:    cfg,
	}
}

func (b *Bot) Chat(ctx context.Context, userMsg string) (string, error) {
	if b.cfg.APIKey == "" {
		return "", errors.New("未配置 api_key，请编辑 data/config.yaml")
	}
	history := make([]openai.ChatCompletionMessageParamUnion, 0, len(b.history)+2)
	history = append(history, openai.SystemMessage(b.cfg.SystemPrompt))
	history = append(history, b.history...)
	history = append(history, openai.UserMessage(userMsg))

	stream := b.client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model:    b.cfg.Model,
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
