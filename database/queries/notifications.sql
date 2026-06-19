-- name: InsertNotification :one
INSERT INTO notifications (participant_id, type, channel, status, payload)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateNotificationStatus :exec
UPDATE notifications
SET status = $2,
    attempts = $3,
    last_attempt_at = $4,
    sent_at = $5
WHERE id = $1;

-- name: ListPendingNotifications :many
SELECT * FROM notifications
WHERE status = 'pending'
ORDER BY created_at
LIMIT $1;

-- name: GetNotificationByID :one
SELECT * FROM notifications WHERE id = $1;

-- name: ListRetryableNotifications :many
SELECT * FROM notifications
WHERE status IN ('pending','failed')
  AND attempts < $1
  AND (next_retry_at IS NULL OR next_retry_at <= now())
ORDER BY created_at
FOR UPDATE SKIP LOCKED
LIMIT $2;

-- name: UpdateNotificationRetry :exec
UPDATE notifications
SET status = $2,
    attempts = $3,
    last_attempt_at = now(),
    last_error = $4,
    next_retry_at = $5,
    sent_at = $6
WHERE id = $1;

-- name: GetDefaultNotificationTemplate :one
SELECT * FROM notification_templates
WHERE type = $1 AND channel = $2 AND is_default = TRUE;
