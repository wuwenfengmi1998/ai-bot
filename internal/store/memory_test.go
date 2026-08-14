package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	"myaibot/internal/config"
)

func openMemDB(t *testing.T) *sql.DB {
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

func TestMemoriesRoundtrip(t *testing.T) {
	db := openMemDB(t)
	ms := []Memory{
		{Content: "用户喜欢喝咖啡", Category: "preference", Importance: 7},
		{Content: "用户是 Go 开发者", Category: "fact", Importance: 9, SourceSessionID: 3},
	}
	if _, err := SaveMemories(db, ms); err != nil {
		t.Fatalf("SaveMemories 出错: %v", err)
	}
	if n, err := MemoryCount(db); err != nil || n != 2 {
		t.Errorf("MemoryCount = %d, %v; want 2", n, err)
	}
	list, err := ListMemories(db)
	if err != nil {
		t.Fatalf("ListMemories 出错: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("列表数量 = %d, want 2", len(list))
	}
	if list[0].Content != "用户是 Go 开发者" || list[0].Importance != 9 || list[0].SourceSessionID != 3 {
		t.Errorf("最新记忆应为 Go 开发者: %+v", list[0])
	}
	if list[1].Content != "用户喜欢喝咖啡" || list[1].Category != "preference" {
		t.Errorf("记忆顺序/内容异常: %+v", list[1])
	}
}

func TestSaveMemoriesEmpty(t *testing.T) {
	db := openMemDB(t)
	if _, err := SaveMemories(db, nil); err != nil {
		t.Fatalf("空列表不应报错: %v", err)
	}
}
