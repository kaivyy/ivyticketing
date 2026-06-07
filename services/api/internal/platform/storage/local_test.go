package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLocal_PutAndPublicURL(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocal(dir, "http://localhost:8080")
	if err != nil {
		t.Fatalf("new local: %v", err)
	}
	key := "org/o1/event/e1/banner/abc.png"
	if err := s.Put(context.Background(), key, strings.NewReader("imgdata"), "image/png"); err != nil {
		t.Fatalf("put: %v", err)
	}
	// File written under dir/key.
	b, err := os.ReadFile(filepath.Join(dir, key))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(b) != "imgdata" {
		t.Errorf("content = %q, want imgdata", string(b))
	}
	if got := s.PublicURL(key); got != "http://localhost:8080/media/"+key {
		t.Errorf("PublicURL = %q", got)
	}
}

func TestLocal_PresignNotSupported(t *testing.T) {
	s, _ := NewLocal(t.TempDir(), "http://localhost:8080")
	_, ok, err := s.PresignUpload(context.Background(), "k", "image/png", time.Minute)
	if err != nil {
		t.Fatalf("presign: %v", err)
	}
	if ok {
		t.Error("local should not support presign (ok=true)")
	}
}

func TestLocal_RejectsPathTraversal(t *testing.T) {
	s, _ := NewLocal(t.TempDir(), "http://localhost:8080")
	if err := s.Put(context.Background(), "../escape.png", strings.NewReader("x"), "image/png"); err == nil {
		t.Fatal("expected error for path traversal key, got nil")
	}
}
