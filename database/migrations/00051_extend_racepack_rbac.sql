-- +goose Up
-- Phase 14 — RBAC extension for racepack

INSERT INTO permissions (key, description) VALUES
    ('racepack.execute',     'Execute a pickup (confirm or proxy) at a counter'),
    ('racepack.problemdesk', 'Open and resolve problem desk cases')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.organization_id IS NULL
  AND r.slug = 'racepack-staff'
  AND p.key IN ('racepack.execute', 'racepack.problemdesk')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.organization_id IS NULL
  AND r.slug = 'manager'
  AND p.key IN ('racepack.manage', 'racepack.execute', 'racepack.problemdesk')
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions WHERE permission_id IN
    (SELECT id FROM permissions WHERE key IN ('racepack.execute','racepack.problemdesk'));
DELETE FROM permissions WHERE key IN ('racepack.execute','racepack.problemdesk');