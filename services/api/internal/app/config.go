package app

import (
	"fmt"
	"os"
	"strconv"
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

	StorageDriver         string
	StorageLocalPath      string
	StoragePublicBaseURL  string
	StorageUploadMaxBytes int64
	StorageBucket         string
	StorageEndpoint       string
	StorageAccessKey      string
	StorageSecretKey      string
	StorageRegion         string
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

	cfg.StorageDriver = getEnv("STORAGE_DRIVER", "local")
	cfg.StorageLocalPath = getEnv("STORAGE_LOCAL_PATH", "./var/media")
	cfg.StoragePublicBaseURL = getEnv("STORAGE_PUBLIC_BASE_URL", "http://localhost:8080")
	cfg.StorageBucket = os.Getenv("STORAGE_BUCKET")
	cfg.StorageEndpoint = os.Getenv("STORAGE_ENDPOINT")
	cfg.StorageAccessKey = os.Getenv("STORAGE_ACCESS_KEY")
	cfg.StorageSecretKey = os.Getenv("STORAGE_SECRET_KEY")
	cfg.StorageRegion = os.Getenv("STORAGE_REGION")

	maxBytes, err := getInt64("STORAGE_UPLOAD_MAX_BYTES", 5242880)
	if err != nil {
		return Config{}, err
	}
	cfg.StorageUploadMaxBytes = maxBytes

	if cfg.StorageDriver != "local" {
		if cfg.StorageBucket == "" || cfg.StorageAccessKey == "" || cfg.StorageSecretKey == "" {
			return Config{}, fmt.Errorf("config: STORAGE_BUCKET/ACCESS_KEY/SECRET_KEY required when STORAGE_DRIVER=%s", cfg.StorageDriver)
		}
	}

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

func getInt64(key string, fallback int64) (int64, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("config: %s invalid int: %w", key, err)
	}
	return n, nil
}
