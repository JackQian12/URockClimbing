package config

import "testing"

func TestLoadRequiresMySQLDSN(t *testing.T) {
	t.Setenv("MYSQL_DSN", "")
	t.Setenv("ACCESS_TOKEN_SECRET", "test-secret-that-is-at-least-32-characters")
	if _, err := Load(); err == nil {
		t.Fatal("expected missing MYSQL_DSN to fail")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("MYSQL_DSN", "user:pass@tcp(localhost:3306)/urock")
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("ACCESS_TOKEN_SECRET", "test-secret-that-is-at-least-32-characters")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AppEnv != "development" || cfg.HTTPAddr != ":8080" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}
