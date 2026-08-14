package builtin

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"myaibot/internal/config"
	"myaibot/internal/store"
	"myaibot/internal/tokens"
)

func recallTestDB(t *testing.T) *sql.DB {
	t.Helper()
	cfg := &config.DatabaseConfig{
		Driver: "sqlite3",
		File:   filepath.Join(t.TempDir(), "memory.db"),
	}
	db, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("Open 出错: %v", err)
	}
	t.Cleanup(func() { store.Close(db) })
	if err := store.Migrate(db, "sqlite3"); err != nil {
		t.Fatalf("Migrate 出错: %v", err)
	}
	ids, err := store.SaveMemories(db, []store.Memory{
		{Content: "用户喜欢喝咖啡", Category: "preference", Importance: 7},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveMemoryTokens(db, ids[0], tokens.Tokenize("用户喜欢喝咖啡")); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestRecallToolFound(t *testing.T) {
	tool := NewRecallTool(recallTestDB(t))
	args, _ := json.Marshal(map[string]any{"query": "咖啡"})
	out, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("Execute 出错: %v", err)
	}
	if !strings.Contains(out, "找到 1 条相关记忆") || !strings.Contains(out, "用户喜欢喝咖啡") {
		t.Errorf("输出异常: %q", out)
	}
}

func TestRecallToolNotFound(t *testing.T) {
	tool := NewRecallTool(recallTestDB(t))
	args, _ := json.Marshal(map[string]any{"query": "不存在的关键词"})
	out, err := tool.Execute(args)
	if err != nil {
		t.Fatalf("Execute 出错: %v", err)
	}
	if out != "未找到相关记忆" {
		t.Errorf("应返回未找到, got %q", out)
	}
}

func TestRecallToolEmptyQuery(t *testing.T) {
	tool := NewRecallTool(recallTestDB(t))
	args, _ := json.Marshal(map[string]any{"query": ""})
	if _, err := tool.Execute(args); err == nil {
		t.Error("空 query 应报错")
	}
}

func TestRecallToolConfigure(t *testing.T) {
	tool := NewRecallTool(nil)
	if err := tool.Configure(map[string]any{"enabled": false, "prompt": "自定义提示"}); err != nil {
		t.Fatalf("Configure 出错: %v", err)
	}
	if tool.Enabled() {
		t.Error("应被禁用")
	}
	if tool.Description() != "自定义提示" {
		t.Errorf("Description = %q", tool.Description())
	}
}
