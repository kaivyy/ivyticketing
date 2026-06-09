package lifecycle

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	CreateLifecycle(ctx context.Context, arg db.CreateLifecycleParams) (db.RegistrationLifecycle, error)
	GetLifecycleByCategory(ctx context.Context, arg db.GetLifecycleByCategoryParams) (db.RegistrationLifecycle, error)
	GetLifecycleByCategoryID(ctx context.Context, categoryID uuid.UUID) (db.RegistrationLifecycle, error)
	ActivateLifecycle(ctx context.Context, id uuid.UUID) (db.RegistrationLifecycle, error)
	UpdateLifecycleStatus(ctx context.Context, arg db.UpdateLifecycleStatusParams) (db.RegistrationLifecycle, error)
	CreateLifecyclePhase(ctx context.Context, arg db.CreateLifecyclePhaseParams) (db.LifecyclePhase, error)
	GetActivePhaseForMode(ctx context.Context, arg db.GetActivePhaseForModeParams) (db.LifecyclePhase, error)
	ListPhasesForLifecycle(ctx context.Context, lifecycleID uuid.UUID) ([]db.LifecyclePhase, error)
	UpdateLifecyclePhaseStatus(ctx context.Context, arg db.UpdateLifecyclePhaseStatusParams) (db.LifecyclePhase, error)
	ListPhasesForAutoAdvance(ctx context.Context) ([]db.LifecyclePhase, error)
	GetNextPendingPhase(ctx context.Context, lifecycleID uuid.UUID) (db.LifecyclePhase, error)
}

type sqlcRepo struct{ q *db.Queries }

func NewRepository(pool *pgxpool.Pool) Repository { return &sqlcRepo{q: db.New(pool)} }

func (r *sqlcRepo) CreateLifecycle(ctx context.Context, arg db.CreateLifecycleParams) (db.RegistrationLifecycle, error) {
	return r.q.CreateLifecycle(ctx, arg)
}

func (r *sqlcRepo) GetLifecycleByCategory(ctx context.Context, arg db.GetLifecycleByCategoryParams) (db.RegistrationLifecycle, error) {
	return r.q.GetLifecycleByCategory(ctx, arg)
}

func (r *sqlcRepo) GetLifecycleByCategoryID(ctx context.Context, categoryID uuid.UUID) (db.RegistrationLifecycle, error) {
	return r.q.GetLifecycleByCategoryID(ctx, categoryID)
}

func (r *sqlcRepo) ActivateLifecycle(ctx context.Context, id uuid.UUID) (db.RegistrationLifecycle, error) {
	return r.q.ActivateLifecycle(ctx, id)
}

func (r *sqlcRepo) UpdateLifecycleStatus(ctx context.Context, arg db.UpdateLifecycleStatusParams) (db.RegistrationLifecycle, error) {
	return r.q.UpdateLifecycleStatus(ctx, arg)
}

func (r *sqlcRepo) CreateLifecyclePhase(ctx context.Context, arg db.CreateLifecyclePhaseParams) (db.LifecyclePhase, error) {
	return r.q.CreateLifecyclePhase(ctx, arg)
}

func (r *sqlcRepo) GetActivePhaseForMode(ctx context.Context, arg db.GetActivePhaseForModeParams) (db.LifecyclePhase, error) {
	return r.q.GetActivePhaseForMode(ctx, arg)
}

func (r *sqlcRepo) ListPhasesForLifecycle(ctx context.Context, lifecycleID uuid.UUID) ([]db.LifecyclePhase, error) {
	return r.q.ListPhasesForLifecycle(ctx, lifecycleID)
}

func (r *sqlcRepo) UpdateLifecyclePhaseStatus(ctx context.Context, arg db.UpdateLifecyclePhaseStatusParams) (db.LifecyclePhase, error) {
	return r.q.UpdateLifecyclePhaseStatus(ctx, arg)
}

func (r *sqlcRepo) ListPhasesForAutoAdvance(ctx context.Context) ([]db.LifecyclePhase, error) {
	return r.q.ListPhasesForAutoAdvance(ctx)
}

func (r *sqlcRepo) GetNextPendingPhase(ctx context.Context, lifecycleID uuid.UUID) (db.LifecyclePhase, error) {
	return r.q.GetNextPendingPhase(ctx, lifecycleID)
}
