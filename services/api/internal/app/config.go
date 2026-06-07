package app

import (
	"fmt"
	"os"
)

type Config struct {
	AppEnv      string
	AppName     string
	APIPort     string
	DatabaseURL string
	RedisURL    string
	WebOrigin   string
}

func LoadConfig() (Config, error) {
	cfg := Config{
		AppEnv:      getEnv("APP_ENV", "local"),
		AppName:     getEnv("APP_NAME", "ivyticketing"),
		APIPort:     getEnv("API_PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		WebOrigin:   getEnv("WEB_ORIGIN", "http://localhost:4321"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("config: DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return Config{}, fmt.Errorf("config: REDIS_URL is required")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
