-- name: CreateBallotDraw :one
INSERT INTO ballot_draws
    (organization_id, event_id, category_id, quota, waitlist_size,
     payment_window_hours, application_opens_at, application_closes_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetBallotDraw :one
SELECT * FROM ballot_draws WHERE id = $1;

-- name: GetActiveBallotDrawByCategory :one
SELECT * FROM ballot_draws
WHERE event_id = $1 AND category_id = $2 AND status NOT IN ('ANNOUNCED')
ORDER BY created_at DESC
LIMIT 1;

-- name: UpdateBallotDrawStatus :one
UPDATE ballot_draws
SET status = $2,
    draw_at = CASE WHEN $2 = 'DRAWN' THEN now() ELSE draw_at END,
    announced_at = CASE WHEN $2 = 'ANNOUNCED' THEN now() ELSE announced_at END,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetBallotDrawSeed :one
UPDATE ballot_draws SET seed = $2, draw_nonce = $3, updated_at = now()
WHERE id = $1 AND seed IS NULL
RETURNING *;

-- name: SetBallotDrawPools :exec
UPDATE ballot_draws SET winner_pool_id = $2, waitlist_id = $3, updated_at = now()
WHERE id = $1;

-- name: CreateBallotEntry :one
INSERT INTO ballot_entries
    (draw_id, organization_id, event_id, category_id, participant_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetBallotEntry :one
SELECT * FROM ballot_entries WHERE draw_id = $1 AND participant_id = $2;

-- name: GetBallotEntryByID :one
SELECT * FROM ballot_entries WHERE id = $1;

-- name: ListAppliedEntriesForDraw :many
SELECT * FROM ballot_entries
WHERE draw_id = $1 AND status = 'APPLIED'
ORDER BY id ASC;

-- name: UpdateBallotEntryStatus :one
UPDATE ballot_entries
SET status = $2,
    payment_deadline = COALESCE($3, payment_deadline),
    access_grant_id  = COALESCE($4, access_grant_id),
    converted_at     = CASE WHEN $2 = 'CONVERTED' THEN now() ELSE converted_at END
WHERE id = $1
RETURNING *;

-- name: BulkUpdateBallotOutcome :exec
UPDATE ballot_entries SET status = $2 WHERE id = ANY($1::uuid[]);

-- name: InsertBallotDrawResult :exec
INSERT INTO ballot_draw_results (draw_id, ballot_entry_id, outcome, rank, result_hash)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (draw_id, ballot_entry_id) DO NOTHING;

-- name: ListBallotDrawResults :many
SELECT bdr.rank, bdr.outcome, bdr.result_hash,
       be.id AS ballot_entry_id, be.participant_id
FROM ballot_draw_results bdr
JOIN ballot_entries be ON be.id = bdr.ballot_entry_id
WHERE bdr.draw_id = $1
ORDER BY bdr.rank ASC
LIMIT $2 OFFSET $3;

-- name: ListAllDrawResults :many
SELECT bdr.rank, bdr.outcome, bdr.result_hash,
       be.id AS ballot_entry_id, be.participant_id
FROM ballot_draw_results bdr
JOIN ballot_entries be ON be.id = bdr.ballot_entry_id
WHERE bdr.draw_id = $1
ORDER BY bdr.rank ASC;

-- name: CountBallotDrawResults :one
SELECT count(*) FROM ballot_draw_results WHERE draw_id = $1 AND outcome = $2;

-- name: ListWinnerEntries :many
SELECT * FROM ballot_entries
WHERE draw_id = $1 AND status = 'WINNER';

-- name: ListExpiringWinners :many
SELECT * FROM ballot_entries
WHERE status = 'WINNER' AND payment_deadline < now()
LIMIT $1;

-- name: GetBallotEntryByParticipant :many
SELECT * FROM ballot_entries
WHERE participant_id = $1
ORDER BY applied_at DESC
LIMIT $2 OFFSET $3;
