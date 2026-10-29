package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileForEnv(t *testing.T) {
	tests := []struct {
		dir, env, want string
	}{
		{"etc", "dev", "etc/config_dev.yaml"},
		{"etc", "prod", "etc/config_prod.yaml"},
		{"etc", "", "etc/config_dev.yaml"}, // empty env falls back to dev
		{"conf", "test", "conf/config_test.yaml"},
	}
	for _, tt := range tests {
		if got := FileForEnv(tt.dir, tt.env); got != tt.want {
			t.Errorf("FileForEnv(%q,%q) = %q, want %q", tt.dir, tt.env, got, tt.want)
		}
	}
}

func TestLoadAppliesDefaultsAndFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: 9090\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want default 0.0.0.0", cfg.Server.Host)
	}
	if cfg.Log.Format != "console" {
		t.Errorf("Log.Format = %q, want default console", cfg.Log.Format)
	}
	if cfg.MySQL.Port != 3306 {
		t.Errorf("MySQL.Port = %d, want default 3306", cfg.MySQL.Port)
	}
	if cfg.MySQL.Charset != "utf8mb4" {
		t.Errorf("MySQL.Charset = %q, want default utf8mb4", cfg.MySQL.Charset)
	}
	if cfg.Redis.DB != 0 {
		t.Errorf("Redis.DB = %d, want default 0", cfg.Redis.DB)
	}
}

func TestMySQLDSNAndRedisAddr(t *testing.T) {
	cfg := Config{
		MySQL: MySQLConf{
			Host:     "db.example.com",
			Port:     3307,
			User:     "app",
			Password: "s3cret",
			Database: "gin_scaffold",
			Charset:  "utf8mb4",
		},
		Redis: RedisConf{Host: "cache.example.com", Port: 6380},
	}

	want := "app:s3cret@tcp(db.example.com:3307)/gin_scaffold?charset=utf8mb4&parseTime=true&loc=Local"
	if got := cfg.MySQL.DSN(); got != want {
		t.Errorf("MySQL.DSN() = %q, want %q", got, want)
	}
	if got := cfg.Redis.Addr(); got != "cache.example.com:6380" {
		t.Errorf("Redis.Addr() = %q, want cache.example.com:6380", got)
	}
}

func TestLoadEnvOverride(t *testing.T) {
	t.Setenv("SERVER_PORT", "9099")

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: 8080\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != 9099 {
		t.Errorf("Server.Port = %d, want env override 9099", cfg.Server.Port)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("Load() expected error for missing file, got nil")
	}
}
