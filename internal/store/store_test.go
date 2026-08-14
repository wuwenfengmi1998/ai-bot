package store

import (
	"path/filepath"
	"testing"

	"myaibot/internal/config"
)

func TestOpenSQLite(t *testing.T) {
	cfg := &config.DatabaseConfig{
		Driver: "sqlite3",
		File:   filepath.Join(t.TempDir(), "memory.db"),
	}
	db, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open 出错: %v", err)
	}
	defer Close(db)
	if err := db.Ping(); err != nil {
		t.Fatalf("Ping 出错: %v", err)
	}
}

func TestOpenUnsupportedDriver(t *testing.T) {
	if _, err := Open(&config.DatabaseConfig{Driver: "oracle"}); err == nil {
		t.Error("不支持的驱动应报错")
	}
}

func TestResolveMySQL(t *testing.T) {
	cfg := &config.DatabaseConfig{
		Driver:   "mysql",
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Password: "secret",
		Name:     "memory",
	}
	driver, dsn, err := resolve(cfg)
	if err != nil {
		t.Fatalf("resolve 出错: %v", err)
	}
	if driver != "mysql" {
		t.Errorf("driver = %q, want mysql", driver)
	}
	want := "root:secret@tcp(127.0.0.1:3306)/memory?charset=utf8mb4&parseTime=True&loc=Local"
	if dsn != want {
		t.Errorf("dsn = %q, want %q", dsn, want)
	}
}
