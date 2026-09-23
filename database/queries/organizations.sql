-- name: CreateOrganization :one
INSERT INTO organizations (name, slug)
VALUES ($1, $2)
RETURNING *;

-- name: GetOrganizationByID :one
SELECT * FROM organizations WHERE id = $1;

-- name: GetOrganizationBySlug :one
SELECT * FROM organizations WHERE slug = $1;

-- name: ListOrganizationsForUser :many
SELECT o.* FROM organizations o
JOIN organization_members m ON m.organization_id = o.id
WHERE m.user_id = $1
ORDER BY o.created_at;

-- name: ListAllOrganizationsWithStats :many
SELECT 
    o.id,
    o.name,
    o.slug,
    o.created_at,
    (SELECT count(*) FROM events e WHERE e.organization_id = o.id)::bigint AS event_count,
    (SELECT count(*) FROM organization_members m WHERE m.organization_id = o.id)::bigint AS member_count,
    COALESCE((
        SELECT u.email FROM organization_members om 
        JOIN users u ON u.id = om.user_id 
        WHERE om.organization_id = o.id 
        ORDER BY om.created_at ASC LIMIT 1
    ), '')::text AS lead_email,
    COALESCE((
        SELECT u.full_name FROM organization_members om 
        JOIN users u ON u.id = om.user_id 
        WHERE om.organization_id = o.id 
        ORDER BY om.created_at ASC LIMIT 1
    ), '')::text AS lead_name
FROM organizations o
ORDER BY o.created_at DESC;

