package access

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	CreateAccessPool(ctx context.Context, arg db.CreateAccessPoolParams) (db.AccessPool, error)
	GetAccessPool(ctx context.Context, id uuid.UUID) (db.AccessPool, error)
	ReservePoolSlot(ctx context.Context, id uuid.UUID) (db.AccessPool, error)
	ConsumePoolSlot(ctx context.Context, id uuid.UUID) error
	ReleasePoolSlot(ctx context.Context, id uuid.UUID) error
	CreateAccessGrant(ctx context.Context, arg db.CreateAccessGrantParams) (db.AccessGrant, error)
	GetAccessGrant(ctx context.Context, id uuid.UUID) (db.AccessGrant, error)
	GetActiveGrantForParticipant(ctx context.Context, arg db.GetActiveGrantForParticipantParams) (db.AccessGrant, error)
	ExpireGrant(ctx context.Context, id uuid.UUID) error
	ConsumeGrant(ctx context.Context, arg db.ConsumeGrantParams) error
	ListExpiredActiveGrants(ctx context.Context, limit int32) ([]db.AccessGrant, error)
}

type sqlcRepo struct{ q *db.Queries }

func NewRepository(pool *pgxpool.Pool) Repository { return &sqlcRepo{q: db.New(pool)} }

func (r *sqlcRepo) CreateAccessPool(ctx context.Context, arg db.CreateAccessPoolParams) (db.AccessPool, error) {
	return r.q.CreateAccessPool(ctx, arg)
}
func (r *sqlcRepo) GetAccessPool(ctx context.Context, id uuid.UUID) (db.AccessPool, error) {
	return r.q.GetAccessPool(ctx, id)
}
func (r *sqlcRepo) ReservePoolSlot(ctx context.Context, id uuid.UUID) (db.AccessPool, error) {
	return r.q.ReservePoolSlot(ctx, id)
}
func (r *sqlcRepo) ConsumePoolSlot(ctx context.Context, id uuid.UUID) error {
	return r.q.ConsumePoolSlot(ctx, id)
}
func (r *sqlcRepo) ReleasePoolSlot(ctx context.Context, id uuid.UUID) error {
	return r.q.ReleasePoolSlot(ctx, id)
}
func (r *sqlcRepo) CreateAccessGrant(ctx context.Context, arg db.CreateAccessGrantParams) (db.AccessGrant, error) {
	return r.q.CreateAccessGrant(ctx, arg)
}
func (r *sqlcRepo) GetAccessGrant(ctx context.Context, id uuid.UUID) (db.AccessGrant, error) {
	return r.q.GetAccessGrant(ctx, id)
}
func (r *sqlcRepo) GetActiveGrantForParticipant(ctx context.Context, arg db.GetActiveGrantForParticipantParams) (db.AccessGrant, error) {
	return r.q.GetActiveGrantForParticipant(ctx, arg)
}
func (r *sqlcRepo) ExpireGrant(ctx context.Context, id uuid.UUID) error {
	return r.q.ExpireGrant(ctx, id)
}
func (r *sqlcRepo) ConsumeGrant(ctx context.Context, arg db.ConsumeGrantParams) error {
	return r.q.ConsumeGrant(ctx, arg)
}
func (r *sqlcRepo) ListExpiredActiveGrants(ctx context.Context, limit int32) ([]db.AccessGrant, error) {
	return r.q.ListExpiredActiveGrants(ctx, limit)
}
