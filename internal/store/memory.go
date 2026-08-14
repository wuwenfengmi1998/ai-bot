package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"myaibot/internal/tokens"
)

type Memory struct {
	ID              int64
	CreatedAt       time.Time
	SourceSessionID int64
	Content         string
	Category        string
	Importance      int
}

const createMemoriesSQLite = `
CREATE TABLE IF NOT EXISTS memories (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    source_session_id INTEGER NOT NULL DEFAULT 0,
    content           TEXT NOT NULL,
    category          TEXT NOT NULL DEFAULT '',
    importance        INTEGER NOT NULL DEFAULT 5
)`

const createMemoriesMySQL = `
CREATE TABLE IF NOT EXISTS memories (
    id                BIGINT AUTO_INCREMENT PRIMARY KEY,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    source_session_id BIGINT NOT NULL DEFAULT 0,
    content           TEXT NOT NULL,
    category          VARCHAR(64) NOT NULL DEFAULT '',
    importance        INT NOT NULL DEFAULT 5
)`

const createMemoriesIndex = "CREATE INDEX IF NOT EXISTS idx_memories_created_at ON memories (created_at)"

const createTokensSQLite = `
CREATE TABLE IF NOT EXISTS tokens (
    token_id   INTEGER PRIMARY KEY,
    token_text TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

const createTokensMySQL = `
CREATE TABLE IF NOT EXISTS tokens (
    token_id   BIGINT PRIMARY KEY,
    token_text TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

const createMemoryTokensSQLite = `
CREATE TABLE IF NOT EXISTS memory_tokens (
    memory_id INTEGER NOT NULL,
    token_id  INTEGER NOT NULL,
    PRIMARY KEY (memory_id, token_id)
)`

const createMemoryTokensMySQL = `
CREATE TABLE IF NOT EXISTS memory_tokens (
    memory_id BIGINT NOT NULL,
    token_id  BIGINT NOT NULL,
    PRIMARY KEY (memory_id, token_id)
)`

const createMemoryTokensIndex = "CREATE INDEX IF NOT EXISTS idx_memory_tokens_token_id ON memory_tokens (token_id)"

// migrateTokens 创建 tokens 表；旧表缺失 token_text 列时补列并回填文本。
func migrateTokens(db *sql.DB, driver string) error {
	switch driver {
	case "sqlite3":
		if _, err := db.Exec(createTokensSQLite); err != nil {
			return fmt.Errorf("创建 tokens 表失败: %w", err)
		}
	case "mysql":
		if _, err := db.Exec(createTokensMySQL); err != nil {
			return fmt.Errorf("创建 tokens 表失败: %w", err)
		}
	}
	has, err := hasTokenTextColumn(db, driver)
	if err != nil {
		return err
	}
	if !has {
		ddl := "ALTER TABLE tokens ADD COLUMN token_text TEXT NOT NULL DEFAULT ''"
		if driver == "mysql" {
			ddl = "ALTER TABLE tokens ADD COLUMN token_text TEXT NOT NULL"
		}
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("tokens 表增加 token_text 列失败: %w", err)
		}
	}
	return backfillTokenTexts(db)
}

func hasTokenTextColumn(db *sql.DB, driver string) (bool, error) {
	var query string
	switch driver {
	case "sqlite3":
		query = "SELECT name FROM pragma_table_info('tokens')"
	case "mysql":
		query = "SELECT column_name FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tokens' AND COLUMN_NAME = 'token_text'"
	default:
		return false, fmt.Errorf("不支持的数据库驱动: %s", driver)
	}
	rows, err := db.Query(query)
	if err != nil {
		return false, fmt.Errorf("检查 tokens 表结构失败: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false, fmt.Errorf("读取 tokens 表结构失败: %w", err)
		}
		if name == "token_text" {
			return true, nil
		}
	}
	return false, rows.Err()
}

// backfillTokenTexts 为 token_text 为空的 token 行回填符号文本。
func backfillTokenTexts(db *sql.DB) error {
	rows, err := db.Query("SELECT token_id FROM tokens WHERE token_text = ''")
	if err != nil {
		return fmt.Errorf("查询待回填 token 失败: %w", err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("读取 token id 失败: %w", err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		text := tokens.Text(id)
		if text == "" {
			continue
		}
		if _, err := db.Exec("UPDATE tokens SET token_text = ? WHERE token_id = ? AND token_text = ''", text, id); err != nil {
			return fmt.Errorf("回填 token 文本失败: %w", err)
		}
	}
	return nil
}

func migrateMemories(db *sql.DB, driver string) error {
	var ddl string
	switch driver {
	case "sqlite3":
		ddl = createMemoriesSQLite
	case "mysql":
		ddl = createMemoriesMySQL
	default:
		return fmt.Errorf("不支持的数据库驱动: %s", driver)
	}
	if _, err := db.Exec(ddl); err != nil {
		return fmt.Errorf("创建 memories 表失败: %w", err)
	}
	if _, err := db.Exec(createMemoriesIndex); err != nil {
		return fmt.Errorf("创建 memories 索引失败: %w", err)
	}
	switch driver {
	case "sqlite3":
		ddl = createTokensSQLite
	case "mysql":
		ddl = createTokensMySQL
	}
	if _, err := db.Exec(ddl); err != nil {
		return fmt.Errorf("创建 tokens 表失败: %w", err)
	}
	if err := migrateTokens(db, driver); err != nil {
		return err
	}
	switch driver {
	case "sqlite3":
		ddl = createMemoryTokensSQLite
	case "mysql":
		ddl = createMemoryTokensMySQL
	}
	if _, err := db.Exec(ddl); err != nil {
		return fmt.Errorf("创建 memory_tokens 表失败: %w", err)
	}
	if _, err := db.Exec(createMemoryTokensIndex); err != nil {
		return fmt.Errorf("创建 memory_tokens 索引失败: %w", err)
	}
	return nil
}

// SaveMemories 逐条插入记忆，返回每条记忆的 id。
func SaveMemories(db *sql.DB, memories []Memory) ([]int64, error) {
	if len(memories) == 0 {
		return nil, nil
	}
	ids := make([]int64, 0, len(memories))
	for _, m := range memories {
		res, err := db.Exec(
			"INSERT INTO memories (source_session_id, content, category, importance) VALUES (?, ?, ?, ?)",
			m.SourceSessionID, m.Content, m.Category, m.Importance,
		)
		if err != nil {
			return nil, fmt.Errorf("保存记忆失败: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("读取记忆 id 失败: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// SaveMemoryTokens 为记忆建立 token 索引：token 去重、关联幂等。
func SaveMemoryTokens(db *sql.DB, memoryID int64, tokenList []tokens.Token) error {
	if len(tokenList) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()
	for _, t := range tokenList {
		if _, err := tx.Exec("INSERT OR IGNORE INTO tokens (token_id, token_text) VALUES (?, ?)", t.ID, t.Text); err != nil {
			return fmt.Errorf("保存 token 失败: %w", err)
		}
		if _, err := tx.Exec("INSERT OR IGNORE INTO memory_tokens (memory_id, token_id) VALUES (?, ?)", memoryID, t.ID); err != nil {
			return fmt.Errorf("建立 token 关联失败: %w", err)
		}
	}
	return tx.Commit()
}

func LoadMemory(db *sql.DB, id int64) (*Memory, error) {
	row := db.QueryRow("SELECT id, created_at, source_session_id, content, category, importance FROM memories WHERE id = ?", id)
	var (
		m       Memory
		created string
	)
	if err := row.Scan(&m.ID, &created, &m.SourceSessionID, &m.Content, &m.Category, &m.Importance); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("读取记忆失败: %w", err)
	}
	m.CreatedAt = parseTime(created)
	return &m, nil
}

// UnindexedMemoryIDs 返回尚未建立 token 索引的记忆 id。
func UnindexedMemoryIDs(db *sql.DB) ([]int64, error) {
	rows, err := db.Query("SELECT m.id FROM memories m LEFT JOIN memory_tokens mt ON m.id = mt.memory_id WHERE mt.memory_id IS NULL")
	if err != nil {
		return nil, fmt.Errorf("查询未索引记忆失败: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("读取记忆 id 失败: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SearchMemoriesByTokens 按 token id 搜索相关记忆，按命中 token 数从多到少排序。
// limit 钳制在 1-10。
func SearchMemoriesByTokens(db *sql.DB, tokenIDs []int64, limit int) ([]Memory, error) {
	if len(tokenIDs) == 0 {
		return nil, nil
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 10 {
		limit = 10
	}
	placeholders := make([]string, len(tokenIDs))
	args := make([]any, 0, len(tokenIDs)+1)
	for i, tid := range tokenIDs {
		placeholders[i] = "?"
		args = append(args, tid)
	}
	args = append(args, limit)
	query := `SELECT m.id, m.created_at, m.source_session_id, m.content, m.category, m.importance
FROM memory_tokens mt JOIN memories m ON m.id = mt.memory_id
WHERE mt.token_id IN (` + strings.Join(placeholders, ", ") + `)
GROUP BY m.id
ORDER BY COUNT(*) DESC, m.id DESC
LIMIT ?`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("搜索记忆失败: %w", err)
	}
	defer rows.Close()
	var out []Memory
	for rows.Next() {
		var (
			m       Memory
			created string
		)
		if err := rows.Scan(&m.ID, &created, &m.SourceSessionID, &m.Content, &m.Category, &m.Importance); err != nil {
			return nil, fmt.Errorf("读取记忆失败: %w", err)
		}
		m.CreatedAt = parseTime(created)
		out = append(out, m)
	}
	return out, rows.Err()
}

func ListMemories(db *sql.DB) ([]Memory, error) {
	rows, err := db.Query("SELECT id, created_at, source_session_id, content, category, importance FROM memories ORDER BY id DESC")
	if err != nil {
		return nil, fmt.Errorf("查询记忆失败: %w", err)
	}
	defer rows.Close()
	var out []Memory
	for rows.Next() {
		var (
			m       Memory
			created string
		)
		if err := rows.Scan(&m.ID, &created, &m.SourceSessionID, &m.Content, &m.Category, &m.Importance); err != nil {
			return nil, fmt.Errorf("读取记忆失败: %w", err)
		}
		m.CreatedAt = parseTime(created)
		out = append(out, m)
	}
	return out, rows.Err()
}

func MemoryCount(db *sql.DB) (int64, error) {
	var n int64
	if err := db.QueryRow("SELECT COUNT(*) FROM memories").Scan(&n); err != nil {
		return 0, fmt.Errorf("统计记忆失败: %w", err)
	}
	return n, nil
}
