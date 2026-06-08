-- name: CreateAccessPool :one
INSERT INTO access_pools
    (organization_id, event_id, category_id, pool_type, name, total_slots,
     is_visible_to_participants, eligibility_rule, valid_from, valid_until, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetAccessPool :one
SELECT * FROM access_pools WHERE id = $1;

-- name: ReservePoolSlot :one
UPDATE access_pools
SET reserved_slots = reserved_slots + 1
WHERE id = $1 AND reserved_slots + used_slots < total_slots
RETURNING *;

-- name: ConsumePoolSlot :exec
UPDATE access_pools
SET reserved_slots = reserved_slots - 1, used_slots = used_slots + 1
WHERE id = $1;

-- name: ReleasePoolSlot :exec
UPDATE access_pools
SET reserved_slots = reserved_slots - 1, released_slots = released_slots + 1
WHERE id = $1;

-- name: CreateAccessGrant :one
INSERT INTO access_grants
    (pool_id, participant_id, event_id, category_id, code_id, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAccessGrant :one
SELECT * FROM access_grants WHERE id = $1;

-- name: GetActiveGrantForParticipant :one
SELECT * FROM access_grants
WHERE participant_id = $1 AND category_id = $2 AND status = 'ACTIVE'
ORDER BY granted_at DESC
LIMIT 1;

-- name: ExpireGrant :exec
UPDATE access_grants SET status = 'EXPIRED' WHERE id = $1;

-- name: ConsumeGrant :exec
UPDATE access_grants
SET status = 'CONSUMED', consumed_at = now(), order_id = $2
WHERE id = $1;

-- name: ListExpiredActiveGrants :many
SELECT * FROM access_grants
WHERE status = 'ACTIVE' AND expires_at < now()
LIMIT $1;
