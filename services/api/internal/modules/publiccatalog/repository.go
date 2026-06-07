package publiccatalog

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	ListPublishedEventsByOrgSlug(ctx context.Context, slug string) ([]db.Event, error)
	GetPublishedEventByOrgAndSlug(ctx context.Context, arg db.GetPublishedEventByOrgAndSlugParams) (db.Event, error)
	ListCategoriesByEventForPublic(ctx context.Context, eventID uuid.UUID) ([]db.EventCategory, error)
}

type sqlcRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &sqlcRepo{pool: pool, q: db.New(pool)}
}

func (r *sqlcRepo) ListPublishedEventsByOrgSlug(ctx context.Context, slug string) ([]db.Event, error) {
	return r.q.ListPublishedEventsByOrgSlug(ctx, slug)
}
func (r *sqlcRepo) GetPublishedEventByOrgAndSlug(ctx context.Context, arg db.GetPublishedEventByOrgAndSlugParams) (db.Event, error) {
	return r.q.GetPublishedEventByOrgAndSlug(ctx, arg)
}
func (r *sqlcRepo) ListCategoriesByEventForPublic(ctx context.Context, eventID uuid.UUID) ([]db.EventCategory, error) {
	return r.q.ListCategoriesByEventForPublic(ctx, eventID)
}
