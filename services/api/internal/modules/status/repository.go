package status

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Repository is the data access surface for the public status page and
// incident timeline.
type Repository interface {
	ListComponents(ctx context.Context) ([]db.StatusComponent, error)
	UpdateComponent(ctx context.Context, arg db.UpdateStatusComponentParams) (db.StatusComponent, error)

	ListActiveIncidents(ctx context.Context) ([]db.Incident, error)
	ListRecentIncidents(ctx context.Context, arg db.ListRecentIncidentsParams) ([]db.Incident, error)
	GetIncident(ctx context.Context, id uuid.UUID) (db.Incident, error)
	CreateIncident(ctx context.Context, arg db.CreateIncidentParams) (db.Incident, error)
	UpdateIncidentStatus(ctx context.Context, arg db.UpdateIncidentStatusParams) (db.Incident, error)

	ListIncidentUpdates(ctx context.Context, incidentID uuid.UUID) ([]db.IncidentUpdate, error)
	ListUpdatesForIncidents(ctx context.Context, ids []uuid.UUID) ([]db.IncidentUpdate, error)
	CreateIncidentUpdate(ctx context.Context, arg db.CreateIncidentUpdateParams) (db.IncidentUpdate, error)
}

type sqlcRepo struct{ q *db.Queries }

// NewRepository constructs a Repository backed by the sqlc-generated Queries.
func NewRepository(pool *pgxpool.Pool) Repository { return &sqlcRepo{q: db.New(pool)} }

func (r *sqlcRepo) ListComponents(ctx context.Context) ([]db.StatusComponent, error) {
	return r.q.ListStatusComponents(ctx)
}
func (r *sqlcRepo) UpdateComponent(ctx context.Context, arg db.UpdateStatusComponentParams) (db.StatusComponent, error) {
	return r.q.UpdateStatusComponent(ctx, arg)
}
func (r *sqlcRepo) ListActiveIncidents(ctx context.Context) ([]db.Incident, error) {
	return r.q.ListActiveIncidents(ctx)
}
func (r *sqlcRepo) ListRecentIncidents(ctx context.Context, arg db.ListRecentIncidentsParams) ([]db.Incident, error) {
	return r.q.ListRecentIncidents(ctx, arg)
}
func (r *sqlcRepo) GetIncident(ctx context.Context, id uuid.UUID) (db.Incident, error) {
	return r.q.GetIncident(ctx, id)
}
func (r *sqlcRepo) CreateIncident(ctx context.Context, arg db.CreateIncidentParams) (db.Incident, error) {
	return r.q.CreateIncident(ctx, arg)
}
func (r *sqlcRepo) UpdateIncidentStatus(ctx context.Context, arg db.UpdateIncidentStatusParams) (db.Incident, error) {
	return r.q.UpdateIncidentStatus(ctx, arg)
}
func (r *sqlcRepo) ListIncidentUpdates(ctx context.Context, incidentID uuid.UUID) ([]db.IncidentUpdate, error) {
	return r.q.ListIncidentUpdates(ctx, incidentID)
}
func (r *sqlcRepo) ListUpdatesForIncidents(ctx context.Context, ids []uuid.UUID) ([]db.IncidentUpdate, error) {
	return r.q.ListUpdatesForIncidents(ctx, ids)
}
func (r *sqlcRepo) CreateIncidentUpdate(ctx context.Context, arg db.CreateIncidentUpdateParams) (db.IncidentUpdate, error) {
	return r.q.CreateIncidentUpdate(ctx, arg)
}
