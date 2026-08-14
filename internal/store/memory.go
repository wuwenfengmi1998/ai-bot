package store

import (
	"database/sql"
	"fmt"
	"strings"
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
	return nil
}

func SaveMemories(db *sql.DB, memories []Memory) (int64, error) {
	if len(memories) == 0 {
		return 0, nil
	}
	placeholders := make([]string, 0, len(memories))
	args := make([]any, 0, len(memories)*4)
	for _, m := range memories {
		placeholders = append(placeholders, "(?, ?, ?, ?)")
		args = append(args, m.SourceSessionID, m.Content, m.Category, m.Importance)
	}
	query := "INSERT INTO memories (source_session_id, content, category, importance) VALUES " +
		strings.Join(placeholders, ", ")
	res, err := db.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("保存记忆失败: %w", err)
	}
	return res.LastInsertId()
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
