-- +goose Up
-- Phase 15 — RBAC extension for scanner check-in

INSERT INTO permissions (key, description) VALUES
    ('checkin.execute', 'Check a participant in at event entry (VALID -> USED)')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.organization_id IS NULL
  AND r.slug IN ('racepack-staff', 'manager')
  AND p.key = 'checkin.execute'
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions WHERE permission_id IN
    (SELECT id FROM permissions WHERE key = 'checkin.execute');
DELETE FROM permissions WHERE key = 'checkin.execute';
