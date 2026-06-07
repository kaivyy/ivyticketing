package events

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error)
	GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error)
	GetEventByOrgAndSlug(ctx context.Context, arg db.GetEventByOrgAndSlugParams) (db.Event, error)
	ListEventsByOrg(ctx context.Context, orgID uuid.UUID) ([]db.Event, error)
	UpdateEvent(ctx context.Context, arg db.UpdateEventParams) (db.Event, error)
	UpdateEventStatus(ctx context.Context, arg db.UpdateEventStatusParams) (db.Event, error)
	SetEventMediaKey(ctx context.Context, arg db.SetEventMediaKeyParams) (db.Event, error)
	DeleteEvent(ctx context.Context, arg db.DeleteEventParams) error
	CountCategoriesForEvent(ctx context.Context, eventID uuid.UUID) (int64, error)
}

type sqlcRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &sqlcRepo{pool: pool, q: db.New(pool)}
}

func (r *sqlcRepo) CreateEvent(ctx context.Context, arg db.CreateEventParams) (db.Event, error) {
	return r.q.CreateEvent(ctx, arg)
}
func (r *sqlcRepo) GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error) {
	return r.q.GetEventByID(ctx, id)
}
func (r *sqlcRepo) GetEventByOrgAndSlug(ctx context.Context, arg db.GetEventByOrgAndSlugParams) (db.Event, error) {
	return r.q.GetEventByOrgAndSlug(ctx, arg)
}
func (r *sqlcRepo) ListEventsByOrg(ctx context.Context, orgID uuid.UUID) ([]db.Event, error) {
	return r.q.ListEventsByOrg(ctx, orgID)
}
func (r *sqlcRepo) UpdateEvent(ctx context.Context, arg db.UpdateEventParams) (db.Event, error) {
	return r.q.UpdateEvent(ctx, arg)
}
func (r *sqlcRepo) UpdateEventStatus(ctx context.Context, arg db.UpdateEventStatusParams) (db.Event, error) {
	return r.q.UpdateEventStatus(ctx, arg)
}
func (r *sqlcRepo) SetEventMediaKey(ctx context.Context, arg db.SetEventMediaKeyParams) (db.Event, error) {
	return r.q.SetEventMediaKey(ctx, arg)
}
func (r *sqlcRepo) DeleteEvent(ctx context.Context, arg db.DeleteEventParams) error {
	return r.q.DeleteEvent(ctx, arg)
}
func (r *sqlcRepo) CountCategoriesForEvent(ctx context.Context, eventID uuid.UUID) (int64, error) {
	return r.q.CountCategoriesForEvent(ctx, eventID)
}
