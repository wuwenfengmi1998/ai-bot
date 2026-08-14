package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	"myaibot/internal/config"
	"myaibot/internal/tokens"
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
	tok := []tokens.Token{{ID: 100, Text: " 100"}, {ID: 200, Text: " 200"}}
	for i := 0; i < 2; i++ {
		if err := SaveMemoryTokens(db, ids[0], tok); err != nil {
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
	var text string
	if err := db.QueryRow("SELECT token_text FROM tokens WHERE token_id = 100").Scan(&text); err != nil {
		t.Fatal(err)
	}
	if text != " 100" {
		t.Errorf("token_text 应为 \" 100\", got %q", text)
	}
	var linkCount int64
	if err := db.QueryRow("SELECT COUNT(*) FROM memory_tokens").Scan(&linkCount); err != nil {
		t.Fatal(err)
	}
	if linkCount != 2 {
		t.Errorf("关联应去重为 2, got %d", linkCount)
	}

	// 另一条记忆建索引
	if err := SaveMemoryTokens(db, ids[1], []tokens.Token{{ID: 200, Text: " 200"}, {ID: 300, Text: " 300"}}); err != nil {
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

func TestMigrateLegacyTokensTable(t *testing.T) {
	db := openMemDB(t)
	// 模拟旧库：删除新结构 tokens 表，重建无 token_text 的旧表并插入旧数据
	if _, err := db.Exec("DROP TABLE tokens"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE tokens (
		token_id   INTEGER PRIMARY KEY,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO tokens (token_id) VALUES (100), (200)"); err != nil {
		t.Fatal(err)
	}

	// 重新执行迁移：应补列并回填文本
	if err := Migrate(db, "sqlite3"); err != nil {
		t.Fatalf("Migrate 出错: %v", err)
	}
	var count int64
	if err := db.QueryRow("SELECT COUNT(*) FROM tokens WHERE token_text = ''").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("回填后不应有空的 token_text, 剩余 %d", count)
	}
	var text string
	if err := db.QueryRow("SELECT token_text FROM tokens WHERE token_id = 100").Scan(&text); err != nil {
		t.Fatal(err)
	}
	if text == "" {
		t.Error("token_id 100 的文本应已回填")
	}
	// 幂等：再次迁移不报错
	if err := Migrate(db, "sqlite3"); err != nil {
		t.Fatalf("重复迁移出错: %v", err)
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
	if err := SaveMemoryTokens(db, ids[0], []tokens.Token{{ID: 100, Text: " 100"}, {ID: 200, Text: " 200"}}); err != nil {
		t.Fatal(err)
	}
	if err := SaveMemoryTokens(db, ids[1], []tokens.Token{{ID: 100, Text: " 100"}, {ID: 200, Text: " 200"}, {ID: 300, Text: " 300"}}); err != nil {
		t.Fatal(err)
	}
	if err := SaveMemoryTokens(db, ids[2], []tokens.Token{{ID: 400, Text: " 400"}}); err != nil {
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

// 回归：大小写规范化后，索引大写内容、小写查询应命中
func TestSearchCaseInsensitive(t *testing.T) {
	db := openMemDB(t)
	ids, err := SaveMemories(db, []Memory{
		{Content: "用户朋友Kevin的生日是9月12日", Category: "fact"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveMemoryTokens(db, ids[0], tokens.Tokenize("用户朋友Kevin的生日是9月12日")); err != nil {
		t.Fatal(err)
	}
	res, err := SearchMemoriesByTokens(db, tokens.TokenIDs(tokens.Tokenize("kevin 是谁")), 5)
	if err != nil {
		t.Fatalf("搜索出错: %v", err)
	}
	if len(res) != 1 || res[0].ID != ids[0] {
		t.Errorf("小写查询应命中大写索引的记忆: %+v", res)
	}
}

func TestRebuildTokenIndex(t *testing.T) {
	db := openMemDB(t)
	ids, err := SaveMemories(db, []Memory{
		{Content: "用户喜欢咖啡"},
		{Content: "用户喜欢茶"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveMemoryTokens(db, ids[0], []tokens.Token{{ID: 1, Text: " 1"}}); err != nil {
		t.Fatal(err)
	}
	n, err := RebuildTokenIndex(db)
	if err != nil {
		t.Fatalf("RebuildTokenIndex 出错: %v", err)
	}
	if n != 2 {
		t.Errorf("重建数量 = %d, want 2", n)
	}
	var count int64
	if err := db.QueryRow("SELECT COUNT(*) FROM tokens").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Error("重建后 tokens 不应为空")
	}
	// 旧 token 1 已被清空重建
	var oldExists int64
	if err := db.QueryRow("SELECT COUNT(*) FROM tokens WHERE token_id = 1").Scan(&oldExists); err != nil {
		t.Fatal(err)
	}
	if oldExists != 0 {
		t.Error("旧 token 应被清空")
	}
}
