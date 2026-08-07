package config

import "testing"

func TestLoadRequiresMySQLDSN(t *testing.T) {
	t.Setenv("MYSQL_DSN", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected missing MYSQL_DSN to fail")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("MYSQL_DSN", "user:pass@tcp(localhost:3306)/urock")
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_ADDR", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AppEnv != "development" || cfg.HTTPAddr != ":8080" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}
