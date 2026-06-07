package app

import "testing"

func TestLoadConfig_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/ivyticketing?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
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

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for missing DATABASE_URL, got nil")
	}
}
