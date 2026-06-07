-- name: CreateCategory :one
INSERT INTO event_categories (organization_id, event_id, name, price, capacity,
    registration_opens_at, registration_closes_at, bib_prefix, min_age, max_order_per_user)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetCategoryByID :one
SELECT * FROM event_categories WHERE id = $1;

-- name: ListCategoriesByEvent :many
SELECT * FROM event_categories WHERE event_id = $1 ORDER BY created_at;

-- name: UpdateCategory :one
UPDATE event_categories SET
    name = $2, price = $3, capacity = $4,
    registration_opens_at = $5, registration_closes_at = $6,
    bib_prefix = $7, min_age = $8, max_order_per_user = $9, updated_at = now()
WHERE id = $1 AND event_id = $10
RETURNING *;

-- name: DeleteCategory :exec
DELETE FROM event_categories WHERE id = $1 AND event_id = $2;

-- name: ListCategoriesByEventForPublic :many
SELECT * FROM event_categories WHERE event_id = $1 ORDER BY price;
