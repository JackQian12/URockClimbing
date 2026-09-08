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

func TestLoadRejectsIncompletePaymentConfiguration(t *testing.T) {
	t.Setenv("MYSQL_DSN", "user:pass@tcp(localhost:3306)/urock")
	t.Setenv("APP_ENV", "production")
	t.Setenv("PHONE_DATA_KEY", "configured")
	t.Setenv("ACCESS_TOKEN_SECRET", "test-secret-that-is-at-least-32-characters")
	t.Setenv("WECHAT_APP_ID", "wx-test")
	t.Setenv("WECHAT_APP_SECRET", "secret")
	t.Setenv("WECHAT_PAY_ENABLED", "true")
	t.Setenv("WECHAT_PAY_MCH_ID", "merchant")
	if _, err := Load(); err == nil {
		t.Fatal("expected incomplete payment configuration to fail closed")
	}
}

func TestLoadAcceptsCompleteProductionPaymentConfiguration(t *testing.T) {
	t.Setenv("MYSQL_DSN", "user:pass@tcp(localhost:3306)/urock")
	t.Setenv("APP_ENV", "production")
	t.Setenv("PHONE_DATA_KEY", "configured")
	t.Setenv("ACCESS_TOKEN_SECRET", "test-secret-that-is-at-least-32-characters")
	t.Setenv("WECHAT_APP_ID", "wx-test")
	t.Setenv("WECHAT_APP_SECRET", "secret")
	t.Setenv("WECHAT_PAY_ENABLED", "true")
	t.Setenv("WECHAT_PAY_MCH_ID", "merchant")
	t.Setenv("WECHAT_PAY_API_V3_KEY", "12345678901234567890123456789012")
	t.Setenv("WECHAT_PAY_CERT_SERIAL_NO", "serial")
	t.Setenv("WECHAT_PAY_PRIVATE_KEY_PATH", "/run/secrets/private.pem")
	t.Setenv("WECHAT_PAY_PUBLIC_KEY_ID", "PUB_KEY_ID_1")
	t.Setenv("WECHAT_PAY_PUBLIC_KEY_PATH", "/run/secrets/public.pem")
	t.Setenv("WECHAT_PAY_NOTIFY_URL", "https://api.example.com/api/v1/payments/wechat/notify")
	t.Setenv("WECHAT_REFUND_NOTIFY_URL", "https://api.example.com/api/v1/refunds/wechat/notify")
	if _, err := Load(); err != nil {
		t.Fatalf("expected complete payment configuration to load: %v", err)
	}
}
