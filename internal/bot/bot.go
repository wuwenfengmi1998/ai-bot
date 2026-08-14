package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/shared/constant"
	"github.com/pkoukk/tiktoken-go"
	"github.com/tidwall/gjson"
	"myaibot/internal/config"
	"myaibot/internal/store"
	"myaibot/internal/tools"
	"myaibot/internal/tools/builtin"
)

const (
	maxHistory    = 20
	maxToolRounds = 5
)

type Bot struct {
	clients        map[string]*openai.Client
	cfg            *config.Config
	provider       *config.Provider
	model          string
	toolProvider   *config.Provider
	toolModel      string
	visionProvider *config.Provider
	visionModel    string
	memoryProvider *config.Provider
	memoryModel    string
	history        []openai.ChatCompletionMessageParamUnion
	systemPrompt   string
	toolRegistry   *tools.Registry
}

func New(cfg *config.Config) (*Bot, error) {
	b := &Bot{
		clients:      make(map[string]*openai.Client),
		cfg:          cfg,
		systemPrompt: cfg.SystemPrompt,
		toolRegistry: tools.NewRegistry(builtin.NewTimeTool(), builtin.NewCalculatorTool(), builtin.NewRandomTool()),
	}
	b.provider = config.FindProviderIn(cfg, cfg.DefaultProvider)
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
	b.memoryProvider, b.memoryModel = b.provider, b.model
	if cfg.MemoryModel != "" {
		if p, m, err := config.ResolveModel(cfg.MemoryModel); err == nil {
			b.memoryProvider, b.memoryModel = p, m
		}
	}
	if err := b.toolRegistry.InitConfigs(); err != nil {
		return nil, err
	}
	return b, nil
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

func (b *Bot) CurrentRoles() (tool, vision, memory string) {
	return b.toolProvider.Name + "/" + b.toolModel,
		b.visionProvider.Name + "/" + b.visionModel,
		b.memoryProvider.Name + "/" + b.memoryModel
}

func (b *Bot) Tools() []string {
	return b.toolRegistry.List()
}

// ClearHistory 清空内存中的会话历史，开启新对话（系统提示词保留）。
func (b *Bot) ClearHistory() {
	b.history = nil
}

func (b *Bot) ContextWindow() int64 {
	if m := config.FindModel(b.provider, b.model); m != nil {
		return m.ContextWindow
	}
	return 0
}

// ContextStats 统计当前上下文的 token 使用量与窗口总大小（0 表示未配置）。
func (b *Bot) ContextStats() (used, total int64) {
	total = b.ContextWindow()
	used += estimateTokens(b.systemPrompt)
	for _, msg := range b.history {
		var content string
		switch {
		case msg.OfUser != nil:
			content = msg.OfUser.Content.OfString.Value
		case msg.OfAssistant != nil:
			content = msg.OfAssistant.Content.OfString.Value
		case msg.OfSystem != nil:
			content = msg.OfSystem.Content.OfString.Value
		}
		used += estimateTokens(content)
	}
	return used, total
}

var tke *tiktoken.Tiktoken

// Tokenize 将文本编码为 o200k_base token id 列表（去重）。
func Tokenize(text string) []int64 {
	if text == "" {
		return nil
	}
	if tke == nil {
		t, err := tiktoken.GetEncoding("o200k_base")
		if err != nil {
			return nil
		}
		tke = t
	}
	seen := make(map[int64]bool)
	var out []int64
	for _, id := range tke.Encode(text, nil, nil) {
		tid := int64(id)
		if seen[tid] {
			continue
		}
		seen[tid] = true
		out = append(out, tid)
	}
	return out
}

// estimateTokens 用 o200k_base 词表精确统计 token；
// 初始化失败（如无法下载词表）时回退为字符数/2 估算。
func estimateTokens(s string) int64 {
	if s == "" {
		return 0
	}
	if tke == nil {
		t, err := tiktoken.GetEncoding("o200k_base")
		if err != nil {
			return int64(len([]rune(s)) / 2)
		}
		tke = t
	}
	return int64(len(tke.Encode(s, nil, nil)))
}

const memoryExtractPrompt = `你是记忆提取器。从下面的对话中提取值得长期记住的信息，包括：
- 用户的个人偏好、习惯、兴趣
- 用户的个人事实（职业、所在地、家庭等）
- 项目或任务的背景信息
- 用户的长期请求或承诺
只提取确定的信息，忽略闲聊与一次性请求。
输出 JSON：{"memories": [{"content": "记忆内容", "category": "preference|fact|background|other", "importance": 1到10的整数}]}
没有新记忆时输出 {"memories": []}`

// ExtractMemories 用记忆 AI 从当前对话历史中提取新记忆。
// existing 为已存记忆列表，注入 prompt 由 AI 判断去重；
// onReasoning 非空时流式输出 AI 的思考过程。
func (b *Bot) ExtractMemories(ctx context.Context, existing []store.Memory, onReasoning func(string)) ([]store.Memory, error) {
	if b.memoryProvider.APIKey == "" {
		return nil, fmt.Errorf("记忆AI供应商 %s 未配置 api_key，请编辑 data/config.yaml", b.memoryProvider.Name)
	}
	if len(b.history) == 0 {
		return nil, errors.New("没有可提取的对话历史")
	}
	sys := memoryExtractPrompt
	if len(existing) > 0 {
		var sb strings.Builder
		sb.WriteString(sys)
		sb.WriteString("\n已有记忆（请勿重复提取）：\n")
		for _, m := range existing {
			fmt.Fprintf(&sb, "- %s (类别: %s, 重要度: %d)\n", m.Content, m.Category, m.Importance)
		}
		sys = sb.String()
	}
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(b.history)+1)
	messages = append(messages, openai.SystemMessage(sys))
	messages = append(messages, b.history...)

	params := openai.ChatCompletionNewParams{
		Model:    b.memoryModel,
		Messages: messages,
	}
	b.applyThinkingParams(&params, b.memoryProvider)
	params.SetExtraFields(map[string]any{"response_format": map[string]string{"type": "json_object"}})
	stream := b.clientFor(b.memoryProvider).Chat.Completions.NewStreaming(ctx, params)
	var content strings.Builder
	for stream.Next() {
		for _, choice := range stream.Current().Choices {
			if onReasoning != nil {
				if s := extraReasoning(choice.Delta.RawJSON()); s != "" {
					onReasoning(s)
				}
			}
			content.WriteString(choice.Delta.Content)
		}
	}
	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("记忆提取请求失败: %w", err)
	}
	text := strings.TrimSpace(content.String())
	if text == "" {
		return nil, nil
	}
	raw := gjson.Get(text, "memories").Raw
	if raw == "" {
		raw = text
	}
	var extracted []struct {
		Content    string `json:"content"`
		Category   string `json:"category"`
		Importance int    `json:"importance"`
	}
	if err := json.Unmarshal([]byte(raw), &extracted); err != nil {
		return nil, fmt.Errorf("解析记忆输出失败: %w", err)
	}
	var out []store.Memory
	for _, e := range extracted {
		c := strings.TrimSpace(e.Content)
		if c == "" {
			continue
		}
		if e.Importance < 0 {
			e.Importance = 0
		}
		if e.Importance > 10 {
			e.Importance = 10
		}
		out = append(out, store.Memory{Content: c, Category: e.Category, Importance: e.Importance})
	}
	return out, nil
}

func (b *Bot) SessionMessages() []store.Message {
	out := make([]store.Message, 0, len(b.history)+1)
	out = append(out, store.Message{Role: "system", Content: b.systemPrompt})
	for _, msg := range b.history {
		var role, content string
		switch {
		case msg.OfUser != nil:
			role, content = "user", msg.OfUser.Content.OfString.Value
		case msg.OfAssistant != nil:
			role, content = "assistant", msg.OfAssistant.Content.OfString.Value
		case msg.OfSystem != nil:
			role, content = "system", msg.OfSystem.Content.OfString.Value
		default:
			continue
		}
		if content == "" {
			continue
		}
		out = append(out, store.Message{Role: role, Content: content})
	}
	return out
}

func (b *Bot) RestoreSession(s *store.Session) {
	if s == nil {
		return
	}
	if s.SystemPrompt != "" {
		b.systemPrompt = s.SystemPrompt
	}
	var history []openai.ChatCompletionMessageParamUnion
	for _, m := range s.Messages {
		switch m.Role {
		case "user":
			history = append(history, openai.UserMessage(m.Content))
		case "assistant":
			history = append(history, openai.AssistantMessage(m.Content))
		case "system":
			if b.systemPrompt == "" || m.Content != b.systemPrompt {
				b.systemPrompt = m.Content
			}
		}
	}
	if len(history) > maxHistory {
		history = history[len(history)-maxHistory:]
	}
	b.history = history
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
			Tools:    b.toolRegistry.ParamList(),
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
			result, err := b.toolRegistry.Execute(tc.Function.Name, json.RawMessage(tc.Function.Arguments))
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
