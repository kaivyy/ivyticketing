-- +goose Up
INSERT INTO permissions (key, description) VALUES
    ('ballot.manage', 'Create and manage ballot draws'),
    ('ballot.apply',  'Apply to ballot draws')
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM permissions WHERE key IN ('ballot.manage', 'ballot.apply');
