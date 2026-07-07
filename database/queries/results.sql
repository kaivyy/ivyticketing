-- Phase 24 — Result, Certificate & Timing Integration.

-- UpsertRaceResult imports or updates one finisher row keyed by (event, bib).
-- Re-importing the same bib overwrites the mutable fields (idempotent import),
-- so a corrected CSV re-run converges. Ranks are intentionally NOT touched here;
-- they are (re)computed by the ranking pass after an import completes.
-- name: UpsertRaceResult :one
INSERT INTO race_results (
    organization_id, event_id, category_id, ticket_id, bib_number,
    participant_name, gender, age, age_group, status,
    chip_time_ms, gun_time_ms, source, finished_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13, $14
)
ON CONFLICT (event_id, bib_number) DO UPDATE SET
    category_id      = EXCLUDED.category_id,
    ticket_id        = EXCLUDED.ticket_id,
    participant_name = EXCLUDED.participant_name,
    gender           = EXCLUDED.gender,
    age              = EXCLUDED.age,
    age_group        = EXCLUDED.age_group,
    status           = EXCLUDED.status,
    chip_time_ms     = EXCLUDED.chip_time_ms,
    gun_time_ms      = EXCLUDED.gun_time_ms,
    source           = EXCLUDED.source,
    finished_at      = EXCLUDED.finished_at,
    updated_at       = now()
RETURNING *;

-- name: GetRaceResultByID :one
SELECT * FROM race_results WHERE id = $1;

-- name: GetRaceResultByBib :one
SELECT * FROM race_results WHERE event_id = $1 AND bib_number = $2;

-- name: GetRaceResultByTicket :one
SELECT * FROM race_results WHERE ticket_id = $1;

-- ListRaceResults returns the event's results ordered by overall rank (finishers
-- first, ranked; then DNF/DNS with NULL rank sorted last). Optional filters:
-- category_id and gender are applied only when the arg is non-empty/non-null.
-- name: ListRaceResults :many
SELECT * FROM race_results
WHERE event_id = $1
  AND (sqlc.narg(category_id)::uuid IS NULL OR category_id = sqlc.narg(category_id))
  AND (sqlc.narg(gender)::text IS NULL OR gender = sqlc.narg(gender))
ORDER BY rank_overall ASC NULLS LAST, chip_time_ms ASC NULLS LAST, participant_name ASC
LIMIT $2 OFFSET $3;

-- name: CountRaceResults :one
SELECT count(*) FROM race_results
WHERE event_id = $1
  AND (sqlc.narg(category_id)::uuid IS NULL OR category_id = sqlc.narg(category_id))
  AND (sqlc.narg(gender)::text IS NULL OR gender = sqlc.narg(gender));

-- name: DeleteRaceResultsByEvent :exec
DELETE FROM race_results WHERE event_id = $1;

-- --- ranking passes ---
-- Each pass recomputes a rank column with a window function over FINISHED rows
-- ordered by net (chip) time, falling back to gun time when chip is absent.
-- DNF/DNS rows keep NULL ranks. Ties share the ordering position via RANK().

-- name: RankOverall :exec
UPDATE race_results r
SET rank_overall = s.rnk, updated_at = now()
FROM (
    SELECT rr.id, RANK() OVER (
        ORDER BY COALESCE(rr.chip_time_ms, rr.gun_time_ms) ASC
    ) AS rnk
    FROM race_results rr
    WHERE rr.event_id = $1 AND rr.status = 'FINISHED'
      AND COALESCE(rr.chip_time_ms, rr.gun_time_ms) IS NOT NULL
) s
WHERE r.id = s.id;

-- name: RankGender :exec
UPDATE race_results r
SET rank_gender = s.rnk, updated_at = now()
FROM (
    SELECT rr.id, RANK() OVER (
        PARTITION BY rr.gender
        ORDER BY COALESCE(rr.chip_time_ms, rr.gun_time_ms) ASC
    ) AS rnk
    FROM race_results rr
    WHERE rr.event_id = $1 AND rr.status = 'FINISHED' AND rr.gender IS NOT NULL
      AND COALESCE(rr.chip_time_ms, rr.gun_time_ms) IS NOT NULL
) s
WHERE r.id = s.id;

-- name: RankCategory :exec
UPDATE race_results r
SET rank_category = s.rnk, updated_at = now()
FROM (
    SELECT rr.id, RANK() OVER (
        PARTITION BY rr.category_id
        ORDER BY COALESCE(rr.chip_time_ms, rr.gun_time_ms) ASC
    ) AS rnk
    FROM race_results rr
    WHERE rr.event_id = $1 AND rr.status = 'FINISHED' AND rr.category_id IS NOT NULL
      AND COALESCE(rr.chip_time_ms, rr.gun_time_ms) IS NOT NULL
) s
WHERE r.id = s.id;

-- name: RankAgeGroup :exec
UPDATE race_results r
SET rank_age_group = s.rnk, updated_at = now()
FROM (
    SELECT rr.id, RANK() OVER (
        PARTITION BY rr.age_group
        ORDER BY COALESCE(rr.chip_time_ms, rr.gun_time_ms) ASC
    ) AS rnk
    FROM race_results rr
    WHERE rr.event_id = $1 AND rr.status = 'FINISHED' AND rr.age_group IS NOT NULL
      AND COALESCE(rr.chip_time_ms, rr.gun_time_ms) IS NOT NULL
) s
WHERE r.id = s.id;

-- --- certificate templates ---

-- name: CreateCertificateTemplate :one
INSERT INTO certificate_templates (
    organization_id, event_id, name, title, subtitle, body_template, background_url, is_active
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateCertificateTemplate :one
UPDATE certificate_templates
SET name = $3, title = $4, subtitle = $5, body_template = $6,
    background_url = $7, is_active = $8, updated_at = now()
WHERE id = $1 AND organization_id = $2
RETURNING *;

-- name: GetActiveCertificateTemplate :one
SELECT * FROM certificate_templates
WHERE event_id = $1 AND is_active = true;

-- name: ListCertificateTemplatesByEvent :many
SELECT * FROM certificate_templates
WHERE event_id = $1
ORDER BY created_at DESC;

-- name: GetCertificateTemplateByID :one
SELECT * FROM certificate_templates WHERE id = $1;

-- DeactivateCertificateTemplatesForEvent clears the active flag on every template
-- of an event so a newly-activated one can claim the partial-unique active slot.
-- name: DeactivateCertificateTemplatesForEvent :exec
UPDATE certificate_templates
SET is_active = false, updated_at = now()
WHERE event_id = $1 AND is_active = true;

-- name: DeleteCertificateTemplate :exec
DELETE FROM certificate_templates WHERE id = $1 AND organization_id = $2;
