package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	AppEnv            string
	HTTPAddr          string
	MySQLDSN          string
	PhoneDataKey      string
	AccessTokenSecret string
	WechatAppID       string
	WechatAppSecret   string
	WechatLoginMock   bool
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:            valueOrDefault("APP_ENV", "development"),
		HTTPAddr:          valueOrDefault("HTTP_ADDR", ":8080"),
		MySQLDSN:          strings.TrimSpace(os.Getenv("MYSQL_DSN")),
		PhoneDataKey:      strings.TrimSpace(os.Getenv("PHONE_DATA_KEY")),
		AccessTokenSecret: strings.TrimSpace(os.Getenv("ACCESS_TOKEN_SECRET")),
		WechatAppID:       strings.TrimSpace(os.Getenv("WECHAT_APP_ID")),
		WechatAppSecret:   strings.TrimSpace(os.Getenv("WECHAT_APP_SECRET")),
		WechatLoginMock:   strings.EqualFold(strings.TrimSpace(os.Getenv("WECHAT_LOGIN_MOCK")), "true"),
	}

	if cfg.MySQLDSN == "" {
		return Config{}, errors.New("MYSQL_DSN is required")
	}
	if cfg.PhoneDataKey == "" && cfg.AppEnv == "production" {
		return Config{}, errors.New("PHONE_DATA_KEY is required in production")
	}
	if len(cfg.AccessTokenSecret) < 32 {
		return Config{}, errors.New("ACCESS_TOKEN_SECRET must contain at least 32 characters")
	}
	if cfg.AppEnv == "production" && (cfg.WechatAppID == "" || cfg.WechatAppSecret == "") {
		return Config{}, errors.New("WECHAT_APP_ID and WECHAT_APP_SECRET are required in production")
	}
	if cfg.AppEnv == "production" && cfg.WechatLoginMock {
		return Config{}, errors.New("WECHAT_LOGIN_MOCK cannot be enabled in production")
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
