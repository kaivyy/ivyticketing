package tickets

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateTicketNumber_Format(t *testing.T) {
	now := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	n, err := generateTicketNumber(now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(n, "TIX-20260608-") {
		t.Fatalf("prefix wrong: %q", n)
	}
	if len(n) != len("TIX-20260608-")+6 {
		t.Fatalf("length wrong: %q", n)
	}
}

func TestGenerateTicketNumber_Unique(t *testing.T) {
	now := time.Now()
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		n, err := generateTicketNumber(now)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if seen[n] {
			t.Fatalf("collision: %q", n)
		}
		seen[n] = true
	}
}
