-- +goose Up
INSERT INTO permissions (key, description) VALUES
    ('order.create', 'Create orders on behalf of participants'),
    ('order.manage', 'Manage and cancel any order in the organization')
ON CONFLICT (key) DO NOTHING;

-- grant to template Owner & Manager roles (organization_id IS NULL)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.key IN ('order.create','order.manage')
WHERE r.organization_id IS NULL AND r.slug IN ('owner','manager')
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE key IN ('order.create','order.manage'));
DELETE FROM permissions WHERE key IN ('order.create','order.manage');
