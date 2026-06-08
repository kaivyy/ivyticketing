-- name: CreateWaitlist :one
INSERT INTO waitlists
    (organization_id, event_id, category_id, mode, max_promotion_batch,
     promotion_window_hours, auto_promote)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetWaitlist :one
SELECT * FROM waitlists WHERE id = $1;

-- name: GetWaitlistByCategory :one
SELECT * FROM waitlists
WHERE event_id = $1 AND category_id = $2 AND status = 'ACTIVE'
LIMIT 1;

-- name: SetWaitlistPool :exec
UPDATE waitlists SET pool_id = $2 WHERE id = $1;

-- name: SetWaitlistSeed :exec
UPDATE waitlists SET seed = $2 WHERE id = $1 AND seed IS NULL;

-- name: UpdateWaitlistStatus :exec
UPDATE waitlists SET status = $2 WHERE id = $1;

-- name: JoinWaitlist :one
INSERT INTO waitlist_entries
    (waitlist_id, participant_id, event_id, category_id, source, source_ref_id,
     rank, promotion_window_hours)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetWaitlistEntry :one
SELECT * FROM waitlist_entries
WHERE waitlist_id = $1 AND participant_id = $2 AND status NOT IN ('WITHDRAWN','EXPIRED')
LIMIT 1;

-- name: ListWaitingEntries :many
SELECT * FROM waitlist_entries
WHERE waitlist_id = $1 AND status = 'WAITING'
ORDER BY rank ASC
LIMIT $2;

-- name: UpdateWaitlistEntryStatus :one
UPDATE waitlist_entries
SET status = $2,
    promoted_at = CASE WHEN $2 = 'PROMOTED' THEN now() ELSE promoted_at END,
    access_grant_id = COALESCE($3, access_grant_id)
WHERE id = $1
RETURNING *;

-- name: CountWaitlistPosition :one
SELECT count(*) FROM waitlist_entries
WHERE waitlist_id = $1 AND status = 'WAITING' AND rank < $2;
