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
	ids, err := SaveMemories(db, ms)
	if err != nil {
		t.Fatalf("SaveMemories 出错: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("返回 ids 数量 = %d, want 2", len(ids))
	}
	if ids[1] <= ids[0] {
		t.Errorf("ids 应递增: %v", ids)
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
	ids, err := SaveMemories(db, nil)
	if err != nil {
		t.Fatalf("空列表不应报错: %v", err)
	}
	if ids != nil {
		t.Errorf("空列表应返回 nil, got %v", ids)
	}
}

func TestMemoryTokens(t *testing.T) {
	db := openMemDB(t)
	ids, err := SaveMemories(db, []Memory{
		{Content: "用户喜欢喝咖啡"},
		{Content: "用户是 Go 开发者"},
	})
	if err != nil {
		t.Fatalf("SaveMemories 出错: %v", err)
	}
	// 同一记忆建索引两次应幂等
	for i := 0; i < 2; i++ {
		if err := SaveMemoryTokens(db, ids[0], []int64{100, 200, 100}); err != nil {
			t.Fatalf("SaveMemoryTokens 出错: %v", err)
		}
	}
	var tokenCount int64
	if err := db.QueryRow("SELECT COUNT(*) FROM tokens").Scan(&tokenCount); err != nil {
		t.Fatal(err)
	}
	if tokenCount != 2 {
		t.Errorf("token 应去重为 2, got %d", tokenCount)
	}
	var linkCount int64
	if err := db.QueryRow("SELECT COUNT(*) FROM memory_tokens").Scan(&linkCount); err != nil {
		t.Fatal(err)
	}
	if linkCount != 2 {
		t.Errorf("关联应去重为 2, got %d", linkCount)
	}

	// 另一条记忆建索引
	if err := SaveMemoryTokens(db, ids[1], []int64{200, 300}); err != nil {
		t.Fatalf("SaveMemoryTokens 出错: %v", err)
	}

	// 未索引记忆查询：全部已索引
	unindexed, err := UnindexedMemoryIDs(db)
	if err != nil {
		t.Fatalf("UnindexedMemoryIDs 出错: %v", err)
	}
	if len(unindexed) != 0 {
		t.Errorf("应无未索引记忆, got %v", unindexed)
	}

	// 新加一条未索引记忆
	ids2, err := SaveMemories(db, []Memory{{Content: "未索引"}})
	if err != nil {
		t.Fatal(err)
	}
	unindexed, err = UnindexedMemoryIDs(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(unindexed) != 1 || unindexed[0] != ids2[0] {
		t.Errorf("未索引应只有新记忆: %v", unindexed)
	}
}

func TestLoadMemory(t *testing.T) {
	db := openMemDB(t)
	ids, err := SaveMemories(db, []Memory{{Content: "测试内容", Category: "fact", Importance: 5}})
	if err != nil {
		t.Fatal(err)
	}
	m, err := LoadMemory(db, ids[0])
	if err != nil {
		t.Fatalf("LoadMemory 出错: %v", err)
	}
	if m == nil || m.Content != "测试内容" || m.Category != "fact" {
		t.Errorf("LoadMemory 异常: %+v", m)
	}
	missing, err := LoadMemory(db, 9999)
	if err != nil || missing != nil {
		t.Errorf("不存在应返回 nil: %v, %v", missing, err)
	}
}

func TestSearchMemoriesByTokens(t *testing.T) {
	db := openMemDB(t)
	ids, err := SaveMemories(db, []Memory{
		{Content: "用户喜欢喝咖啡", Category: "preference"},
		{Content: "用户喜欢喝咖啡和茶", Category: "preference"},
		{Content: "用户是 Go 开发者", Category: "fact"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// 手工建索引：记忆1 含 token 100,200；记忆2 含 100,200,300；记忆3 含 400
	if err := SaveMemoryTokens(db, ids[0], []int64{100, 200}); err != nil {
		t.Fatal(err)
	}
	if err := SaveMemoryTokens(db, ids[1], []int64{100, 200, 300}); err != nil {
		t.Fatal(err)
	}
	if err := SaveMemoryTokens(db, ids[2], []int64{400}); err != nil {
		t.Fatal(err)
	}

	// 命中数优先：记忆2 命中 3 个 token 排第一
	res, err := SearchMemoriesByTokens(db, []int64{100, 200, 300, 400}, 5)
	if err != nil {
		t.Fatalf("搜索出错: %v", err)
	}
	if len(res) != 3 {
		t.Fatalf("命中数量 = %d, want 3", len(res))
	}
	if res[0].ID != ids[1] {
		t.Errorf("命中数最多的应排第一: %v", res[0])
	}

	// limit 钳制与命中子集
	res, err = SearchMemoriesByTokens(db, []int64{100}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Errorf("limit 10 应命中 2 条, got %d", len(res))
	}
	res, err = SearchMemoriesByTokens(db, []int64{100}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Errorf("limit 1 应只返回 1 条, got %d", len(res))
	}

	// 无命中
	res, err = SearchMemoriesByTokens(db, []int64{9999}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Errorf("无命中应返回空, got %d", len(res))
	}

	// 空 token
	res, err = SearchMemoriesByTokens(db, nil, 5)
	if err != nil || res != nil {
		t.Errorf("空 token 应返回 nil, %v %v", res, err)
	}
}
