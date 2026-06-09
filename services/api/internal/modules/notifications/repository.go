package notifications

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Repository persists and retrieves notification records.
type Repository interface {
	Create(ctx context.Context, participantID uuid.UUID, typ, channel, status string, payload []byte) (db.Notification, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, attempts int32, lastAttemptAt, sentAt *time.Time) error
	ListPending(ctx context.Context, limit int32) ([]db.Notification, error)
	GetByID(ctx context.Context, id uuid.UUID) (db.Notification, error)
}

type sqlcRepo struct {
	q *db.Queries
}

// NewRepository creates a Repository backed by a pgxpool.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &sqlcRepo{q: db.New(pool)}
}

func (r *sqlcRepo) Create(ctx context.Context, participantID uuid.UUID, typ, channel, status string, payload []byte) (db.Notification, error) {
	return r.q.InsertNotification(ctx, db.InsertNotificationParams{
		ParticipantID: participantID,
		Type:          typ,
		Channel:       channel,
		Status:        status,
		Payload:       payload,
	})
}

func (r *sqlcRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string, attempts int32, lastAttemptAt, sentAt *time.Time) error {
	var la, sa pgtype.Timestamptz
	if lastAttemptAt != nil {
		la = pgtype.Timestamptz{Time: *lastAttemptAt, Valid: true}
	}
	if sentAt != nil {
		sa = pgtype.Timestamptz{Time: *sentAt, Valid: true}
	}
	return r.q.UpdateNotificationStatus(ctx, db.UpdateNotificationStatusParams{
		ID:            id,
		Status:        status,
		Attempts:      attempts,
		LastAttemptAt: la,
		SentAt:        sa,
	})
}

func (r *sqlcRepo) ListPending(ctx context.Context, limit int32) ([]db.Notification, error) {
	return r.q.ListPendingNotifications(ctx, limit)
}

func (r *sqlcRepo) GetByID(ctx context.Context, id uuid.UUID) (db.Notification, error) {
	return r.q.GetNotificationByID(ctx, id)
}
