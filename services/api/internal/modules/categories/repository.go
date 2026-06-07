package categories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error)
	CreateCategory(ctx context.Context, arg db.CreateCategoryParams) (db.EventCategory, error)
	GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error)
	ListCategoriesByEvent(ctx context.Context, eventID uuid.UUID) ([]db.EventCategory, error)
	UpdateCategory(ctx context.Context, arg db.UpdateCategoryParams) (db.EventCategory, error)
	DeleteCategory(ctx context.Context, arg db.DeleteCategoryParams) error
}

type sqlcRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &sqlcRepo{pool: pool, q: db.New(pool)}
}

func (r *sqlcRepo) GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error) {
	return r.q.GetEventByID(ctx, id)
}
func (r *sqlcRepo) CreateCategory(ctx context.Context, arg db.CreateCategoryParams) (db.EventCategory, error) {
	return r.q.CreateCategory(ctx, arg)
}
func (r *sqlcRepo) GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error) {
	return r.q.GetCategoryByID(ctx, id)
}
func (r *sqlcRepo) ListCategoriesByEvent(ctx context.Context, eventID uuid.UUID) ([]db.EventCategory, error) {
	return r.q.ListCategoriesByEvent(ctx, eventID)
}
func (r *sqlcRepo) UpdateCategory(ctx context.Context, arg db.UpdateCategoryParams) (db.EventCategory, error) {
	return r.q.UpdateCategory(ctx, arg)
}
func (r *sqlcRepo) DeleteCategory(ctx context.Context, arg db.DeleteCategoryParams) error {
	return r.q.DeleteCategory(ctx, arg)
}
