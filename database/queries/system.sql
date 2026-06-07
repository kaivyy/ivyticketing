-- name: HealthPing :one
SELECT checked_at FROM schema_health WHERE id = 1;
