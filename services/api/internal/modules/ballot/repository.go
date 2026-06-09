package ballot

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	CreateBallotDraw(ctx context.Context, arg db.CreateBallotDrawParams) (db.BallotDraw, error)
	GetBallotDraw(ctx context.Context, id uuid.UUID) (db.BallotDraw, error)
	GetActiveBallotDrawByCategory(ctx context.Context, arg db.GetActiveBallotDrawByCategoryParams) (db.BallotDraw, error)
	UpdateBallotDrawStatus(ctx context.Context, arg db.UpdateBallotDrawStatusParams) (db.BallotDraw, error)
	SetBallotDrawSeed(ctx context.Context, arg db.SetBallotDrawSeedParams) (db.BallotDraw, error)
	SetBallotDrawPools(ctx context.Context, arg db.SetBallotDrawPoolsParams) error
	CreateBallotEntry(ctx context.Context, arg db.CreateBallotEntryParams) (db.BallotEntry, error)
	GetBallotEntry(ctx context.Context, arg db.GetBallotEntryParams) (db.BallotEntry, error)
	GetBallotEntryByID(ctx context.Context, id uuid.UUID) (db.BallotEntry, error)
	ListAppliedEntriesForDraw(ctx context.Context, drawID uuid.UUID) ([]db.BallotEntry, error)
	UpdateBallotEntryStatus(ctx context.Context, arg db.UpdateBallotEntryStatusParams) (db.BallotEntry, error)
	BulkUpdateBallotOutcome(ctx context.Context, arg db.BulkUpdateBallotOutcomeParams) error
	InsertBallotDrawResult(ctx context.Context, arg db.InsertBallotDrawResultParams) error
	ListBallotDrawResults(ctx context.Context, arg db.ListBallotDrawResultsParams) ([]db.ListBallotDrawResultsRow, error)
	ListAllDrawResults(ctx context.Context, drawID uuid.UUID) ([]db.ListAllDrawResultsRow, error)
	CountBallotDrawResults(ctx context.Context, arg db.CountBallotDrawResultsParams) (int64, error)
	ListWinnerEntries(ctx context.Context, drawID uuid.UUID) ([]db.BallotEntry, error)
	ListExpiringWinners(ctx context.Context, limit int32) ([]db.BallotEntry, error)
	GetBallotEntryByParticipant(ctx context.Context, arg db.GetBallotEntryByParticipantParams) ([]db.BallotEntry, error)
}

type sqlcRepo struct{ q *db.Queries }

func NewRepository(pool *pgxpool.Pool) Repository { return &sqlcRepo{q: db.New(pool)} }

func (r *sqlcRepo) CreateBallotDraw(ctx context.Context, arg db.CreateBallotDrawParams) (db.BallotDraw, error) {
	return r.q.CreateBallotDraw(ctx, arg)
}
func (r *sqlcRepo) GetBallotDraw(ctx context.Context, id uuid.UUID) (db.BallotDraw, error) {
	return r.q.GetBallotDraw(ctx, id)
}
func (r *sqlcRepo) GetActiveBallotDrawByCategory(ctx context.Context, arg db.GetActiveBallotDrawByCategoryParams) (db.BallotDraw, error) {
	return r.q.GetActiveBallotDrawByCategory(ctx, arg)
}
func (r *sqlcRepo) UpdateBallotDrawStatus(ctx context.Context, arg db.UpdateBallotDrawStatusParams) (db.BallotDraw, error) {
	return r.q.UpdateBallotDrawStatus(ctx, arg)
}
func (r *sqlcRepo) SetBallotDrawSeed(ctx context.Context, arg db.SetBallotDrawSeedParams) (db.BallotDraw, error) {
	return r.q.SetBallotDrawSeed(ctx, arg)
}
func (r *sqlcRepo) SetBallotDrawPools(ctx context.Context, arg db.SetBallotDrawPoolsParams) error {
	return r.q.SetBallotDrawPools(ctx, arg)
}
func (r *sqlcRepo) CreateBallotEntry(ctx context.Context, arg db.CreateBallotEntryParams) (db.BallotEntry, error) {
	return r.q.CreateBallotEntry(ctx, arg)
}
func (r *sqlcRepo) GetBallotEntry(ctx context.Context, arg db.GetBallotEntryParams) (db.BallotEntry, error) {
	return r.q.GetBallotEntry(ctx, arg)
}
func (r *sqlcRepo) GetBallotEntryByID(ctx context.Context, id uuid.UUID) (db.BallotEntry, error) {
	return r.q.GetBallotEntryByID(ctx, id)
}
func (r *sqlcRepo) ListAppliedEntriesForDraw(ctx context.Context, drawID uuid.UUID) ([]db.BallotEntry, error) {
	return r.q.ListAppliedEntriesForDraw(ctx, drawID)
}
func (r *sqlcRepo) UpdateBallotEntryStatus(ctx context.Context, arg db.UpdateBallotEntryStatusParams) (db.BallotEntry, error) {
	return r.q.UpdateBallotEntryStatus(ctx, arg)
}
func (r *sqlcRepo) BulkUpdateBallotOutcome(ctx context.Context, arg db.BulkUpdateBallotOutcomeParams) error {
	return r.q.BulkUpdateBallotOutcome(ctx, arg)
}
func (r *sqlcRepo) InsertBallotDrawResult(ctx context.Context, arg db.InsertBallotDrawResultParams) error {
	return r.q.InsertBallotDrawResult(ctx, arg)
}
func (r *sqlcRepo) ListBallotDrawResults(ctx context.Context, arg db.ListBallotDrawResultsParams) ([]db.ListBallotDrawResultsRow, error) {
	return r.q.ListBallotDrawResults(ctx, arg)
}
func (r *sqlcRepo) ListAllDrawResults(ctx context.Context, drawID uuid.UUID) ([]db.ListAllDrawResultsRow, error) {
	return r.q.ListAllDrawResults(ctx, drawID)
}
func (r *sqlcRepo) CountBallotDrawResults(ctx context.Context, arg db.CountBallotDrawResultsParams) (int64, error) {
	return r.q.CountBallotDrawResults(ctx, arg)
}
func (r *sqlcRepo) ListWinnerEntries(ctx context.Context, drawID uuid.UUID) ([]db.BallotEntry, error) {
	return r.q.ListWinnerEntries(ctx, drawID)
}
func (r *sqlcRepo) ListExpiringWinners(ctx context.Context, limit int32) ([]db.BallotEntry, error) {
	return r.q.ListExpiringWinners(ctx, limit)
}
func (r *sqlcRepo) GetBallotEntryByParticipant(ctx context.Context, arg db.GetBallotEntryByParticipantParams) ([]db.BallotEntry, error) {
	return r.q.GetBallotEntryByParticipant(ctx, arg)
}
