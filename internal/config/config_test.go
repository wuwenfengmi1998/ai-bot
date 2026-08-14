package config

import "testing"

func TestApplyDatabaseDefaults(t *testing.T) {
	c := &Config{Providers: []Provider{{Name: "p", BaseURL: "x", Models: []string{"m"}}}}
	applyDefaults(c)
	if c.Database.Driver != "sqlite3" {
		t.Errorf("默认 driver = %q, want sqlite3", c.Database.Driver)
	}
	if c.Database.File != "data/memory.db" {
		t.Errorf("默认 file = %q, want data/memory.db", c.Database.File)
	}
}

func TestApplyMySQLDefaults(t *testing.T) {
	c := &Config{
		Providers: []Provider{{Name: "p", BaseURL: "x", Models: []string{"m"}}},
		Database:  DatabaseConfig{Driver: "mysql", Name: "memory"},
	}
	applyDefaults(c)
	if c.Database.Host != "127.0.0.1" {
		t.Errorf("默认 host = %q, want 127.0.0.1", c.Database.Host)
	}
	if c.Database.Port != 3306 {
		t.Errorf("默认 port = %d, want 3306", c.Database.Port)
	}
}

func TestValidateDatabase(t *testing.T) {
	c := &Config{
		DefaultProvider: "p",
		DefaultModel:    "m",
		Providers:       []Provider{{Name: "p", BaseURL: "x", Models: []string{"m"}}},
		Database:        DatabaseConfig{Driver: "oracle"},
	}
	cfg = c
	if err := validate(c); err == nil {
		t.Error("非法驱动应报错")
	}
	c.Database = DatabaseConfig{Driver: "mysql"}
	if err := validate(c); err == nil {
		t.Error("mysql 缺 name 应报错")
	}
	c.Database = DatabaseConfig{Driver: "mysql", Name: "memory"}
	if err := validate(c); err != nil {
		t.Errorf("合法 mysql 配置不应报错: %v", err)
	}
	c.Database = DatabaseConfig{Driver: "sqlite3"}
	if err := validate(c); err != nil {
		t.Errorf("合法 sqlite3 配置不应报错: %v", err)
	}
}
