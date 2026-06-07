-- +goose Up
INSERT INTO permissions (key, description)
VALUES ('payment.manage', 'Reconcile/manage payments in org')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.key = 'payment.manage'
WHERE r.organization_id IS NULL AND r.is_system = true AND r.slug IN ('owner','finance')
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions
WHERE permission_id = (SELECT id FROM permissions WHERE key = 'payment.manage');
DELETE FROM permissions WHERE key = 'payment.manage';
