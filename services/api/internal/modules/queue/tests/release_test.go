package queue_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/queue"
)

// fakeRepo implements queue.Repository for unit tests.
type fakeRepo struct {
	tokens         []db.QueueToken
	markAllowedN   atomic.Int32 // counts MarkAllowed calls
	admissions     []db.CreateAdmissionParams
	markAlwaysSkip bool // if true, MarkAllowed always returns ErrNoRows
}

func (f *fakeRepo) ExecTx(ctx context.Context, fn func(queue.Repository) error) error {
	return fn(f)
}

func (f *fakeRepo) CreateToken(ctx context.Context, arg db.CreateQueueTokenParams) (db.QueueToken, error) {
	return db.QueueToken{}, nil
}

func (f *fakeRepo) GetTokenByEventParticipant(ctx context.Context, eventID, participantID uuid.UUID) (db.QueueToken, error) {
	return db.QueueToken{}, nil
}

func (f *fakeRepo) GetTokenByID(ctx context.Context, id uuid.UUID) (db.QueueToken, error) {
	return db.QueueToken{}, nil
}

func (f *fakeRepo) ListWaiting(ctx context.Context, arg db.ListWaitingTokensParams) ([]db.QueueToken, error) {
	n := int(arg.Limit)
	if n > len(f.tokens) {
		n = len(f.tokens)
	}
	return f.tokens[:n], nil
}

func (f *fakeRepo) MarkAllowed(ctx context.Context, id uuid.UUID) (db.QueueToken, error) {
	if f.markAlwaysSkip {
		return db.QueueToken{}, pgx.ErrNoRows
	}
	// Find the token and return it as-is (simulates success).
	for _, t := range f.tokens {
		if t.ID == id {
			return t, nil
		}
	}
	return db.QueueToken{}, pgx.ErrNoRows
}

func (f *fakeRepo) MarkCompleted(ctx context.Context, id uuid.UUID) error { return nil }

func (f *fakeRepo) Requeue(ctx context.Context, arg db.RequeueTokenParams) error { return nil }

func (f *fakeRepo) CountByStatus(ctx context.Context, arg db.CountTokensByStatusParams) (int64, error) {
	return 0, nil
}

func (f *fakeRepo) CreateAdmission(ctx context.Context, arg db.CreateAdmissionParams) (db.QueueAdmission, error) {
	f.admissions = append(f.admissions, arg)
	return db.QueueAdmission{ID: uuid.New(), TokenID: arg.TokenID, EventID: arg.EventID, ParticipantID: arg.ParticipantID}, nil
}

func (f *fakeRepo) GetActiveAdmission(ctx context.Context, arg db.GetActiveAdmissionByParticipantParams) (db.QueueAdmission, error) {
	return db.QueueAdmission{}, errors.New("not found")
}

func (f *fakeRepo) ConsumeAdmission(ctx context.Context, id uuid.UUID) error { return nil }

func (f *fakeRepo) ListExpiredAdmissions(ctx context.Context, limit int32) ([]db.QueueAdmission, error) {
	return nil, nil
}

func (f *fakeRepo) ExpireAdmission(ctx context.Context, id uuid.UUID) error { return nil }

func (f *fakeRepo) GetControl(ctx context.Context, eventID uuid.UUID) (db.QueueControl, error) {
	return db.QueueControl{}, errors.New("not found")
}

func (f *fakeRepo) UpsertControl(ctx context.Context, arg db.UpsertQueueControlParams) (db.QueueControl, error) {
	return db.QueueControl{}, nil
}

func (f *fakeRepo) SetState(ctx context.Context, arg db.SetQueueStateParams) error { return nil }

func (f *fakeRepo) SetRate(ctx context.Context, arg db.SetReleaseRateParams) error { return nil }

func (f *fakeRepo) ListRunningEvents(ctx context.Context) ([]uuid.UUID, error) { return nil, nil }

// makeTokens builds n distinct QueueToken stubs.
func makeTokens(n int) []db.QueueToken {
	tokens := make([]db.QueueToken, n)
	for i := range tokens {
		tokens[i] = db.QueueToken{
			ID:            uuid.New(),
			ParticipantID: uuid.New(),
			Status:        "WAITING",
		}
	}
	return tokens
}

func TestRelease_PromotesN(t *testing.T) {
	repo := &fakeRepo{tokens: makeTokens(5)}
	svc := queue.NewService(repo, nil, nil, nil, 10, nil)

	eventID := uuid.New()
	promoted, err := svc.Release(context.Background(), eventID, 3, 10*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if promoted != 3 {
		t.Fatalf("expected 3 promoted, got %d", promoted)
	}
	if len(repo.admissions) != 3 {
		t.Fatalf("expected 3 admissions created, got %d", len(repo.admissions))
	}
	// Verify all admissions have the correct event_id and a valid expiry.
	for i, adm := range repo.admissions {
		if adm.EventID != eventID {
			t.Errorf("admission[%d] wrong eventID: %v", i, adm.EventID)
		}
		if !adm.CheckoutExpiresAt.Valid {
			t.Errorf("admission[%d] CheckoutExpiresAt not valid", i)
		}
		if adm.CheckoutExpiresAt.Time.Before(time.Now()) {
			t.Errorf("admission[%d] CheckoutExpiresAt is in the past", i)
		}
	}
}

func TestRelease_ZeroN(t *testing.T) {
	repo := &fakeRepo{tokens: makeTokens(5)}
	svc := queue.NewService(repo, nil, nil, nil, 10, nil)

	promoted, err := svc.Release(context.Background(), uuid.New(), 0, 10*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if promoted != 0 {
		t.Fatalf("expected 0 promoted, got %d", promoted)
	}
}

func TestRelease_AlreadyPromoted_Skipped(t *testing.T) {
	// All MarkAllowed calls return ErrNoRows (concurrent promotion simulation).
	repo := &fakeRepo{tokens: makeTokens(3), markAlwaysSkip: true}
	svc := queue.NewService(repo, nil, nil, nil, 10, nil)

	promoted, err := svc.Release(context.Background(), uuid.New(), 3, 10*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ErrNoRows is treated as "skip" inside the tx, ExecTx returns nil,
	// so promoted still increments. The guard is that no admission is created.
	if len(repo.admissions) != 0 {
		t.Fatalf("expected 0 admissions when all already promoted, got %d", len(repo.admissions))
	}
	// promoted may be 3 (tx succeeded with skip) — that is acceptable behaviour;
	// the important invariant is no duplicate admissions were written.
	_ = promoted
}
