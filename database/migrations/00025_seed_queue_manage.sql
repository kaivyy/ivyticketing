-- +goose Up
INSERT INTO permissions (key, description) VALUES
    ('queue.manage', 'Pause/resume queue & set release rate')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.key = 'queue.manage'
WHERE r.organization_id IS NULL AND r.is_system = true AND r.slug IN ('owner','manager')
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions
WHERE permission_id = (SELECT id FROM permissions WHERE key = 'queue.manage');
DELETE FROM permissions WHERE key = 'queue.manage';
