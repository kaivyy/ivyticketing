package waitlist

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	CreateWaitlist(ctx context.Context, arg db.CreateWaitlistParams) (db.Waitlist, error)
	GetWaitlist(ctx context.Context, id uuid.UUID) (db.Waitlist, error)
	GetWaitlistByCategory(ctx context.Context, arg db.GetWaitlistByCategoryParams) (db.Waitlist, error)
	SetWaitlistPool(ctx context.Context, arg db.SetWaitlistPoolParams) error
	SetWaitlistSeed(ctx context.Context, arg db.SetWaitlistSeedParams) error
	UpdateWaitlistStatus(ctx context.Context, arg db.UpdateWaitlistStatusParams) error
	JoinWaitlist(ctx context.Context, arg db.JoinWaitlistParams) (db.WaitlistEntry, error)
	GetWaitlistEntry(ctx context.Context, arg db.GetWaitlistEntryParams) (db.WaitlistEntry, error)
	ListWaitingEntries(ctx context.Context, arg db.ListWaitingEntriesParams) ([]db.WaitlistEntry, error)
	UpdateWaitlistEntryStatus(ctx context.Context, arg db.UpdateWaitlistEntryStatusParams) (db.WaitlistEntry, error)
	CountWaitlistPosition(ctx context.Context, arg db.CountWaitlistPositionParams) (int64, error)
}

type sqlcRepo struct{ q *db.Queries }

func NewRepository(pool *pgxpool.Pool) Repository { return &sqlcRepo{q: db.New(pool)} }

func (r *sqlcRepo) CreateWaitlist(ctx context.Context, arg db.CreateWaitlistParams) (db.Waitlist, error) {
	return r.q.CreateWaitlist(ctx, arg)
}
func (r *sqlcRepo) GetWaitlist(ctx context.Context, id uuid.UUID) (db.Waitlist, error) {
	return r.q.GetWaitlist(ctx, id)
}
func (r *sqlcRepo) GetWaitlistByCategory(ctx context.Context, arg db.GetWaitlistByCategoryParams) (db.Waitlist, error) {
	return r.q.GetWaitlistByCategory(ctx, arg)
}
func (r *sqlcRepo) SetWaitlistPool(ctx context.Context, arg db.SetWaitlistPoolParams) error {
	return r.q.SetWaitlistPool(ctx, arg)
}
func (r *sqlcRepo) SetWaitlistSeed(ctx context.Context, arg db.SetWaitlistSeedParams) error {
	return r.q.SetWaitlistSeed(ctx, arg)
}
func (r *sqlcRepo) UpdateWaitlistStatus(ctx context.Context, arg db.UpdateWaitlistStatusParams) error {
	return r.q.UpdateWaitlistStatus(ctx, arg)
}
func (r *sqlcRepo) JoinWaitlist(ctx context.Context, arg db.JoinWaitlistParams) (db.WaitlistEntry, error) {
	return r.q.JoinWaitlist(ctx, arg)
}
func (r *sqlcRepo) GetWaitlistEntry(ctx context.Context, arg db.GetWaitlistEntryParams) (db.WaitlistEntry, error) {
	return r.q.GetWaitlistEntry(ctx, arg)
}
func (r *sqlcRepo) ListWaitingEntries(ctx context.Context, arg db.ListWaitingEntriesParams) ([]db.WaitlistEntry, error) {
	return r.q.ListWaitingEntries(ctx, arg)
}
func (r *sqlcRepo) UpdateWaitlistEntryStatus(ctx context.Context, arg db.UpdateWaitlistEntryStatusParams) (db.WaitlistEntry, error) {
	return r.q.UpdateWaitlistEntryStatus(ctx, arg)
}
func (r *sqlcRepo) CountWaitlistPosition(ctx context.Context, arg db.CountWaitlistPositionParams) (int64, error) {
	return r.q.CountWaitlistPosition(ctx, arg)
}
