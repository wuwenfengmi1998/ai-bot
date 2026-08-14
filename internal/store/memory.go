package store

import (
	"database/sql"
	"fmt"
	"time"
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
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)`

const createTokensMySQL = `
CREATE TABLE IF NOT EXISTS tokens (
    token_id   BIGINT PRIMARY KEY,
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
func SaveMemoryTokens(db *sql.DB, memoryID int64, tokenIDs []int64) error {
	if len(tokenIDs) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()
	for _, tid := range tokenIDs {
		if _, err := tx.Exec("INSERT OR IGNORE INTO tokens (token_id) VALUES (?)", tid); err != nil {
			return fmt.Errorf("保存 token 失败: %w", err)
		}
		if _, err := tx.Exec("INSERT OR IGNORE INTO memory_tokens (memory_id, token_id) VALUES (?, ?)", memoryID, tid); err != nil {
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
