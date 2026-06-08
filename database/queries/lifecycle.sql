-- name: CreateLifecycle :one
INSERT INTO registration_lifecycles
    (organization_id, event_id, category_id, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetLifecycleByCategory :one
SELECT * FROM registration_lifecycles
WHERE event_id = $1 AND category_id = $2
  AND status NOT IN ('COMPLETED','CANCELLED')
LIMIT 1;

-- name: GetLifecycleByCategoryID :one
SELECT * FROM registration_lifecycles
WHERE category_id = $1
  AND status NOT IN ('COMPLETED','CANCELLED')
LIMIT 1;

-- name: ActivateLifecycle :one
UPDATE registration_lifecycles SET status = 'ACTIVE', updated_at = now()
WHERE id = $1 AND status = 'DRAFT'
RETURNING *;

-- name: UpdateLifecycleStatus :one
UPDATE registration_lifecycles SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateLifecyclePhase :one
INSERT INTO lifecycle_phases
    (lifecycle_id, phase_index, registration_mode, label, opens_at, closes_at,
     capacity_override, auto_advance)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetActivePhaseForMode :one
SELECT lp.* FROM lifecycle_phases lp
JOIN registration_lifecycles rl ON rl.id = lp.lifecycle_id
WHERE lp.lifecycle_id = $1
  AND lp.registration_mode = $2
  AND lp.status = 'ACTIVE'
  AND rl.status = 'ACTIVE'
LIMIT 1;

-- name: ListPhasesForLifecycle :many
SELECT * FROM lifecycle_phases WHERE lifecycle_id = $1 ORDER BY phase_index ASC;

-- name: UpdateLifecyclePhaseStatus :one
UPDATE lifecycle_phases
SET status = $2,
    activated_at = CASE WHEN $2 = 'ACTIVE' THEN now() ELSE activated_at END,
    completed_at = CASE WHEN $2 IN ('COMPLETED','SKIPPED') THEN now() ELSE completed_at END
WHERE id = $1
RETURNING *;

-- name: ListPhasesForAutoAdvance :many
SELECT lp.* FROM lifecycle_phases lp
JOIN registration_lifecycles rl ON rl.id = lp.lifecycle_id
WHERE lp.status = 'ACTIVE'
  AND lp.auto_advance = true
  AND lp.closes_at < now()
  AND rl.status = 'ACTIVE';

-- name: GetNextPendingPhase :one
SELECT * FROM lifecycle_phases
WHERE lifecycle_id = $1 AND status = 'PENDING'
ORDER BY phase_index ASC
LIMIT 1;
