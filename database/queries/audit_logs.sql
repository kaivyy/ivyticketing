-- name: CreateAuditLog :exec
INSERT INTO audit_logs (organization_id, actor_user_id, action, target_type, target_id, metadata)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListAuditLogsByOrg :many
SELECT * FROM audit_logs WHERE organization_id = $1 ORDER BY created_at DESC LIMIT $2;
