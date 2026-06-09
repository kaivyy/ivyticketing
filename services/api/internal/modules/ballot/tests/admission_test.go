package ballot_test

// admission_test.go — tests for CheckBallotAdmission contract.
//
// The flow:
//   BallotWinner calls GET .../ballot/my-entry → receives access_grant_id
//   BallotWinner sends POST .../checkout with X-Queue-Token: <access_grant_id>
//   orders handler passes admissionToken to registration.Gate.Admit()
//   Gate.Admit(ModeBallot) → ballot.Service.CheckBallotAdmission()
//   CheckBallotAdmission → grantChecker.CheckGrant(participantID, categoryID, admissionToken)
//
// This file tests CheckBallotAdmission in isolation using a fake GrantChecker.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/modules/ballot"
)

// ─── fake GrantIssuerChecker ────────────────────────────────────────────────

type fakeGrantChecker struct {
	// checkErr is returned by CheckGrant. nil means valid grant.
	checkErr error
}

func (f *fakeGrantChecker) ReserveSlot(_ context.Context, _ uuid.UUID) error { return nil }
func (f *fakeGrantChecker) CreateGrant(_ context.Context, _, _, _, _ uuid.UUID, _ time.Time) (uuid.UUID, error) {
	return uuid.New(), nil
}
func (f *fakeGrantChecker) CheckGrant(_ context.Context, _, _ uuid.UUID, _ string) error {
	return f.checkErr
}

// ─── minimal repo stub (reuses fakeRepo from service_test.go in same package) ─

// ─── helpers ────────────────────────────────────────────────────────────────

func buildSvcWithGrant(gc *fakeGrantChecker) *ballot.Service {
	repo := &fakeRepo{}
	pools := &fakePoolCreator{poolID: uuid.New()}
	// NewService wires grants as GrantIssuer; we need it to also satisfy GrantChecker.
	// fakeGrantChecker implements both GrantIssuer and GrantChecker.
	return ballot.NewService(repo, nil, pools, gc, &fakeWaitlistCreator{})
}

// ─── tests ──────────────────────────────────────────────────────────────────

// WINNER with a valid, active grant — admission must succeed.
func TestCheckBallotAdmission_WinnerValidGrant(t *testing.T) {
	gc := &fakeGrantChecker{checkErr: nil}
	svc := buildSvcWithGrant(gc)

	grantID := uuid.New().String()
	err := svc.CheckBallotAdmission(context.Background(), uuid.New(), uuid.New(), grantID)
	if err != nil {
		t.Fatalf("winner with valid grant: want nil, got %v", err)
	}
}

// No grant checker wired — must return ErrNotWinner (service guard).
func TestCheckBallotAdmission_NoGrantChecker(t *testing.T) {
	// Build svc with a grants that does NOT implement GrantChecker.
	// Use fakeGrantIssuerOnly which only has ReserveSlot + CreateGrant.
	repo := &fakeRepo{}
	pools := &fakePoolCreator{poolID: uuid.New()}
	svc := ballot.NewService(repo, nil, pools, &fakeGrantIssuerOnly{}, &fakeWaitlistCreator{})

	err := svc.CheckBallotAdmission(context.Background(), uuid.New(), uuid.New(), uuid.New().String())
	if !errors.Is(err, ballot.ErrNotWinner) {
		t.Fatalf("no grant checker: want ErrNotWinner, got %v", err)
	}
}

// Invalid grant token (bad UUID string) — CheckGrant returns ErrGrantNotFound-like error.
func TestCheckBallotAdmission_InvalidToken(t *testing.T) {
	errNotFound := errors.New("grant not found")
	gc := &fakeGrantChecker{checkErr: errNotFound}
	svc := buildSvcWithGrant(gc)

	err := svc.CheckBallotAdmission(context.Background(), uuid.New(), uuid.New(), "not-a-uuid")
	if err == nil {
		t.Fatal("invalid token: want error, got nil")
	}
}

// Expired grant — CheckGrant signals expiry.
func TestCheckBallotAdmission_ExpiredGrant(t *testing.T) {
	errExpired := errors.New("grant expired")
	gc := &fakeGrantChecker{checkErr: errExpired}
	svc := buildSvcWithGrant(gc)

	err := svc.CheckBallotAdmission(context.Background(), uuid.New(), uuid.New(), uuid.New().String())
	if !errors.Is(err, errExpired) {
		t.Fatalf("expired grant: want errExpired, got %v", err)
	}
}

// Consumed grant (already used for a previous checkout attempt).
func TestCheckBallotAdmission_ConsumedGrant(t *testing.T) {
	errConsumed := errors.New("grant already consumed")
	gc := &fakeGrantChecker{checkErr: errConsumed}
	svc := buildSvcWithGrant(gc)

	err := svc.CheckBallotAdmission(context.Background(), uuid.New(), uuid.New(), uuid.New().String())
	if !errors.Is(err, errConsumed) {
		t.Fatalf("consumed grant: want errConsumed, got %v", err)
	}
}

// ─── fakeGrantIssuerOnly — implements GrantIssuer but NOT GrantChecker ──────

type fakeGrantIssuerOnly struct{}

func (f *fakeGrantIssuerOnly) ReserveSlot(_ context.Context, _ uuid.UUID) error { return nil }
func (f *fakeGrantIssuerOnly) CreateGrant(_ context.Context, _, _, _, _ uuid.UUID, _ time.Time) (uuid.UUID, error) {
	return uuid.New(), nil
}
