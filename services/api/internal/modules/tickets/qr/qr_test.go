package qr

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestSignVerify_Roundtrip(t *testing.T) {
	s := NewSigner("secret-key")
	tid := uuid.New()
	eid := uuid.New()

	tok, err := s.Sign(tid, eid)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	ref, err := s.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if ref.TicketID != tid || ref.EventID != eid {
		t.Fatalf("ref mismatch: got %+v", ref)
	}
	if ref.Version != 1 {
		t.Fatalf("version = %d, want 1", ref.Version)
	}
}

func TestVerify_WrongSecret(t *testing.T) {
	tok, _ := NewSigner("secret-a").Sign(uuid.New(), uuid.New())
	if _, err := NewSigner("secret-b").Verify(tok); err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestVerify_TamperedPayload(t *testing.T) {
	s := NewSigner("secret-key")
	tok, _ := s.Sign(uuid.New(), uuid.New())
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected token shape: %q", tok)
	}
	// flip a char in the payload segment
	payload := []byte(parts[1])
	payload[0] ^= 0x01
	tampered := parts[0] + "." + string(payload) + "." + parts[2]
	if _, err := s.Verify(tampered); err == nil {
		t.Fatal("expected error for tampered payload, got nil")
	}
}

func TestVerify_MalformedToken(t *testing.T) {
	s := NewSigner("secret-key")
	for _, bad := range []string{"", "a.b", "a.b.c.d", "not-a-token"} {
		if _, err := s.Verify(bad); err == nil {
			t.Fatalf("expected error for malformed token %q, got nil", bad)
		}
	}
}

func TestVerify_UnknownVersion(t *testing.T) {
	s := NewSigner("secret-key")
	tok, _ := s.Sign(uuid.New(), uuid.New())
	parts := strings.Split(tok, ".")
	// replace version segment with "9"
	bad := "9." + parts[1] + "." + parts[2]
	if _, err := s.Verify(bad); err == nil {
		t.Fatal("expected error for unknown version, got nil")
	}
}
