-- name: ListStatusComponents :many
SELECT * FROM status_components ORDER BY sort_order, key;

-- name: UpdateStatusComponent :one
UPDATE status_components
SET status = $2, updated_at = now()
WHERE key = $1
RETURNING *;

-- name: ListActiveIncidents :many
SELECT * FROM incidents
WHERE resolved_at IS NULL
ORDER BY started_at DESC;

-- name: ListRecentIncidents :many
SELECT * FROM incidents
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetIncident :one
SELECT * FROM incidents WHERE id = $1;

-- name: CreateIncident :one
INSERT INTO incidents (title, impact, status)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateIncidentStatus :one
UPDATE incidents
SET status = $2,
    resolved_at = CASE WHEN $2 = 'RESOLVED' THEN now() ELSE resolved_at END,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListIncidentUpdates :many
SELECT * FROM incident_updates
WHERE incident_id = $1
ORDER BY created_at DESC;

-- name: CreateIncidentUpdate :one
INSERT INTO incident_updates (incident_id, status, body)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListUpdatesForIncidents :many
SELECT * FROM incident_updates
WHERE incident_id = ANY($1::uuid[])
ORDER BY created_at DESC;
