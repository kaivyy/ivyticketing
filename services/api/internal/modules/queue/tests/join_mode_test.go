package queue_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/modules/queue"
)

// fakeResolver implements queue.EventModeResolver.
type fakeResolver struct{ mode string }

func (f *fakeResolver) ResolveEventMode(_ context.Context, _ uuid.UUID) (string, error) {
	return f.mode, nil
}

func TestJoin_WAR_UsesFifo(t *testing.T) {
	// When resolver returns WAR_QUEUE, join should use FIFO pool.
	// This is a build/compile test — verifies the interface is satisfied.
	var _ queue.EventModeResolver = &fakeResolver{mode: "WAR_QUEUE"}
}

func TestPresaleScore_Reproducible(t *testing.T) {
	seed := "test-seed"
	uid := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	s1 := queue.PresaleScore(seed, uid)
	s2 := queue.PresaleScore(seed, uid)
	if s1 != s2 {
		t.Fatalf("presale score not reproducible: %d != %d", s1, s2)
	}
}

func TestPresaleScore_DifferentUsers(t *testing.T) {
	seed := "test-seed"
	u1 := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	u2 := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	if queue.PresaleScore(seed, u1) == queue.PresaleScore(seed, u2) {
		t.Fatal("different users should have different scores (with overwhelming probability)")
	}
}
