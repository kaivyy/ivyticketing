package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Local struct {
	root          string
	publicBaseURL string
}

func NewLocal(root, publicBaseURL string) (*Local, error) {
	if root == "" {
		return nil, errors.New("storage: local path is empty")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Local{root: root, publicBaseURL: strings.TrimRight(publicBaseURL, "/")}, nil
}

// safePath resolves key under root and rejects any key containing ".." segments.
func (l *Local) safePath(key string) (string, error) {
	// Reject keys that contain ".." segments before any cleaning.
	for _, part := range strings.Split(filepath.ToSlash(key), "/") {
		if part == ".." {
			return "", errors.New("storage: invalid key")
		}
	}
	full := filepath.Join(l.root, filepath.FromSlash(key))
	rel, err := filepath.Rel(l.root, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errors.New("storage: invalid key")
	}
	return full, nil
}

func (l *Local) PresignUpload(_ context.Context, _, _ string, _ time.Duration) (PutTicket, bool, error) {
	return PutTicket{}, false, nil
}

func (l *Local) Put(_ context.Context, key string, r io.Reader, _ string) error {
	full, err := l.safePath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	f, err := os.Create(full)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (l *Local) PublicURL(key string) string {
	return l.publicBaseURL + "/media/" + key
}

func (l *Local) Delete(_ context.Context, key string) error {
	full, err := l.safePath(key)
	if err != nil {
		return err
	}
	err = os.Remove(full)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
