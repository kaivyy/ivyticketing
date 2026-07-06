package notifications

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/notifications/templates"
)

// Repository persists and retrieves notification records.
type Repository interface {
	Create(ctx context.Context, participantID uuid.UUID, typ, channel, status string, payload []byte) (db.Notification, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, attempts int32, lastAttemptAt, sentAt *time.Time) error
	UpdateRetry(ctx context.Context, id uuid.UUID, status string, attempts int32, lastError *string, nextRetryAt, sentAt *time.Time) error
	ListPending(ctx context.Context, limit int32) ([]db.Notification, error)
	ListRetryable(ctx context.Context, maxAttempts, limit int32) ([]db.Notification, error)
	GetByID(ctx context.Context, id uuid.UUID) (db.Notification, error)
	GetDefaultTemplate(ctx context.Context, typ, channel string) (templates.DBTemplate, error)
}

type sqlcRepo struct{ q *db.Queries }

func NewRepository(pool *pgxpool.Pool) Repository { return &sqlcRepo{q: db.New(pool)} }

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

func (r *sqlcRepo) UpdateRetry(ctx context.Context, id uuid.UUID, status string, attempts int32, lastError *string, nextRetryAt, sentAt *time.Time) error {
	var le pgtype.Text
	if lastError != nil {
		le = pgtype.Text{String: *lastError, Valid: true}
	}
	var nra pgtype.Timestamptz
	if nextRetryAt != nil {
		nra = pgtype.Timestamptz{Time: *nextRetryAt, Valid: true}
	}
	var sa pgtype.Timestamptz
	if sentAt != nil {
		sa = pgtype.Timestamptz{Time: *sentAt, Valid: true}
	}
	return r.q.UpdateNotificationRetry(ctx, db.UpdateNotificationRetryParams{
		ID:          id,
		Status:      status,
		Attempts:    attempts,
		LastError:   le,
		NextRetryAt: nra,
		SentAt:      sa,
	})
}

func (r *sqlcRepo) ListPending(ctx context.Context, limit int32) ([]db.Notification, error) {
	return r.q.ListPendingNotifications(ctx, limit)
}

func (r *sqlcRepo) ListRetryable(ctx context.Context, maxAttempts, limit int32) ([]db.Notification, error) {
	return r.q.ListRetryableNotifications(ctx, db.ListRetryableNotificationsParams{
		Attempts: maxAttempts,
		Limit:    limit,
	})
}

func (r *sqlcRepo) GetByID(ctx context.Context, id uuid.UUID) (db.Notification, error) {
	return r.q.GetNotificationByID(ctx, id)
}

// GetDefaultTemplate implements templates.DBReader for the resolver.
func (r *sqlcRepo) GetDefaultTemplate(ctx context.Context, typ, channel string) (templates.DBTemplate, error) {
	tmpl, err := r.q.GetDefaultNotificationTemplate(ctx, db.GetDefaultNotificationTemplateParams{
		Type:    typ,
		Channel: channel,
	})
	if err != nil {
		return templates.DBTemplate{}, err
	}
	return templates.DBTemplate{
		Subject:  tmpl.Subject,
		HTMLBody: tmpl.BodyHtml,
	}, nil
}
