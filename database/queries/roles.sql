-- name: ListPermissions :many
SELECT * FROM permissions ORDER BY key;

-- name: GetPermissionByKey :one
SELECT * FROM permissions WHERE key = $1;

-- name: ListTemplateRoles :many
SELECT * FROM roles WHERE organization_id IS NULL ORDER BY name;

-- name: ListPermissionsForRole :many
SELECT p.* FROM permissions p
JOIN role_permissions rp ON rp.permission_id = p.id
WHERE rp.role_id = $1
ORDER BY p.key;

-- name: CreateRole :one
INSERT INTO roles (organization_id, name, slug, is_system)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetRoleByID :one
SELECT * FROM roles WHERE id = $1;

-- name: GetRoleByOrgAndSlug :one
SELECT * FROM roles WHERE organization_id = $1 AND slug = $2;

-- name: ListRolesByOrg :many
SELECT * FROM roles WHERE organization_id = $1 ORDER BY name;

-- name: UpdateRoleName :one
UPDATE roles SET name = $2 WHERE id = $1 AND organization_id = $3
RETURNING *;

-- name: DeleteRole :exec
DELETE FROM roles WHERE id = $1 AND organization_id = $2 AND is_system = false;

-- name: AddRolePermission :exec
INSERT INTO role_permissions (role_id, permission_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: ClearRolePermissions :exec
DELETE FROM role_permissions WHERE role_id = $1;

-- name: CountMembersWithRole :one
SELECT count(*) FROM member_roles WHERE role_id = $1;
