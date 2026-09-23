-- name: ListPaymentChannelsByEvent :many
SELECT * FROM event_payment_channels WHERE event_id = $1 ORDER BY channel_code;

-- name: UpsertEventPaymentChannel :one
INSERT INTO event_payment_channels (event_id, channel_code, is_enabled, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (event_id, channel_code)
DO UPDATE SET is_enabled = EXCLUDED.is_enabled, updated_at = now()
RETURNING *;

-- name: IsPaymentChannelEnabled :one
SELECT COALESCE(
    (SELECT is_enabled FROM event_payment_channels WHERE event_id = $1 AND channel_code = $2),
    true
)::boolean AS is_enabled;
