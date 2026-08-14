package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Session struct {
	ID           int64
	CreatedAt    time.Time
	Provider     string
	Model        string
	SystemPrompt string
	Messages     []Message
}

type SessionSummary struct {
	ID           int64
	CreatedAt    time.Time
	MessageCount int
}

const createSessionsSQLite = `
CREATE TABLE IF NOT EXISTS sessions (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    provider      TEXT NOT NULL DEFAULT '',
    model         TEXT NOT NULL DEFAULT '',
    system_prompt TEXT NOT NULL DEFAULT '',
    messages      TEXT NOT NULL,
    message_count INTEGER NOT NULL DEFAULT 0
)`

const createSessionsMySQL = `
CREATE TABLE IF NOT EXISTS sessions (
    id            BIGINT AUTO_INCREMENT PRIMARY KEY,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    provider      VARCHAR(255) NOT NULL DEFAULT '',
    model         VARCHAR(255) NOT NULL DEFAULT '',
    system_prompt TEXT NOT NULL,
    messages      LONGTEXT NOT NULL,
    message_count INT NOT NULL DEFAULT 0
)`

func Migrate(db *sql.DB, driver string) error {
	var ddl string
	switch driver {
	case "sqlite3":
		ddl = createSessionsSQLite
	case "mysql":
		ddl = createSessionsMySQL
	default:
		return fmt.Errorf("不支持的数据库驱动: %s", driver)
	}
	if _, err := db.Exec(ddl); err != nil {
		return fmt.Errorf("创建 sessions 表失败: %w", err)
	}
	return nil
}

func SaveSession(db *sql.DB, s *Session) (int64, error) {
	if s.Messages == nil {
		s.Messages = []Message{}
	}
	data, err := json.Marshal(s.Messages)
	if err != nil {
		return 0, fmt.Errorf("序列化消息失败: %w", err)
	}
	res, err := db.Exec(
		"INSERT INTO sessions (provider, model, system_prompt, messages, message_count) VALUES (?, ?, ?, ?, ?)",
		s.Provider, s.Model, s.SystemPrompt, string(data), len(s.Messages),
	)
	if err != nil {
		return 0, fmt.Errorf("保存会话失败: %w", err)
	}
	return res.LastInsertId()
}

func LoadLatestSession(db *sql.DB) (*Session, error) {
	return loadSession(db, "SELECT id, created_at, provider, model, system_prompt, messages FROM sessions ORDER BY id DESC LIMIT 1")
}

func LoadSession(db *sql.DB, id int64) (*Session, error) {
	return loadSession(db, "SELECT id, created_at, provider, model, system_prompt, messages FROM sessions WHERE id = ?", id)
}

func loadSession(db *sql.DB, query string, args ...any) (*Session, error) {
	row := db.QueryRow(query, args...)
	var (
		s        Session
		created  string
		messages string
	)
	if err := row.Scan(&s.ID, &created, &s.Provider, &s.Model, &s.SystemPrompt, &messages); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("读取会话失败: %w", err)
	}
	s.CreatedAt = parseTime(created)
	if err := json.Unmarshal([]byte(messages), &s.Messages); err != nil {
		return nil, fmt.Errorf("解析会话消息失败: %w", err)
	}
	return &s, nil
}

func ListSessions(db *sql.DB) ([]SessionSummary, error) {
	rows, err := db.Query("SELECT id, created_at, message_count FROM sessions ORDER BY id DESC")
	if err != nil {
		return nil, fmt.Errorf("查询会话列表失败: %w", err)
	}
	defer rows.Close()
	var out []SessionSummary
	for rows.Next() {
		var (
			sm SessionSummary
			t  string
		)
		if err := rows.Scan(&sm.ID, &t, &sm.MessageCount); err != nil {
			return nil, fmt.Errorf("读取会话列表失败: %w", err)
		}
		sm.CreatedAt = parseTime(t)
		out = append(out, sm)
	}
	return out, rows.Err()
}

func parseTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}
