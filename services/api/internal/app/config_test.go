package app

import (
	"testing"
	"time"
)

func TestLoadConfig_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/ivyticketing?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("API_PORT", "")
	t.Setenv("APP_ENV", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIPort != "8080" {
		t.Errorf("APIPort = %q, want 8080", cfg.APIPort)
	}
	if cfg.AppEnv != "local" {
		t.Errorf("AppEnv = %q, want local", cfg.AppEnv)
	}
}

func TestLoadConfig_MissingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for missing DATABASE_URL, got nil")
	}
}

func TestLoadConfig_AuthDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/ivyticketing?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("ACCESS_TOKEN_TTL", "")
	t.Setenv("REFRESH_TOKEN_TTL", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AccessTokenTTL != 15*time.Minute {
		t.Errorf("AccessTokenTTL = %v, want 15m", cfg.AccessTokenTTL)
	}
	if cfg.RefreshTokenTTL != 168*time.Hour {
		t.Errorf("RefreshTokenTTL = %v, want 168h", cfg.RefreshTokenTTL)
	}
}

func TestLoadConfig_MissingJWTSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/ivyticketing?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error for missing JWT_SECRET, got nil")
	}
}

func TestLoadConfig_StorageDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/ivyticketing?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("STORAGE_DRIVER", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.StorageDriver != "local" {
		t.Errorf("StorageDriver = %q, want local", cfg.StorageDriver)
	}
	if cfg.StorageLocalPath != "./var/media" {
		t.Errorf("StorageLocalPath = %q, want ./var/media", cfg.StorageLocalPath)
	}
	if cfg.StorageUploadMaxBytes != 5242880 {
		t.Errorf("StorageUploadMaxBytes = %d, want 5242880", cfg.StorageUploadMaxBytes)
	}
}

func TestLoadConfig_OrderDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/ivyticketing?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("ORDER_EXPIRATION", "")
	t.Setenv("WORKER_INTERVAL", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.OrderExpiration != 15*time.Minute {
		t.Errorf("OrderExpiration = %v, want 15m", cfg.OrderExpiration)
	}
	if cfg.WorkerInterval != time.Minute {
		t.Errorf("WorkerInterval = %v, want 1m", cfg.WorkerInterval)
	}
}

func TestLoadConfig_CloudDriverRequiresCredentials(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/ivyticketing?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("STORAGE_DRIVER", "r2")
	t.Setenv("STORAGE_BUCKET", "")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error for cloud driver without credentials, got nil")
	}
}

func TestLoadConfig_PaymentDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/x?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "secret")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WebhookPort != "8090" {
		t.Errorf("WebhookPort = %q, want 8090", cfg.WebhookPort)
	}
	if cfg.PaymentDefaultExpiry != 15*time.Minute {
		t.Errorf("PaymentDefaultExpiry = %v, want 15m", cfg.PaymentDefaultExpiry)
	}
}

func TestLoadConfig_DuitkuEnabledRequiresCreds(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/x?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("DUITKU_ENABLED", "true")
	t.Setenv("DUITKU_MERCHANT_CODE", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when DUITKU_ENABLED=true but creds missing")
	}
}
