package forms

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error
	GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error)
	GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error)
	GetFormSchemaByEvent(ctx context.Context, eventID uuid.UUID) (db.FormSchema, error)
	CreateFormSchema(ctx context.Context, arg db.CreateFormSchemaParams) (db.FormSchema, error)
	UpdateFormSchemaName(ctx context.Context, arg db.UpdateFormSchemaNameParams) (db.FormSchema, error)
	ListFieldsBySchema(ctx context.Context, schemaID uuid.UUID) ([]db.FormField, error)
	GetFieldByID(ctx context.Context, id uuid.UUID) (db.FormField, error)
	CreateField(ctx context.Context, arg db.CreateFieldParams) (db.FormField, error)
	UpdateField(ctx context.Context, arg db.UpdateFieldParams) (db.FormField, error)
	UpdateFieldOrder(ctx context.Context, arg db.UpdateFieldOrderParams) error
	DeleteField(ctx context.Context, arg db.DeleteFieldParams) error
	MaxFieldOrder(ctx context.Context, schemaID uuid.UUID) (int32, error)
}

type sqlcRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &sqlcRepo{pool: pool, q: db.New(pool)}
}

func (r *sqlcRepo) ExecTx(ctx context.Context, fn func(Repository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(&sqlcRepo{pool: r.pool, q: db.New(tx)}); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (r *sqlcRepo) GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error) {
	return r.q.GetEventByID(ctx, id)
}
func (r *sqlcRepo) GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error) {
	return r.q.GetCategoryByID(ctx, id)
}
func (r *sqlcRepo) GetFormSchemaByEvent(ctx context.Context, eventID uuid.UUID) (db.FormSchema, error) {
	return r.q.GetFormSchemaByEvent(ctx, eventID)
}
func (r *sqlcRepo) CreateFormSchema(ctx context.Context, arg db.CreateFormSchemaParams) (db.FormSchema, error) {
	return r.q.CreateFormSchema(ctx, arg)
}
func (r *sqlcRepo) UpdateFormSchemaName(ctx context.Context, arg db.UpdateFormSchemaNameParams) (db.FormSchema, error) {
	return r.q.UpdateFormSchemaName(ctx, arg)
}
func (r *sqlcRepo) ListFieldsBySchema(ctx context.Context, schemaID uuid.UUID) ([]db.FormField, error) {
	return r.q.ListFieldsBySchema(ctx, schemaID)
}
func (r *sqlcRepo) GetFieldByID(ctx context.Context, id uuid.UUID) (db.FormField, error) {
	return r.q.GetFieldByID(ctx, id)
}
func (r *sqlcRepo) CreateField(ctx context.Context, arg db.CreateFieldParams) (db.FormField, error) {
	return r.q.CreateField(ctx, arg)
}
func (r *sqlcRepo) UpdateField(ctx context.Context, arg db.UpdateFieldParams) (db.FormField, error) {
	return r.q.UpdateField(ctx, arg)
}
func (r *sqlcRepo) UpdateFieldOrder(ctx context.Context, arg db.UpdateFieldOrderParams) error {
	return r.q.UpdateFieldOrder(ctx, arg)
}
func (r *sqlcRepo) DeleteField(ctx context.Context, arg db.DeleteFieldParams) error {
	return r.q.DeleteField(ctx, arg)
}
func (r *sqlcRepo) MaxFieldOrder(ctx context.Context, schemaID uuid.UUID) (int32, error) {
	return r.q.MaxFieldOrder(ctx, schemaID)
}
