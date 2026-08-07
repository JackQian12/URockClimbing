package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	AppEnv       string
	HTTPAddr     string
	MySQLDSN     string
	PhoneDataKey string
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:       valueOrDefault("APP_ENV", "development"),
		HTTPAddr:     valueOrDefault("HTTP_ADDR", ":8080"),
		MySQLDSN:     strings.TrimSpace(os.Getenv("MYSQL_DSN")),
		PhoneDataKey: strings.TrimSpace(os.Getenv("PHONE_DATA_KEY")),
	}

	if cfg.MySQLDSN == "" {
		return Config{}, errors.New("MYSQL_DSN is required")
	}
	if cfg.PhoneDataKey == "" && cfg.AppEnv == "production" {
		return Config{}, errors.New("PHONE_DATA_KEY is required in production")
	}
	if !strings.HasPrefix(cfg.HTTPAddr, ":") && !strings.Contains(cfg.HTTPAddr, ":") {
		return Config{}, fmt.Errorf("HTTP_ADDR must be host:port or :port, got %q", cfg.HTTPAddr)
	}
	return cfg, nil
}

func valueOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
