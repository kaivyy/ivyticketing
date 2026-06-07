-- +goose Up
INSERT INTO permissions (key, description) VALUES
    ('member.manage',       'Manage organization staff and their roles'),
    ('role.manage',         'Create and edit custom roles'),
    ('organization.manage', 'Edit organization settings'),
    ('event.create',        'Create events'),
    ('event.edit',          'Edit events'),
    ('event.publish',       'Publish events'),
    ('event.delete',        'Delete events'),
    ('category.manage',     'Manage event categories'),
    ('form.manage',         'Manage registration forms'),
    ('order.view',          'View orders'),
    ('order.refund',        'Refund orders'),
    ('payment.view',        'View payments'),
    ('payment.refund',      'Refund payments'),
    ('participant.view',    'View participants'),
    ('participant.export',  'Export participant data'),
    ('coupon.manage',       'Manage coupons'),
    ('bib.manage',          'Manage BIB numbers'),
    ('racepack.scan',       'Scan racepack pickups'),
    ('racepack.manage',     'Manage racepack pickup config'),
    ('report.view',         'View reports'),
    ('broadcast.send',      'Send broadcasts')
ON CONFLICT (key) DO NOTHING;

INSERT INTO roles (organization_id, name, slug, is_system) VALUES
    (NULL, 'Owner',            'owner',            true),
    (NULL, 'Manager',          'manager',          true),
    (NULL, 'Finance',          'finance',          true),
    (NULL, 'Customer Service', 'customer-service', true),
    (NULL, 'Racepack Staff',   'racepack-staff',   true)
ON CONFLICT DO NOTHING;

-- Owner: all permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.organization_id IS NULL AND r.slug = 'owner'
ON CONFLICT DO NOTHING;

-- Manager
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.key IN (
    'event.create','event.edit','event.publish','event.delete',
    'category.manage','form.manage','participant.view','order.view',
    'report.view','broadcast.send','coupon.manage','bib.manage')
WHERE r.organization_id IS NULL AND r.slug = 'manager'
ON CONFLICT DO NOTHING;

-- Finance
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.key IN (
    'order.view','order.refund','payment.view','payment.refund','report.view')
WHERE r.organization_id IS NULL AND r.slug = 'finance'
ON CONFLICT DO NOTHING;

-- Customer Service
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.key IN (
    'participant.view','order.view')
WHERE r.organization_id IS NULL AND r.slug = 'customer-service'
ON CONFLICT DO NOTHING;

-- Racepack Staff
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.key IN (
    'racepack.scan','racepack.manage','participant.view')
WHERE r.organization_id IS NULL AND r.slug = 'racepack-staff'
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions WHERE role_id IN (SELECT id FROM roles WHERE organization_id IS NULL);
DELETE FROM roles WHERE organization_id IS NULL;
DELETE FROM permissions;
