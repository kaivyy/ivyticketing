-- name: CreateAuditLog :exec
INSERT INTO audit_logs (organization_id, actor_user_id, action, target_type, target_id, metadata)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListAuditLogsByOrg :many
SELECT * FROM audit_logs WHERE organization_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: ListAuditLogsWithActorByOrg :many
SELECT
    al.id,
    al.organization_id,
    al.actor_user_id,
    al.action,
    al.target_type,
    al.target_id,
    al.metadata,
    al.created_at,
    COALESCE(u.email, '')::citext AS actor_email,
    COALESCE(u.full_name, '')::text AS actor_name
FROM audit_logs al
LEFT JOIN users u ON u.id = al.actor_user_id
WHERE al.organization_id = $1
ORDER BY al.created_at DESC
LIMIT $2;
