-- Phase 23 — Enterprise API: API keys, webhook endpoints, delivery ledger.

-- name: CreateAPIKey :one
INSERT INTO api_keys (organization_id, name, key_prefix, key_hash, scopes, rate_limit_per_min)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListAPIKeysByOrg :many
SELECT * FROM api_keys
WHERE organization_id = $1
ORDER BY created_at DESC;

-- name: GetAPIKeyByHash :one
SELECT * FROM api_keys
WHERE key_hash = $1 AND revoked_at IS NULL;

-- name: RevokeAPIKey :one
UPDATE api_keys
SET revoked_at = now()
WHERE id = $1 AND organization_id = $2 AND revoked_at IS NULL
RETURNING *;

-- name: TouchAPIKey :exec
UPDATE api_keys SET last_used_at = now() WHERE id = $1;

-- name: CreateWebhookEndpoint :one
INSERT INTO webhook_endpoints (organization_id, url, secret, events, is_active)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListWebhookEndpointsByOrg :many
SELECT * FROM webhook_endpoints
WHERE organization_id = $1
ORDER BY created_at DESC;

-- name: ListActiveWebhookEndpointsForEvent :many
SELECT * FROM webhook_endpoints
WHERE organization_id = $1
  AND is_active = true
  AND events ? sqlc.arg(event_type)::text;

-- name: GetWebhookEndpointByID :one
SELECT * FROM webhook_endpoints WHERE id = $1;

-- name: DeleteWebhookEndpoint :exec
DELETE FROM webhook_endpoints WHERE id = $1 AND organization_id = $2;

-- name: EnqueueWebhookDelivery :one
INSERT INTO webhook_deliveries (endpoint_id, organization_id, event_type, event_key, payload)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (endpoint_id, event_key) DO NOTHING
RETURNING *;

-- name: ListDueWebhookDeliveries :many
SELECT * FROM webhook_deliveries
WHERE status = 'PENDING' AND next_attempt_at <= now()
ORDER BY next_attempt_at
LIMIT $1;

-- name: MarkWebhookDelivered :exec
UPDATE webhook_deliveries
SET status = 'DELIVERED', delivered_at = now(), attempts = attempts + 1, updated_at = now()
WHERE id = $1;

-- name: MarkWebhookRetry :exec
UPDATE webhook_deliveries
SET attempts = attempts + 1,
    last_error = $2,
    next_attempt_at = $3,
    status = CASE WHEN attempts + 1 >= $4 THEN 'DEAD' ELSE 'PENDING' END,
    updated_at = now()
WHERE id = $1;

-- name: ListWebhookDeliveriesByOrg :many
SELECT * FROM webhook_deliveries
WHERE organization_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
