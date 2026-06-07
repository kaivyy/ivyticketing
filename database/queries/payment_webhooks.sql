-- name: CreatePaymentWebhook :one
INSERT INTO payment_webhooks (
    gateway, event_type, merchant_reference, gateway_reference, signature,
    signature_valid, payload, processing_status
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
RETURNING *;

-- name: ClaimWebhookDedupe :one
UPDATE payment_webhooks
SET dedupe_key = $2
WHERE id = $1
RETURNING *;

-- name: MarkWebhookProcessed :exec
UPDATE payment_webhooks
SET processing_status = $2, processed_payment_id = $3, error_detail = $4, processed_at = now()
WHERE id = $1;
