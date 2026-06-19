package db

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ListRetryableNotifications queries retryable notifications. Use only until
// sqlc is regenerated.
const listRetryableNotifications = `-- name: ListRetryableNotifications :many
SELECT id, participant_id, type, channel, status, payload, attempts, last_attempt_at, sent_at, created_at, next_retry_at, last_error FROM notifications
WHERE status IN ('pending','failed')
  AND attempts < $1
  AND (next_retry_at IS NULL OR next_retry_at <= now())
ORDER BY created_at
FOR UPDATE SKIP LOCKED
LIMIT $2`

type ListRetryableNotificationsParams struct {
	MaxAttempts int32
	Limit       int32
}

func (q *Queries) ListRetryableNotifications(ctx context.Context, arg ListRetryableNotificationsParams) ([]Notification, error) {
	rows, err := q.db.Query(ctx, listRetryableNotifications, arg.MaxAttempts, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Notification
	for rows.Next() {
		var i Notification
		if err := rows.Scan(
			&i.ID,
			&i.ParticipantID,
			&i.Type,
			&i.Channel,
			&i.Status,
			&i.Payload,
			&i.Attempts,
			&i.LastAttemptAt,
			&i.SentAt,
			&i.CreatedAt,
			&i.NextRetryAt,
			&i.LastError,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// UpdateNotificationRetry updates a notification row during retry.
const updateNotificationRetry = `-- name: UpdateNotificationRetry :exec
UPDATE notifications
SET status = $2,
    attempts = $3,
    last_attempt_at = now(),
    last_error = $4,
    next_retry_at = $5,
    sent_at = $6
WHERE id = $1`

type UpdateNotificationRetryParams struct {
	ID           uuid.UUID
	Status       string
	Attempts     int32
	LastError    sql.NullString
	NextRetryAt  pgtype.Timestamptz
	SentAt       pgtype.Timestamptz
}

func (q *Queries) UpdateNotificationRetry(ctx context.Context, arg UpdateNotificationRetryParams) error {
	_, err := q.db.Exec(ctx, updateNotificationRetry,
		arg.ID,
		arg.Status,
		arg.Attempts,
		arg.LastError,
		arg.NextRetryAt,
		arg.SentAt,
	)
	return err
}

// GetDefaultNotificationTemplate queries the default template for a type/channel.
const getDefaultNotificationTemplate = `-- name: GetDefaultNotificationTemplate :one
SELECT id, org_id, type, channel, subject, body_html, body_text, is_default, created_at, updated_at FROM notification_templates
WHERE type = $1 AND channel = $2 AND is_default = TRUE`

func (q *Queries) GetDefaultNotificationTemplate(ctx context.Context, arg GetDefaultNotificationTemplateParams) (NotificationTemplate, error) {
	row := q.db.QueryRow(ctx, getDefaultNotificationTemplate, arg.Type, arg.Channel)
	var i NotificationTemplate
	err := row.Scan(
		&i.ID,
		&i.OrgID,
		&i.Type,
		&i.Channel,
		&i.Subject,
		&i.BodyHtml,
		&i.BodyText,
		&i.IsDefault,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}

type GetDefaultNotificationTemplateParams struct {
	Type    string
	Channel string
}
