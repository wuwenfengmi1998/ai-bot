package builtin

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"myaibot/internal/store"
	"myaibot/internal/tokens"
)

type recallTool struct {
	db      *sql.DB
	enabled bool
	prompt  string
}

func NewRecallTool(db *sql.DB) *recallTool {
	return &recallTool{
		db:      db,
		enabled: true,
		prompt:  "从长期记忆中回忆与用户提问相关的信息。当用户询问个人偏好、个人信息、之前聊过的话题或需要回顾历史对话时调用此工具",
	}
}

func (t *recallTool) Name() string        { return "recall_memory" }
func (t *recallTool) Description() string { return t.prompt }
func (t *recallTool) Enabled() bool       { return t.enabled }
func (t *recallTool) DefaultConfig() map[string]any {
	return map[string]any{"enabled": true, "prompt": t.prompt}
}

func (t *recallTool) Configure(cfg map[string]any) error {
	var err error
	if t.enabled, err = parseEnabled(cfg); err != nil {
		return err
	}
	if p, ok := cfg["prompt"].(string); ok && p != "" {
		t.prompt = p
	}
	return nil
}

func (t *recallTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string", "description": "要回忆的内容或用户的提问"},
			"limit": map[string]any{"type": "integer", "description": "最多返回的记忆条数，默认 5，最大 10"},
		},
		"required": []string{"query"},
	}
}

func (t *recallTool) Execute(args json.RawMessage) (string, error) {
	var p struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", err
	}
	query := strings.TrimSpace(p.Query)
	if query == "" {
		return "", fmt.Errorf("query 不能为空")
	}
	if p.Limit == 0 {
		p.Limit = 5
	}
	memories, err := store.SearchMemoriesByTokens(t.db, tokens.Tokenize(query), p.Limit)
	if err != nil {
		return "", err
	}
	if len(memories) == 0 {
		return "未找到相关记忆", nil
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "找到 %d 条相关记忆：\n", len(memories))
	for _, m := range memories {
		fmt.Fprintf(&sb, "- [%s %d] %s\n", m.Category, m.Importance, m.Content)
	}
	return strings.TrimSuffix(sb.String(), "\n"), nil
}
