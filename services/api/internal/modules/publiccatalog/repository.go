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
	ListAllPublishedEvents(ctx context.Context) ([]db.ListAllPublishedEventsRow, error)
	GetPublishedEventByIDOrSlug(ctx context.Context, identifier string) (db.GetPublishedEventByIDOrSlugRow, error)
	ListCategoriesByEventForPublic(ctx context.Context, eventID uuid.UUID) ([]db.EventCategory, error)
	ListCategoriesByEventForPublicWithMode(ctx context.Context, eventID uuid.UUID) ([]db.EventCategoryWithMode, error)
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
func (r *sqlcRepo) ListAllPublishedEvents(ctx context.Context) ([]db.ListAllPublishedEventsRow, error) {
	return r.q.ListAllPublishedEvents(ctx)
}
func (r *sqlcRepo) GetPublishedEventByIDOrSlug(ctx context.Context, identifier string) (db.GetPublishedEventByIDOrSlugRow, error) {
	return r.q.GetPublishedEventByIDOrSlug(ctx, identifier)
}
func (r *sqlcRepo) ListCategoriesByEventForPublic(ctx context.Context, eventID uuid.UUID) ([]db.EventCategory, error) {
	return r.q.ListCategoriesByEventForPublic(ctx, eventID)
}
func (r *sqlcRepo) ListCategoriesByEventForPublicWithMode(ctx context.Context, eventID uuid.UUID) ([]db.EventCategoryWithMode, error) {
	return r.q.ListCategoriesByEventForPublicWithMode(ctx, eventID)
}
