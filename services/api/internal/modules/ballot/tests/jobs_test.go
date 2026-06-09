package ballot_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/ballot"
)

type jobRepo struct {
	fakeRepo
	lapsed []db.BallotEntry
}

func (r *jobRepo) ListExpiringWinners(_ context.Context, _ int32) ([]db.BallotEntry, error) {
	drawID := uuid.New()
	return []db.BallotEntry{
		{
			ID:              uuid.New(),
			DrawID:          drawID,
			Status:          ballot.StatusWinner,
			PaymentDeadline: pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true},
		},
	}, nil
}

func (r *jobRepo) UpdateBallotEntryStatus(_ context.Context, arg db.UpdateBallotEntryStatusParams) (db.BallotEntry, error) {
	r.lapsed = append(r.lapsed, db.BallotEntry{ID: arg.ID, Status: arg.Status})
	return db.BallotEntry{}, nil
}

func (r *jobRepo) GetBallotDraw(_ context.Context, _ uuid.UUID) (db.BallotDraw, error) {
	wlID := uuid.New()
	return db.BallotDraw{WaitlistID: &wlID}, nil
}

type fakePromoter struct{ batchCalled bool }

func (f *fakePromoter) PromoteBatch(_ context.Context, _ uuid.UUID) error {
	f.batchCalled = true
	return nil
}

func TestExpireBallotWinners_LapsesAndPromotes(t *testing.T) {
	repo := &jobRepo{}
	promoter := &fakePromoter{}
	job := ballot.NewWinnerExpirer(repo, promoter)
	if err := job.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.lapsed) == 0 {
		t.Fatal("winner should be lapsed")
	}
	if repo.lapsed[0].Status != ballot.StatusLapsed {
		t.Fatalf("want LAPSED got %s", repo.lapsed[0].Status)
	}
	if !promoter.batchCalled {
		t.Fatal("PromoteBatch should be called after lapsing")
	}
}
