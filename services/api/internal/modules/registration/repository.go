package registration

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	GetEventSettings(ctx context.Context, eventID uuid.UUID) (db.EventRegistrationSetting, error)
	UpsertEventSettings(ctx context.Context, arg db.UpsertEventRegistrationSettingsParams) (db.EventRegistrationSetting, error)
	GetCategorySettings(ctx context.Context, categoryID uuid.UUID) (db.CategoryRegistrationSetting, error)
	UpsertCategorySettings(ctx context.Context, arg db.UpsertCategoryRegistrationSettingsParams) (db.CategoryRegistrationSetting, error)
	ListCategorySettingsByEvent(ctx context.Context, eventID uuid.UUID) ([]db.CategoryRegistrationSetting, error)
	GetCategoryByID(ctx context.Context, categoryID uuid.UUID) (db.EventCategory, error)
}

type sqlcRepo struct {
	q *db.Queries
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &sqlcRepo{q: db.New(pool)}
}

func (r *sqlcRepo) GetEventSettings(ctx context.Context, eventID uuid.UUID) (db.EventRegistrationSetting, error) {
	return r.q.GetEventRegistrationSettings(ctx, eventID)
}

func (r *sqlcRepo) UpsertEventSettings(ctx context.Context, arg db.UpsertEventRegistrationSettingsParams) (db.EventRegistrationSetting, error) {
	return r.q.UpsertEventRegistrationSettings(ctx, arg)
}

func (r *sqlcRepo) GetCategorySettings(ctx context.Context, categoryID uuid.UUID) (db.CategoryRegistrationSetting, error) {
	return r.q.GetCategoryRegistrationSettings(ctx, categoryID)
}

func (r *sqlcRepo) UpsertCategorySettings(ctx context.Context, arg db.UpsertCategoryRegistrationSettingsParams) (db.CategoryRegistrationSetting, error) {
	return r.q.UpsertCategoryRegistrationSettings(ctx, arg)
}

func (r *sqlcRepo) ListCategorySettingsByEvent(ctx context.Context, eventID uuid.UUID) ([]db.CategoryRegistrationSetting, error) {
	return r.q.ListCategoryRegistrationSettingsByEvent(ctx, eventID)
}

func (r *sqlcRepo) GetCategoryByID(ctx context.Context, categoryID uuid.UUID) (db.EventCategory, error) {
	return r.q.GetCategoryByID(ctx, categoryID)
}
