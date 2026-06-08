package queue

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Repository defines all data-access operations needed by the queue module.
type Repository interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error

	// Token operations
	CreateToken(ctx context.Context, arg db.CreateQueueTokenParams) (db.QueueToken, error)
	GetTokenByEventParticipant(ctx context.Context, eventID, participantID uuid.UUID) (db.QueueToken, error)
	GetTokenByID(ctx context.Context, id uuid.UUID) (db.QueueToken, error)
	ListWaiting(ctx context.Context, arg db.ListWaitingTokensParams) ([]db.QueueToken, error)
	MarkAllowed(ctx context.Context, id uuid.UUID) (db.QueueToken, error)
	MarkCompleted(ctx context.Context, id uuid.UUID) error
	Requeue(ctx context.Context, arg db.RequeueTokenParams) error
	CountByStatus(ctx context.Context, arg db.CountTokensByStatusParams) (int64, error)

	// Admission operations
	CreateAdmission(ctx context.Context, arg db.CreateAdmissionParams) (db.QueueAdmission, error)
	GetActiveAdmission(ctx context.Context, arg db.GetActiveAdmissionByParticipantParams) (db.QueueAdmission, error)
	ConsumeAdmission(ctx context.Context, id uuid.UUID) error
	ListExpiredAdmissions(ctx context.Context, limit int32) ([]db.QueueAdmission, error)
	ExpireAdmission(ctx context.Context, id uuid.UUID) error

	// Control operations
	GetControl(ctx context.Context, eventID uuid.UUID) (db.QueueControl, error)
	UpsertControl(ctx context.Context, arg db.UpsertQueueControlParams) (db.QueueControl, error)
	SetState(ctx context.Context, arg db.SetQueueStateParams) error
	SetRate(ctx context.Context, arg db.SetReleaseRateParams) error
	ListRunningEvents(ctx context.Context) ([]uuid.UUID, error)
}

type sqlcRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

// NewRepository returns a pool-backed Repository.
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

// Token operations

func (r *sqlcRepo) CreateToken(ctx context.Context, arg db.CreateQueueTokenParams) (db.QueueToken, error) {
	return r.q.CreateQueueToken(ctx, arg)
}

func (r *sqlcRepo) GetTokenByEventParticipant(ctx context.Context, eventID, participantID uuid.UUID) (db.QueueToken, error) {
	return r.q.GetQueueTokenByEventParticipant(ctx, db.GetQueueTokenByEventParticipantParams{
		EventID:       eventID,
		ParticipantID: participantID,
	})
}

func (r *sqlcRepo) GetTokenByID(ctx context.Context, id uuid.UUID) (db.QueueToken, error) {
	return r.q.GetQueueTokenByID(ctx, id)
}

func (r *sqlcRepo) ListWaiting(ctx context.Context, arg db.ListWaitingTokensParams) ([]db.QueueToken, error) {
	return r.q.ListWaitingTokens(ctx, arg)
}

func (r *sqlcRepo) MarkAllowed(ctx context.Context, id uuid.UUID) (db.QueueToken, error) {
	return r.q.MarkTokenAllowed(ctx, id)
}

func (r *sqlcRepo) MarkCompleted(ctx context.Context, id uuid.UUID) error {
	return r.q.MarkTokenCompleted(ctx, id)
}

func (r *sqlcRepo) Requeue(ctx context.Context, arg db.RequeueTokenParams) error {
	return r.q.RequeueToken(ctx, arg)
}

func (r *sqlcRepo) CountByStatus(ctx context.Context, arg db.CountTokensByStatusParams) (int64, error) {
	return r.q.CountTokensByStatus(ctx, arg)
}

// Admission operations

func (r *sqlcRepo) CreateAdmission(ctx context.Context, arg db.CreateAdmissionParams) (db.QueueAdmission, error) {
	return r.q.CreateAdmission(ctx, arg)
}

func (r *sqlcRepo) GetActiveAdmission(ctx context.Context, arg db.GetActiveAdmissionByParticipantParams) (db.QueueAdmission, error) {
	return r.q.GetActiveAdmissionByParticipant(ctx, arg)
}

func (r *sqlcRepo) ConsumeAdmission(ctx context.Context, id uuid.UUID) error {
	return r.q.ConsumeAdmission(ctx, id)
}

func (r *sqlcRepo) ListExpiredAdmissions(ctx context.Context, limit int32) ([]db.QueueAdmission, error) {
	return r.q.ListExpiredActiveAdmissions(ctx, limit)
}

func (r *sqlcRepo) ExpireAdmission(ctx context.Context, id uuid.UUID) error {
	return r.q.ExpireAdmission(ctx, id)
}

// Control operations

func (r *sqlcRepo) GetControl(ctx context.Context, eventID uuid.UUID) (db.QueueControl, error) {
	return r.q.GetQueueControl(ctx, eventID)
}

func (r *sqlcRepo) UpsertControl(ctx context.Context, arg db.UpsertQueueControlParams) (db.QueueControl, error) {
	return r.q.UpsertQueueControl(ctx, arg)
}

func (r *sqlcRepo) SetState(ctx context.Context, arg db.SetQueueStateParams) error {
	return r.q.SetQueueState(ctx, arg)
}

func (r *sqlcRepo) SetRate(ctx context.Context, arg db.SetReleaseRateParams) error {
	return r.q.SetReleaseRate(ctx, arg)
}

func (r *sqlcRepo) ListRunningEvents(ctx context.Context) ([]uuid.UUID, error) {
	return r.q.ListEventsWithRunningQueue(ctx)
}
