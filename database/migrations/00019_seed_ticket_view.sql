-- +goose Up
INSERT INTO permissions (key, description)
VALUES ('ticket.view', 'View tickets in org/event')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.key = 'ticket.view'
WHERE r.organization_id IS NULL AND r.is_system = true AND r.slug IN ('owner','manager','customer-service')
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions
WHERE permission_id = (SELECT id FROM permissions WHERE key = 'ticket.view');
DELETE FROM permissions WHERE key = 'ticket.view';
