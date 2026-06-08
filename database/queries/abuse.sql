-- name: ListPlatformSettings :many
SELECT * FROM platform_settings;

-- name: UpsertPlatformSetting :one
INSERT INTO platform_settings (key, value, updated_by, updated_at)
VALUES ($1,$2,$3,now())
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = now()
RETURNING *;

-- name: GetBlockedSubject :one
SELECT * FROM blocked_subjects
WHERE subject_type = $1 AND subject_value = $2
  AND (expires_at IS NULL OR expires_at > now());

-- name: ListBlockedSubjects :many
SELECT * FROM blocked_subjects ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: UpsertBlockedSubject :one
INSERT INTO blocked_subjects (subject_type, subject_value, reason, blocked_by, expires_at)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (subject_type, subject_value) DO UPDATE SET reason = EXCLUDED.reason, blocked_by = EXCLUDED.blocked_by, expires_at = EXCLUDED.expires_at, created_at = now()
RETURNING *;

-- name: DeleteBlockedSubject :exec
DELETE FROM blocked_subjects WHERE subject_type = $1 AND subject_value = $2;

-- name: ListIPRules :many
SELECT * FROM ip_rules ORDER BY created_at DESC;

-- name: CreateIPRule :one
INSERT INTO ip_rules (cidr, rule, note, created_by) VALUES ($1,$2,$3,$4) RETURNING *;

-- name: DeleteIPRule :exec
DELETE FROM ip_rules WHERE id = $1;

-- name: InsertAbuseLog :exec
INSERT INTO abuse_log (subject_type, subject_value, action, category, fingerprint, ip, user_id, detail)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8);

-- name: ListAbuseLog :many
SELECT * FROM abuse_log ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: GetReputation :one
SELECT * FROM ip_reputation WHERE subject_type = $1 AND subject_value = $2;

-- name: BumpReputation :one
INSERT INTO ip_reputation (subject_type, subject_value, score, updated_at)
VALUES ($1,$2,$3,now())
ON CONFLICT (subject_type, subject_value) DO UPDATE SET score = ip_reputation.score + EXCLUDED.score, updated_at = now()
RETURNING *;

-- name: CountActiveQueueTokensByUser :one
SELECT count(*) FROM queue_tokens WHERE participant_id = $1 AND status IN ('WAITING','ALLOWED');
