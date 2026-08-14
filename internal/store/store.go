package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"

	"myaibot/internal/config"
)

func Open(cfg *config.DatabaseConfig) (*sql.DB, error) {
	driver, dsn, err := resolve(cfg)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}
	return db, nil
}

func Close(db *sql.DB) {
	if db != nil {
		db.Close()
	}
}

func resolve(cfg *config.DatabaseConfig) (driver, dsn string, err error) {
	switch cfg.Driver {
	case "sqlite3":
		return "sqlite", cfg.File, nil
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)
		return "mysql", dsn, nil
	default:
		return "", "", fmt.Errorf("不支持的数据库驱动: %s", cfg.Driver)
	}
}
