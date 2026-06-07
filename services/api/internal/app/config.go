package app

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	AppEnv          string
	AppName         string
	APIPort         string
	DatabaseURL     string
	RedisURL        string
	WebOrigin       string
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

func LoadConfig() (Config, error) {
	cfg := Config{
		AppEnv:      getEnv("APP_ENV", "local"),
		AppName:     getEnv("APP_NAME", "ivyticketing"),
		APIPort:     getEnv("API_PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		WebOrigin:   getEnv("WEB_ORIGIN", "http://localhost:4321"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("config: DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return Config{}, fmt.Errorf("config: REDIS_URL is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("config: JWT_SECRET is required")
	}

	accessTTL, err := getDuration("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	refreshTTL, err := getDuration("REFRESH_TOKEN_TTL", 168*time.Hour)
	if err != nil {
		return Config{}, err
	}
	cfg.AccessTokenTTL = accessTTL
	cfg.RefreshTokenTTL = refreshTTL

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("config: %s invalid duration: %w", key, err)
	}
	return d, nil
}
