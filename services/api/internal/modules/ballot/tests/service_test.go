package ballot_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/ballot"
	"time"
)

type fakeRepo struct {
	draw    db.BallotDraw
	entries []db.BallotEntry
	results []db.BallotDrawResult
	seedSet bool
}

func (r *fakeRepo) CreateBallotDraw(_ context.Context, _ db.CreateBallotDrawParams) (db.BallotDraw, error) {
	return r.draw, nil
}
func (r *fakeRepo) GetBallotDraw(_ context.Context, _ uuid.UUID) (db.BallotDraw, error) {
	return r.draw, nil
}
func (r *fakeRepo) GetActiveBallotDrawByCategory(_ context.Context, _ db.GetActiveBallotDrawByCategoryParams) (db.BallotDraw, error) {
	return r.draw, nil
}
func (r *fakeRepo) UpdateBallotDrawStatus(_ context.Context, arg db.UpdateBallotDrawStatusParams) (db.BallotDraw, error) {
	r.draw.Status = arg.Status
	return r.draw, nil
}
func (r *fakeRepo) SetBallotDrawSeed(_ context.Context, _ db.SetBallotDrawSeedParams) (db.BallotDraw, error) {
	if r.seedSet {
		return db.BallotDraw{}, pgx.ErrNoRows
	}
	r.seedSet = true
	return r.draw, nil
}
func (r *fakeRepo) SetBallotDrawPools(_ context.Context, _ db.SetBallotDrawPoolsParams) error {
	return nil
}
func (r *fakeRepo) CreateBallotEntry(_ context.Context, _ db.CreateBallotEntryParams) (db.BallotEntry, error) {
	return db.BallotEntry{}, nil
}
func (r *fakeRepo) GetBallotEntry(_ context.Context, _ db.GetBallotEntryParams) (db.BallotEntry, error) {
	return db.BallotEntry{}, pgx.ErrNoRows
}
func (r *fakeRepo) GetBallotEntryByID(_ context.Context, _ uuid.UUID) (db.BallotEntry, error) {
	return db.BallotEntry{}, nil
}
func (r *fakeRepo) ListAppliedEntriesForDraw(_ context.Context, _ uuid.UUID) ([]db.BallotEntry, error) {
	return r.entries, nil
}
func (r *fakeRepo) UpdateBallotEntryStatus(_ context.Context, _ db.UpdateBallotEntryStatusParams) (db.BallotEntry, error) {
	return db.BallotEntry{}, nil
}
func (r *fakeRepo) BulkUpdateBallotOutcome(_ context.Context, _ db.BulkUpdateBallotOutcomeParams) error {
	return nil
}
func (r *fakeRepo) InsertBallotDrawResult(_ context.Context, _ db.InsertBallotDrawResultParams) error {
	return nil
}
func (r *fakeRepo) ListBallotDrawResults(_ context.Context, _ db.ListBallotDrawResultsParams) ([]db.ListBallotDrawResultsRow, error) {
	return nil, nil
}
func (r *fakeRepo) ListAllDrawResults(_ context.Context, _ uuid.UUID) ([]db.ListAllDrawResultsRow, error) {
	return nil, nil
}
func (r *fakeRepo) CountBallotDrawResults(_ context.Context, _ db.CountBallotDrawResultsParams) (int64, error) {
	if len(r.results) > 0 {
		return int64(len(r.results)), nil
	}
	return 0, nil
}
func (r *fakeRepo) ListWinnerEntries(_ context.Context, _ uuid.UUID) ([]db.BallotEntry, error) {
	return nil, nil
}
func (r *fakeRepo) ListExpiringWinners(_ context.Context, _ int32) ([]db.BallotEntry, error) {
	return nil, nil
}
func (r *fakeRepo) GetBallotEntryByParticipant(_ context.Context, _ db.GetBallotEntryByParticipantParams) ([]db.BallotEntry, error) {
	return nil, nil
}

type fakePoolCreator struct{ poolID uuid.UUID }

func (f *fakePoolCreator) CreatePool(_ context.Context, _, _, _ uuid.UUID, _, _ string, _ int, _ uuid.UUID) (uuid.UUID, error) {
	return f.poolID, nil
}

type fakeGrantIssuer struct{ callCount int }

func (f *fakeGrantIssuer) ReserveSlot(_ context.Context, _ uuid.UUID) error { return nil }
func (f *fakeGrantIssuer) CreateGrant(_ context.Context, _, _, _, _ uuid.UUID, _ time.Time) (uuid.UUID, error) {
	f.callCount++
	return uuid.New(), nil
}

type fakeWaitlistCreator struct{}

func (f *fakeWaitlistCreator) CreateWaitlist(_ context.Context, _, _, _, _ uuid.UUID) (uuid.UUID, error) {
	return uuid.New(), nil
}
func (f *fakeWaitlistCreator) JoinWithRank(_ context.Context, _, _ uuid.UUID, _ string, _ *uuid.UUID, _ int64) error {
	return nil
}

func buildSvc(repo *fakeRepo) *ballot.Service {
	return ballot.NewService(repo, nil, &fakePoolCreator{poolID: uuid.New()}, &fakeGrantIssuer{}, &fakeWaitlistCreator{})
}

func TestRunDraw_Idempotent(t *testing.T) {
	entries := make([]db.BallotEntry, 5)
	for i := range entries {
		entries[i].ID = uuid.New()
	}
	repo := &fakeRepo{
		draw:    db.BallotDraw{Status: ballot.DrawStatusClosed, Quota: 2, WaitlistSize: pgtype.Int4{Int32: 1, Valid: true}},
		entries: entries,
		results: []db.BallotDrawResult{{ID: uuid.New()}}, // pre-existing results → idempotent
	}
	svc := buildSvc(repo)
	err := svc.RunDraw(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("idempotent RunDraw should return nil, got: %v", err)
	}
	if repo.seedSet {
		t.Fatal("seed should not be set again when results already exist")
	}
}

func TestRunDraw_StatusGuard(t *testing.T) {
	repo := &fakeRepo{draw: db.BallotDraw{Status: ballot.DrawStatusOpen}}
	svc := buildSvc(repo)
	err := svc.RunDraw(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("RunDraw on OPEN draw should return error")
	}
}
