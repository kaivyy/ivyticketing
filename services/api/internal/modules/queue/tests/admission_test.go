package queue_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/queue"
)

// admissionFakeRepo extends fakeRepo with admission-expiry tracking.
type admissionFakeRepo struct {
	fakeRepo
	expiredIDs  []uuid.UUID
	requeueArgs []db.RequeueTokenParams
	expiredAdms []db.QueueAdmission
}

func (f *admissionFakeRepo) ListExpiredAdmissions(ctx context.Context, limit int32) ([]db.QueueAdmission, error) {
	n := int(limit)
	if n > len(f.expiredAdms) {
		n = len(f.expiredAdms)
	}
	return f.expiredAdms[:n], nil
}

func (f *admissionFakeRepo) ExpireAdmission(ctx context.Context, id uuid.UUID) error {
	f.expiredIDs = append(f.expiredIDs, id)
	return nil
}

func (f *admissionFakeRepo) Requeue(ctx context.Context, arg db.RequeueTokenParams) error {
	f.requeueArgs = append(f.requeueArgs, arg)
	return nil
}

func (f *admissionFakeRepo) ExecTx(ctx context.Context, fn func(queue.Repository) error) error {
	return fn(f)
}

func makeExpiredAdmission() db.QueueAdmission {
	return db.QueueAdmission{
		ID:            uuid.New(),
		TokenID:       uuid.New(),
		EventID:       uuid.New(),
		ParticipantID: uuid.New(),
		Status:        "ACTIVE",
		CheckoutExpiresAt: pgtype.Timestamptz{
			Valid: true,
		},
	}
}

func TestExpireDue_RequeuesToken(t *testing.T) {
	adm := makeExpiredAdmission()
	repo := &admissionFakeRepo{
		expiredAdms: []db.QueueAdmission{adm},
	}
	svc := queue.NewService(repo, nil, nil, nil, 10, nil)

	count, err := svc.ExpireDue(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count=1, got %d", count)
	}

	// ExpireAdmission must have been called with the admission ID.
	if len(repo.expiredIDs) != 1 {
		t.Fatalf("expected 1 ExpireAdmission call, got %d", len(repo.expiredIDs))
	}
	if repo.expiredIDs[0] != adm.ID {
		t.Errorf("ExpireAdmission called with wrong id: got %v, want %v", repo.expiredIDs[0], adm.ID)
	}

	// Requeue must have been called with the token ID.
	if len(repo.requeueArgs) != 1 {
		t.Fatalf("expected 1 Requeue call, got %d", len(repo.requeueArgs))
	}
	if repo.requeueArgs[0].ID != adm.TokenID {
		t.Errorf("Requeue called with wrong token id: got %v, want %v", repo.requeueArgs[0].ID, adm.TokenID)
	}
	if repo.requeueArgs[0].Score == 0 {
		t.Error("Requeue score should be non-zero")
	}
}

func TestExpireDue_EmptyList(t *testing.T) {
	repo := &admissionFakeRepo{}
	svc := queue.NewService(repo, nil, nil, nil, 10, nil)

	count, err := svc.ExpireDue(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected count=0, got %d", count)
	}
}

func TestExpireDue_MultipleAdmissions(t *testing.T) {
	adms := []db.QueueAdmission{makeExpiredAdmission(), makeExpiredAdmission(), makeExpiredAdmission()}
	repo := &admissionFakeRepo{expiredAdms: adms}
	svc := queue.NewService(repo, nil, nil, nil, 10, nil)

	count, err := svc.ExpireDue(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected count=3, got %d", count)
	}
	if len(repo.expiredIDs) != 3 {
		t.Fatalf("expected 3 ExpireAdmission calls, got %d", len(repo.expiredIDs))
	}
	if len(repo.requeueArgs) != 3 {
		t.Fatalf("expected 3 Requeue calls, got %d", len(repo.requeueArgs))
	}
}

func TestExpireDue_LimitRespected(t *testing.T) {
	adms := []db.QueueAdmission{makeExpiredAdmission(), makeExpiredAdmission(), makeExpiredAdmission()}
	repo := &admissionFakeRepo{expiredAdms: adms}
	svc := queue.NewService(repo, nil, nil, nil, 10, nil)

	count, err := svc.ExpireDue(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count=2 (limit), got %d", count)
	}
}
