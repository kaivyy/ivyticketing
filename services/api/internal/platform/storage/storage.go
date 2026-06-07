package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var ErrNotConfigured = errors.New("storage: driver not configured")

// Storage abstracts object storage across local disk and S3-compatible clouds.
type Storage interface {
	// PresignUpload returns a direct-to-storage upload ticket if supported.
	// ok=false means the backend cannot presign (local) — use Put instead.
	PresignUpload(ctx context.Context, key, contentType string, ttl time.Duration) (PutTicket, bool, error)
	// Put writes bytes directly (local driver, or fallback).
	Put(ctx context.Context, key string, r io.Reader, contentType string) error
	// PublicURL builds the readable URL for a stored object.
	PublicURL(key string) string
	// Delete removes an object (best-effort).
	Delete(ctx context.Context, key string) error
}

type PutTicket struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Expires time.Time         `json:"expires"`
}

type Config struct {
	Driver        string
	LocalPath     string
	PublicBaseURL string
	Bucket        string
	Endpoint      string
	AccessKey     string
	SecretKey     string
	Region        string
}

// New builds a Storage from config.
func New(cfg Config) (Storage, error) {
	switch cfg.Driver {
	case "local", "":
		return NewLocal(cfg.LocalPath, cfg.PublicBaseURL)
	case "r2", "tencent", "s3":
		return NewS3(cfg)
	default:
		return nil, errors.New("storage: unknown driver " + cfg.Driver)
	}
}
