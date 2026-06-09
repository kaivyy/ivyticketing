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
