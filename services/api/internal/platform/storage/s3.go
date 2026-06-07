package storage

import (
	"context"
	"io"
	"strings"
	"time"
)

// S3 is an S3-compatible driver (R2/Tencent/AWS). Phase 3 ships the contract;
// the implementation is filled in when cloud credentials are available.
// Until then every operation returns ErrNotConfigured.
type S3 struct {
	cfg Config
}

func NewS3(cfg Config) (*S3, error) {
	return &S3{cfg: cfg}, nil
}

// PresignUpload will issue a presigned PUT URL (direct-to-storage upload) so
// the API never proxies file bytes. Contract: ok=true on success.
func (s *S3) PresignUpload(_ context.Context, _, _ string, _ time.Duration) (PutTicket, bool, error) {
	return PutTicket{}, false, ErrNotConfigured
}

func (s *S3) Put(_ context.Context, _ string, _ io.Reader, _ string) error {
	return ErrNotConfigured
}

func (s *S3) PublicURL(key string) string {
	base := strings.TrimRight(s.cfg.PublicBaseURL, "/")
	return base + "/" + key
}

func (s *S3) Delete(_ context.Context, _ string) error {
	return ErrNotConfigured
}
