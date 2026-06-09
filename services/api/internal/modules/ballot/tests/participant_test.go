package ballot_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/ballot"
)

// participantRepo extends fakeRepo with per-test control over entry queries.
type participantRepo struct {
	fakeRepo
	entryByDraw          *db.BallotEntry
	entriesByParticipant []db.BallotEntry
	createEntryResult    db.BallotEntry
	createEntryErr       error
}

func (r *participantRepo) GetBallotEntry(_ context.Context, _ db.GetBallotEntryParams) (db.BallotEntry, error) {
	if r.entryByDraw != nil {
		return *r.entryByDraw, nil
	}
	return db.BallotEntry{}, pgx.ErrNoRows
}

func (r *participantRepo) GetBallotEntryByParticipant(_ context.Context, _ db.GetBallotEntryByParticipantParams) ([]db.BallotEntry, error) {
	return r.entriesByParticipant, nil
}

func (r *participantRepo) CreateBallotEntry(_ context.Context, _ db.CreateBallotEntryParams) (db.BallotEntry, error) {
	return r.createEntryResult, r.createEntryErr
}

func (r *participantRepo) UpdateBallotEntryStatus(_ context.Context, _ db.UpdateBallotEntryStatusParams) (db.BallotEntry, error) {
	return db.BallotEntry{}, nil
}

func buildParticipantSvc(repo ballot.Repository) *ballot.Service {
	return ballot.NewService(repo, nil, &fakePoolCreator{poolID: uuid.New()}, &fakeGrantIssuer{}, &fakeWaitlistCreator{})
}

func openDraw(eventID, categoryID, orgID uuid.UUID) db.BallotDraw {
	return db.BallotDraw{
		ID:             uuid.New(),
		Status:         ballot.DrawStatusOpen,
		EventID:        eventID,
		CategoryID:     categoryID,
		OrganizationID: orgID,
		Quota:          10,
		WaitlistSize:   pgtype.Int4{Int32: 5, Valid: true},
	}
}

// TestApply_Success — happy path: open draw, no prior entry.
func TestApply_Success(t *testing.T) {
	eventID := uuid.New()
	categoryID := uuid.New()
	orgID := uuid.New()
	drawID := uuid.New()
	participantID := uuid.New()

	newEntry := db.BallotEntry{
		ID:            uuid.New(),
		DrawID:        drawID,
		ParticipantID: participantID,
		Status:        ballot.StatusApplied,
	}

	draw := openDraw(eventID, categoryID, orgID)
	draw.ID = drawID

	repo := &participantRepo{
		fakeRepo:          fakeRepo{draw: draw},
		createEntryResult: newEntry,
	}

	svc := buildParticipantSvc(repo)
	entry, err := svc.Apply(context.Background(), participantID, eventID, categoryID, drawID)
	if err != nil {
		t.Fatalf("Apply should succeed, got: %v", err)
	}
	if entry.ID != newEntry.ID {
		t.Fatalf("expected entry ID %s, got %s", newEntry.ID, entry.ID)
	}
}

// TestApply_DuplicateBlocked — second apply returns ErrAlreadyApplied.
func TestApply_DuplicateBlocked(t *testing.T) {
	eventID := uuid.New()
	categoryID := uuid.New()
	orgID := uuid.New()
	drawID := uuid.New()
	participantID := uuid.New()

	existingEntry := db.BallotEntry{
		ID:            uuid.New(),
		DrawID:        drawID,
		ParticipantID: participantID,
		Status:        ballot.StatusApplied,
	}

	draw := openDraw(eventID, categoryID, orgID)
	draw.ID = drawID

	repo := &participantRepo{
		fakeRepo:    fakeRepo{draw: draw},
		entryByDraw: &existingEntry,
	}

	svc := buildParticipantSvc(repo)
	_, err := svc.Apply(context.Background(), participantID, eventID, categoryID, drawID)
	if !errors.Is(err, ballot.ErrAlreadyApplied) {
		t.Fatalf("expected ErrAlreadyApplied, got: %v", err)
	}
}

// TestApply_DrawNotOpen_Blocked — draw not OPEN returns ErrBallotClosed.
func TestApply_DrawNotOpen_Blocked(t *testing.T) {
	eventID := uuid.New()
	categoryID := uuid.New()
	orgID := uuid.New()
	drawID := uuid.New()

	closedDraw := openDraw(eventID, categoryID, orgID)
	closedDraw.ID = drawID
	closedDraw.Status = ballot.DrawStatusClosed

	repo := &participantRepo{
		fakeRepo: fakeRepo{draw: closedDraw},
	}

	svc := buildParticipantSvc(repo)
	_, err := svc.Apply(context.Background(), uuid.New(), eventID, categoryID, drawID)
	if !errors.Is(err, ballot.ErrBallotClosed) {
		t.Fatalf("expected ErrBallotClosed, got: %v", err)
	}
}

// TestWithdraw_NotApplied_Blocked — WINNER entry → ErrBallotWithdrawNotAllowed.
func TestWithdraw_NotApplied_Blocked(t *testing.T) {
	eventID := uuid.New()
	categoryID := uuid.New()
	orgID := uuid.New()
	drawID := uuid.New()
	participantID := uuid.New()

	winnerEntry := db.BallotEntry{
		ID:            uuid.New(),
		DrawID:        drawID,
		CategoryID:    categoryID,
		ParticipantID: participantID,
		Status:        ballot.StatusWinner,
	}

	draw := openDraw(eventID, categoryID, orgID)
	draw.ID = drawID

	repo := &participantRepo{
		fakeRepo:             fakeRepo{draw: draw},
		entriesByParticipant: []db.BallotEntry{winnerEntry},
	}

	svc := buildParticipantSvc(repo)
	err := svc.Withdraw(context.Background(), participantID, categoryID)
	if !errors.Is(err, ballot.ErrBallotWithdrawNotAllowed) {
		t.Fatalf("expected ErrBallotWithdrawNotAllowed for WINNER entry, got: %v", err)
	}
}

// TestWithdraw_DrawClosed_Blocked — APPLIED entry but draw CLOSED → ErrBallotWithdrawNotAllowed.
func TestWithdraw_DrawClosed_Blocked(t *testing.T) {
	eventID := uuid.New()
	categoryID := uuid.New()
	orgID := uuid.New()
	drawID := uuid.New()
	participantID := uuid.New()

	appliedEntry := db.BallotEntry{
		ID:            uuid.New(),
		DrawID:        drawID,
		CategoryID:    categoryID,
		ParticipantID: participantID,
		Status:        ballot.StatusApplied,
	}

	closedDraw := openDraw(eventID, categoryID, orgID)
	closedDraw.ID = drawID
	closedDraw.Status = ballot.DrawStatusClosed

	repo := &participantRepo{
		fakeRepo:             fakeRepo{draw: closedDraw},
		entriesByParticipant: []db.BallotEntry{appliedEntry},
	}

	svc := buildParticipantSvc(repo)
	err := svc.Withdraw(context.Background(), participantID, categoryID)
	if !errors.Is(err, ballot.ErrBallotWithdrawNotAllowed) {
		t.Fatalf("expected ErrBallotWithdrawNotAllowed for closed draw, got: %v", err)
	}
}

// TestWithdraw_AppliedOpenDraw_Success — APPLIED entry + OPEN draw → success.
func TestWithdraw_AppliedOpenDraw_Success(t *testing.T) {
	eventID := uuid.New()
	categoryID := uuid.New()
	orgID := uuid.New()
	drawID := uuid.New()
	participantID := uuid.New()

	appliedEntry := db.BallotEntry{
		ID:            uuid.New(),
		DrawID:        drawID,
		CategoryID:    categoryID,
		ParticipantID: participantID,
		Status:        ballot.StatusApplied,
	}

	draw := openDraw(eventID, categoryID, orgID)
	draw.ID = drawID

	repo := &participantRepo{
		fakeRepo:             fakeRepo{draw: draw},
		entriesByParticipant: []db.BallotEntry{appliedEntry},
	}

	svc := buildParticipantSvc(repo)
	err := svc.Withdraw(context.Background(), participantID, categoryID)
	if err != nil {
		t.Fatalf("Withdraw should succeed for APPLIED+OPEN, got: %v", err)
	}
}

// --- CheckBallotAdmission tests ---

// fakeGrantIssuerChecker implements ballot.GrantIssuerChecker for injection.
type fakeGrantIssuerChecker struct {
	checkErr error
}

func (f *fakeGrantIssuerChecker) ReserveSlot(_ context.Context, _ uuid.UUID) error { return nil }
func (f *fakeGrantIssuerChecker) CreateGrant(_ context.Context, _, _, _, _ uuid.UUID, _ time.Time) (uuid.UUID, error) {
	return uuid.New(), nil
}
func (f *fakeGrantIssuerChecker) CheckGrant(_ context.Context, _, _ uuid.UUID, _ string) error {
	return f.checkErr
}

// TestCheckBallotAdmission_WinnerWithGrant_Passes — valid grant returns nil.
func TestCheckBallotAdmission_WinnerWithGrant_Passes(t *testing.T) {
	repo := &participantRepo{fakeRepo: fakeRepo{draw: db.BallotDraw{}}}
	gc := &fakeGrantIssuerChecker{checkErr: nil}
	svc := ballot.NewService(repo, nil, &fakePoolCreator{poolID: uuid.New()}, gc, &fakeWaitlistCreator{})

	err := svc.CheckBallotAdmission(context.Background(), uuid.New(), uuid.New(), "valid-token")
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

// TestCheckBallotAdmission_NoEntry_Fails — grant checker returns error → propagated.
func TestCheckBallotAdmission_NoEntry_Fails(t *testing.T) {
	repo := &participantRepo{fakeRepo: fakeRepo{draw: db.BallotDraw{}}}
	sentinel := errors.New("grant not found")
	gc := &fakeGrantIssuerChecker{checkErr: sentinel}
	svc := ballot.NewService(repo, nil, &fakePoolCreator{poolID: uuid.New()}, gc, &fakeWaitlistCreator{})

	err := svc.CheckBallotAdmission(context.Background(), uuid.New(), uuid.New(), "bad-token")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got: %v", err)
	}
}

