package queue

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestFifoScore_Monotonic(t *testing.T) {
	t0 := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	s1 := FifoScore(t0)
	s2 := FifoScore(t0.Add(time.Nanosecond))
	if s2 <= s1 {
		t.Fatalf("expected monotonic increasing scores, got %d then %d", s1, s2)
	}
}

func TestPresaleScore_Deterministic(t *testing.T) {
	seed := "seed-123"
	u := uuid.New()
	a := PresaleScore(seed, u)
	b := PresaleScore(seed, u)
	if a != b {
		t.Fatalf("presale score not deterministic: %d != %d", a, b)
	}
}

func TestPresaleScore_SeedSensitive(t *testing.T) {
	u := uuid.New()
	if PresaleScore("seed-a", u) == PresaleScore("seed-b", u) {
		t.Fatal("different seeds should (almost always) produce different scores")
	}
}
