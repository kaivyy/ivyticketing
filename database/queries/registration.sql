-- name: GetEventRegistrationSettings :one
SELECT * FROM event_registration_settings WHERE event_id = $1;

-- name: UpsertEventRegistrationSettings :one
INSERT INTO event_registration_settings (event_id, default_mode, queue_enabled, ballot_enabled, priority_enabled, waitlist_enabled)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (event_id) DO UPDATE SET
    default_mode = EXCLUDED.default_mode,
    queue_enabled = EXCLUDED.queue_enabled,
    ballot_enabled = EXCLUDED.ballot_enabled,
    priority_enabled = EXCLUDED.priority_enabled,
    waitlist_enabled = EXCLUDED.waitlist_enabled,
    updated_at = now()
RETURNING *;

-- name: GetCategoryRegistrationSettings :one
SELECT * FROM category_registration_settings WHERE category_id = $1;

-- name: UpsertCategoryRegistrationSettings :one
INSERT INTO category_registration_settings (category_id, registration_mode, override_enabled)
VALUES ($1,$2,$3)
ON CONFLICT (category_id) DO UPDATE SET
    registration_mode = EXCLUDED.registration_mode,
    override_enabled = EXCLUDED.override_enabled,
    updated_at = now()
RETURNING *;

-- name: ListCategoryRegistrationSettingsByEvent :many
SELECT crs.* FROM category_registration_settings crs
JOIN event_categories ec ON ec.id = crs.category_id
WHERE ec.event_id = $1;
