-- name: CreateQueueToken :one
INSERT INTO queue_tokens (organization_id, event_id, participant_id, status, pool, score)
VALUES ($1,$2,$3,'WAITING',$4,$5)
ON CONFLICT (event_id, participant_id) DO NOTHING
RETURNING *;

-- name: GetQueueTokenByEventParticipant :one
SELECT * FROM queue_tokens WHERE event_id = $1 AND participant_id = $2;

-- name: GetQueueTokenByID :one
SELECT * FROM queue_tokens WHERE id = $1;

-- name: ListWaitingTokens :many
SELECT * FROM queue_tokens
WHERE event_id = $1 AND status = 'WAITING'
ORDER BY pool DESC, score ASC
LIMIT $2;

-- name: MarkTokenAllowed :one
UPDATE queue_tokens SET status='ALLOWED', allowed_at=now(), updated_at=now()
WHERE id = $1 AND status = 'WAITING'
RETURNING *;

-- name: MarkTokenCompleted :exec
UPDATE queue_tokens SET status='COMPLETED', completed_at=now(), updated_at=now()
WHERE id = $1 AND status = 'ALLOWED';

-- name: RequeueToken :exec
UPDATE queue_tokens SET status='WAITING', score=$2, allowed_at=NULL, updated_at=now()
WHERE id = $1 AND status = 'ALLOWED';

-- name: CountTokensByStatus :one
SELECT count(*) FROM queue_tokens WHERE event_id = $1 AND status = $2;

-- name: ListWaitingTokensAll :many
SELECT * FROM queue_tokens WHERE event_id = $1 AND status = 'WAITING' ORDER BY pool DESC, score ASC;

-- name: CreateAdmission :one
INSERT INTO queue_admissions (token_id, event_id, participant_id, checkout_expires_at, status)
VALUES ($1,$2,$3,$4,'ACTIVE')
RETURNING *;

-- name: GetActiveAdmissionByParticipant :one
SELECT * FROM queue_admissions
WHERE event_id = $1 AND participant_id = $2 AND status = 'ACTIVE';

-- name: ConsumeAdmission :exec
UPDATE queue_admissions SET status='CONSUMED' WHERE id = $1 AND status = 'ACTIVE';

-- name: ListExpiredActiveAdmissions :many
SELECT * FROM queue_admissions
WHERE status = 'ACTIVE' AND checkout_expires_at < now()
ORDER BY checkout_expires_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: ExpireAdmission :exec
UPDATE queue_admissions SET status='EXPIRED' WHERE id = $1 AND status = 'ACTIVE';

-- name: GetQueueControl :one
SELECT * FROM queue_control WHERE event_id = $1;

-- name: UpsertQueueControl :one
INSERT INTO queue_control (event_id, state, release_rate, randomization_seed, sale_start_at, presale_pool_open_at)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (event_id) DO UPDATE SET
    state = EXCLUDED.state,
    release_rate = EXCLUDED.release_rate,
    randomization_seed = EXCLUDED.randomization_seed,
    sale_start_at = EXCLUDED.sale_start_at,
    presale_pool_open_at = EXCLUDED.presale_pool_open_at,
    updated_at = now()
RETURNING *;

-- name: SetQueueState :exec
UPDATE queue_control SET state=$2, updated_at=now() WHERE event_id=$1;

-- name: SetReleaseRate :exec
UPDATE queue_control SET release_rate=$2, updated_at=now() WHERE event_id=$1;

-- name: ListEventsWithRunningQueue :many
SELECT event_id FROM queue_control WHERE state = 'RUNNING';
