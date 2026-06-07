-- name: CreateMember :one
INSERT INTO organization_members (organization_id, user_id)
VALUES ($1, $2)
RETURNING *;

-- name: GetMemberByOrgAndUser :one
SELECT * FROM organization_members
WHERE organization_id = $1 AND user_id = $2;

-- name: GetMemberByID :one
SELECT * FROM organization_members WHERE id = $1;

-- name: ListMembersByOrg :many
SELECT m.id, m.user_id, m.created_at, u.email, u.full_name
FROM organization_members m
JOIN users u ON u.id = m.user_id
WHERE m.organization_id = $1
ORDER BY m.created_at;

-- name: DeleteMember :exec
DELETE FROM organization_members WHERE id = $1 AND organization_id = $2;

-- name: AddMemberRole :exec
INSERT INTO member_roles (organization_member_id, role_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: ClearMemberRoles :exec
DELETE FROM member_roles WHERE organization_member_id = $1;

-- name: ListRolesForMember :many
SELECT r.* FROM roles r
JOIN member_roles mr ON mr.role_id = r.id
WHERE mr.organization_member_id = $1
ORDER BY r.name;

-- name: ListPermissionsForMember :many
SELECT DISTINCT p.key
FROM member_roles mr
JOIN role_permissions rp ON rp.role_id = mr.role_id
JOIN permissions p ON p.id = rp.permission_id
WHERE mr.organization_member_id = $1;

-- name: CountOwnersInOrg :one
SELECT count(DISTINCT mr.organization_member_id)
FROM member_roles mr
JOIN roles r ON r.id = mr.role_id
WHERE r.organization_id = $1 AND r.slug = 'owner';

-- name: MemberHasRoleSlug :one
SELECT EXISTS (
    SELECT 1 FROM member_roles mr
    JOIN roles r ON r.id = mr.role_id
    WHERE mr.organization_member_id = $1 AND r.slug = $2
);
