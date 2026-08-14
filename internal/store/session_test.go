package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	"myaibot/internal/config"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	cfg := &config.DatabaseConfig{
		Driver: "sqlite3",
		File:   filepath.Join(t.TempDir(), "memory.db"),
	}
	db, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open 出错: %v", err)
	}
	t.Cleanup(func() { Close(db) })
	if err := Migrate(db, "sqlite3"); err != nil {
		t.Fatalf("Migrate 出错: %v", err)
	}
	return db
}

func TestSessionRoundtrip(t *testing.T) {
	db := openTestDB(t)
	first := &Session{
		Provider: "deepseek",
		Model:    "deepseek-v4-flash",
		Messages: []Message{{Role: "user", Content: "你好"}, {Role: "assistant", Content: "你好！"}},
	}
	id1, err := SaveSession(db, first)
	if err != nil {
		t.Fatalf("SaveSession 出错: %v", err)
	}
	second := &Session{
		Provider: "deepseek",
		Model:    "deepseek-v4-flash",
		Messages: []Message{{Role: "user", Content: "现在几点"}},
	}
	id2, err := SaveSession(db, second)
	if err != nil {
		t.Fatalf("SaveSession 出错: %v", err)
	}
	if id2 <= id1 {
		t.Errorf("id2 (%d) 应大于 id1 (%d)", id2, id1)
	}

	latest, err := LoadLatestSession(db)
	if err != nil {
		t.Fatalf("LoadLatestSession 出错: %v", err)
	}
	if latest == nil || latest.ID != id2 {
		t.Errorf("最新会话应为 #%d, got %+v", id2, latest)
	}
	if len(latest.Messages) != 1 || latest.Messages[0].Content != "现在几点" {
		t.Errorf("消息还原异常: %+v", latest.Messages)
	}

	byID, err := LoadSession(db, id1)
	if err != nil {
		t.Fatalf("LoadSession 出错: %v", err)
	}
	if byID == nil || len(byID.Messages) != 2 {
		t.Errorf("按 id 加载异常: %+v", byID)
	}

	list, err := ListSessions(db)
	if err != nil {
		t.Fatalf("ListSessions 出错: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("列表数量 = %d, want 2", len(list))
	}
	if list[0].ID != id2 || list[0].MessageCount != 1 {
		t.Errorf("列表首条应为最新会话: %+v", list[0])
	}
}

func TestLoadSessionMissing(t *testing.T) {
	db := openTestDB(t)
	sess, err := LoadSession(db, 999)
	if err != nil {
		t.Fatalf("LoadSession 出错: %v", err)
	}
	if sess != nil {
		t.Errorf("不存在的会话应返回 nil, got %+v", sess)
	}
	latest, err := LoadLatestSession(db)
	if err != nil {
		t.Fatalf("LoadLatestSession 出错: %v", err)
	}
	if latest != nil {
		t.Errorf("空库最新会话应为 nil, got %+v", latest)
	}
}
