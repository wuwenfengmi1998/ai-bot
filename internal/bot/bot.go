package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/shared/constant"
	"github.com/tidwall/gjson"
	"myaibot/internal/config"
	"myaibot/internal/tools"
)

const (
	maxHistory    = 20
	maxToolRounds = 5
)

var toolRegistry = tools.NewRegistry(tools.TimeTool{}, tools.CalculatorTool{}, tools.RandomTool{})

type Bot struct {
	clients        map[string]*openai.Client
	cfg            *config.Config
	provider       *config.Provider
	model          string
	toolProvider   *config.Provider
	toolModel      string
	visionProvider *config.Provider
	visionModel    string
	history        []openai.ChatCompletionMessageParamUnion
	systemPrompt   string
}

func New(cfg *config.Config) *Bot {
	b := &Bot{
		clients:      make(map[string]*openai.Client),
		cfg:          cfg,
		systemPrompt: cfg.SystemPrompt,
	}
	b.provider = config.FindProvider(cfg.DefaultProvider)
	b.model = cfg.DefaultModel
	b.toolProvider, b.toolModel = b.provider, b.model
	if cfg.ToolModel != "" {
		if p, m, err := config.ResolveModel(cfg.ToolModel); err == nil {
			b.toolProvider, b.toolModel = p, m
		}
	}
	b.visionProvider, b.visionModel = b.provider, b.model
	if cfg.VisionModel != "" {
		if p, m, err := config.ResolveModel(cfg.VisionModel); err == nil {
			b.visionProvider, b.visionModel = p, m
		}
	}
	return b
}

func (b *Bot) clientFor(p *config.Provider) *openai.Client {
	if c, ok := b.clients[p.Name]; ok {
		return c
	}
	c := openai.NewClient(
		option.WithAPIKey(p.APIKey),
		option.WithBaseURL(p.BaseURL),
	)
	b.clients[p.Name] = &c
	return &c
}

func (b *Bot) client() *openai.Client {
	return b.clientFor(b.provider)
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

func (b *Bot) CurrentRoles() (tool, vision string) {
	return b.toolProvider.Name + "/" + b.toolModel,
		b.visionProvider.Name + "/" + b.visionModel
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

func (b *Bot) Chat(ctx context.Context, userMsg string, onReasoning, onContent func(string), onTool func(string, string), onToolReasoning func(string)) (string, error) {
	if b.provider.APIKey == "" {
		return "", fmt.Errorf("供应商 %s 未配置 api_key，请编辑 data/config.yaml", b.provider.Name)
	}
	toolMsgs, usedTool, err := b.toolRound(ctx, userMsg, onToolReasoning, onTool)
	if err != nil {
		return "", err
	}
	history := make([]openai.ChatCompletionMessageParamUnion, 0, len(b.history)+len(toolMsgs)+2)
	history = append(history, openai.SystemMessage(b.systemPrompt))
	history = append(history, b.history...)
	history = append(history, openai.UserMessage(userMsg))
	if usedTool {
		history = append(history, toolMsgs...)
	}

	params := openai.ChatCompletionNewParams{
		Model:    b.model,
		Messages: history,
	}
	b.applyThinkingParams(&params, b.provider)
	stream := b.client().Chat.Completions.NewStreaming(ctx, params)
	answer := ""
	for stream.Next() {
		for _, choice := range stream.Current().Choices {
			if s := extraReasoning(choice.Delta.RawJSON()); s != "" {
				onReasoning(s)
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

func (b *Bot) toolRound(ctx context.Context, userMsg string, onReasoning func(string), onTool func(string, string)) ([]openai.ChatCompletionMessageParamUnion, bool, error) {
	if b.toolProvider.APIKey == "" {
		return nil, false, fmt.Errorf("工具调用供应商 %s 未配置 api_key，请编辑 data/config.yaml", b.toolProvider.Name)
	}
	history := make([]openai.ChatCompletionMessageParamUnion, 0, len(b.history)+2)
	history = append(history, openai.SystemMessage(b.systemPrompt))
	history = append(history, b.history...)
	history = append(history, openai.UserMessage(userMsg))

	var toolMsgs []openai.ChatCompletionMessageParamUnion
	for range maxToolRounds {
		params := openai.ChatCompletionNewParams{
			Model:    b.toolModel,
			Messages: history,
			Tools:    toolRegistry.ParamList(),
		}
		b.applyThinkingParams(&params, b.toolProvider)
		stream := b.clientFor(b.toolProvider).Chat.Completions.NewStreaming(ctx, params)

		var (
			reasoning strings.Builder
			calls     []pendingToolCall
		)
		for stream.Next() {
			for _, choice := range stream.Current().Choices {
				if s := extraReasoning(choice.Delta.RawJSON()); s != "" {
					reasoning.WriteString(s)
					onReasoning(s)
				}
				for _, tc := range choice.Delta.ToolCalls {
					for len(calls) <= int(tc.Index) {
						calls = append(calls, pendingToolCall{})
					}
					c := &calls[tc.Index]
					if tc.ID != "" {
						c.id = tc.ID
					}
					if tc.Function.Name != "" {
						c.name = tc.Function.Name
					}
					c.arguments.WriteString(tc.Function.Arguments)
				}
			}
		}
		if err := stream.Err(); err != nil {
			return nil, false, fmt.Errorf("工具调用 AI 请求失败: %w", err)
		}
		if len(calls) == 0 {
			break
		}
		asst := openai.ChatCompletionAssistantMessageParam{
			Content: openai.ChatCompletionAssistantMessageParamContentUnion{
				OfString: param.NewOpt(""),
			},
			ToolCalls: pendingToParams(calls),
		}
		if s := reasoning.String(); s != "" {
			asst.SetExtraFields(map[string]any{"reasoning_content": s})
		}
		asstMsg := openai.ChatCompletionMessageParamUnion{OfAssistant: &asst}
		history = append(history, asstMsg)
		toolMsgs = append(toolMsgs, asstMsg)
		for _, tc := range asst.ToolCalls {
			if onTool != nil {
				onTool(tc.Function.Name, tc.Function.Arguments)
			}
			result, err := toolRegistry.Execute(tc.Function.Name, json.RawMessage(tc.Function.Arguments))
			if err != nil {
				result = "工具执行失败: " + err.Error()
			}
			toolMsg := openai.ToolMessage(result, tc.ID)
			history = append(history, toolMsg)
			toolMsgs = append(toolMsgs, toolMsg)
		}
	}
	return toolMsgs, len(toolMsgs) > 0, nil
}

func (b *Bot) applyThinkingParams(params *openai.ChatCompletionNewParams, p *config.Provider) {
	if p.ReasoningEffort != "" && p.Thinking != "disabled" {
		params.ReasoningEffort = shared.ReasoningEffort(p.ReasoningEffort)
	}
	if p.Thinking != "" {
		params.SetExtraFields(map[string]any{
			"thinking": map[string]string{"type": p.Thinking},
		})
	}
}

func extraReasoning(raw string) string {
	if raw == "" {
		return ""
	}
	return gjson.Get(raw, "reasoning_content").String()
}

type pendingToolCall struct {
	id        string
	name      string
	arguments strings.Builder
}

func pendingToParams(calls []pendingToolCall) []openai.ChatCompletionMessageToolCallParam {
	out := make([]openai.ChatCompletionMessageToolCallParam, 0, len(calls))
	for _, c := range calls {
		out = append(out, openai.ChatCompletionMessageToolCallParam{
			ID: c.id,
			Function: openai.ChatCompletionMessageToolCallFunctionParam{
				Name:      c.name,
				Arguments: c.arguments.String(),
			},
			Type: constant.Function("function"),
		})
	}
	return out
}
