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
	TicketQRSecret  string
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

	OrderExpiration time.Duration
	WorkerInterval  time.Duration

	QueueReleaseInterval    time.Duration
	QueueCheckoutWindow     time.Duration
	QueueDefaultReleaseRate int

	// Payments / webhook
	WebhookPort            string
	PaymentCallbackBaseURL string
	PaymentDefaultExpiry   time.Duration

	DuitkuEnabled      bool
	DuitkuMerchantCode string
	DuitkuAPIKey       string
	DuitkuEnv          string

	XenditEnabled       bool
	XenditSecretKey     string
	XenditCallbackToken string
	XenditEnv           string

	TurnstileSecret              string
	TurnstileSiteKey             string
	MaxActiveQueuePerUser        int
	ReputationChallengeThreshold int
	ReputationDenyThreshold      int
	AbuseSettingsRefresh         time.Duration

	EmailDriver      string
	SMTPHost         string
	SMTPPort         string
	SMTPUser         string
	SMTPPass         string
	EmailFromName    string
	EmailFromAddress string
}

func LoadConfig() (Config, error) {
	cfg := Config{
		AppEnv:      getEnv("APP_ENV", "local"),
		AppName:     getEnv("APP_NAME", "ivyticketing"),
		APIPort:     getEnv("API_PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		WebOrigin:   getEnv("WEB_ORIGIN", "http://localhost:4321"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		TicketQRSecret: os.Getenv("TICKET_QR_SECRET"),
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
	if cfg.TicketQRSecret == "" {
		return Config{}, fmt.Errorf("config: TICKET_QR_SECRET is required")
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

	orderExp, err := getDuration("ORDER_EXPIRATION", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	cfg.OrderExpiration = orderExp

	workerInterval, err := getDuration("WORKER_INTERVAL", time.Minute)
	if err != nil {
		return Config{}, err
	}
	cfg.WorkerInterval = workerInterval

	qInterval, err := getDuration("QUEUE_RELEASE_INTERVAL", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	qWindow, err := getDuration("QUEUE_CHECKOUT_WINDOW", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	qRate, err := getInt64("QUEUE_DEFAULT_RELEASE_RATE", 100)
	if err != nil {
		return Config{}, err
	}
	cfg.QueueReleaseInterval = qInterval
	cfg.QueueCheckoutWindow = qWindow
	cfg.QueueDefaultReleaseRate = int(qRate)

	cfg.WebhookPort = getEnv("WEBHOOK_PORT", "8090")
	cfg.PaymentCallbackBaseURL = os.Getenv("PAYMENT_CALLBACK_BASE_URL")

	payExpiry, err := getDuration("PAYMENT_DEFAULT_EXPIRY", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	cfg.PaymentDefaultExpiry = payExpiry

	cfg.DuitkuEnabled = getEnv("DUITKU_ENABLED", "false") == "true"
	cfg.DuitkuMerchantCode = os.Getenv("DUITKU_MERCHANT_CODE")
	cfg.DuitkuAPIKey = os.Getenv("DUITKU_API_KEY")
	cfg.DuitkuEnv = getEnv("DUITKU_ENV", "sandbox")
	if cfg.DuitkuEnabled && (cfg.DuitkuMerchantCode == "" || cfg.DuitkuAPIKey == "") {
		return Config{}, fmt.Errorf("config: DUITKU_MERCHANT_CODE/DUITKU_API_KEY required when DUITKU_ENABLED=true")
	}

	cfg.XenditEnabled = getEnv("XENDIT_ENABLED", "false") == "true"
	cfg.XenditSecretKey = os.Getenv("XENDIT_SECRET_KEY")
	cfg.XenditCallbackToken = os.Getenv("XENDIT_CALLBACK_TOKEN")
	cfg.XenditEnv = getEnv("XENDIT_ENV", "sandbox")
	if cfg.XenditEnabled && (cfg.XenditSecretKey == "" || cfg.XenditCallbackToken == "") {
		return Config{}, fmt.Errorf("config: XENDIT_SECRET_KEY/XENDIT_CALLBACK_TOKEN required when XENDIT_ENABLED=true")
	}

	cfg.TurnstileSecret = os.Getenv("TURNSTILE_SECRET")
	cfg.TurnstileSiteKey = os.Getenv("TURNSTILE_SITE_KEY")

	maxQueue, err := getInt64("MAX_ACTIVE_QUEUE_PER_USER", 5)
	if err != nil {
		return Config{}, err
	}
	repChallenge, err := getInt64("REPUTATION_CHALLENGE_THRESHOLD", 10)
	if err != nil {
		return Config{}, err
	}
	repDeny, err := getInt64("REPUTATION_DENY_THRESHOLD", 25)
	if err != nil {
		return Config{}, err
	}
	abuseRefresh, err := getDuration("ABUSE_SETTINGS_REFRESH", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	cfg.MaxActiveQueuePerUser = int(maxQueue)
	cfg.ReputationChallengeThreshold = int(repChallenge)
	cfg.ReputationDenyThreshold = int(repDeny)
	cfg.AbuseSettingsRefresh = abuseRefresh

	cfg.EmailDriver = getEnv("EMAIL_DRIVER", "log")
	cfg.SMTPHost = os.Getenv("SMTP_HOST")
	cfg.SMTPPort = getEnv("SMTP_PORT", "587")
	cfg.SMTPUser = os.Getenv("SMTP_USER")
	cfg.SMTPPass = os.Getenv("SMTP_PASS")
	cfg.EmailFromName = getEnv("EMAIL_FROM_NAME", "IvyTicketing")
	cfg.EmailFromAddress = os.Getenv("EMAIL_FROM_ADDRESS")

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
