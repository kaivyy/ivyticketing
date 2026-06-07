-- name: CreateEvent :one
INSERT INTO events (organization_id, name, slug, description, event_type,
    venue_name, venue_address, starts_at, ends_at, faq, terms, waiver)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetEventByID :one
SELECT * FROM events WHERE id = $1;

-- name: GetEventByOrgAndSlug :one
SELECT * FROM events WHERE organization_id = $1 AND slug = $2;

-- name: ListEventsByOrg :many
SELECT * FROM events WHERE organization_id = $1 ORDER BY created_at DESC;

-- name: UpdateEvent :one
UPDATE events SET
    name = $2, description = $3, event_type = $4,
    venue_name = $5, venue_address = $6, starts_at = $7, ends_at = $8,
    faq = $9, terms = $10, waiver = $11, updated_at = now()
WHERE id = $1 AND organization_id = $12
RETURNING *;

-- name: UpdateEventStatus :one
UPDATE events SET status = $2, published_at = $3, updated_at = now()
WHERE id = $1 AND organization_id = $4
RETURNING *;

-- name: SetEventMediaKey :one
UPDATE events SET banner_object_key = COALESCE($2, banner_object_key),
    logo_object_key = COALESCE($3, logo_object_key), updated_at = now()
WHERE id = $1 AND organization_id = $4
RETURNING *;

-- name: DeleteEvent :exec
DELETE FROM events WHERE id = $1 AND organization_id = $2;

-- name: CountCategoriesForEvent :one
SELECT count(*) FROM event_categories WHERE event_id = $1;

-- name: ListPublishedEventsByOrgSlug :many
SELECT e.* FROM events e
JOIN organizations o ON o.id = e.organization_id
WHERE o.slug = $1 AND e.status = 'published'
ORDER BY e.starts_at NULLS LAST, e.created_at DESC;

-- name: GetPublishedEventByOrgAndSlug :one
SELECT e.* FROM events e
JOIN organizations o ON o.id = e.organization_id
WHERE o.slug = $1 AND e.slug = $2 AND e.status = 'published';
