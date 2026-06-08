-- +goose Up
CREATE TABLE platform_settings (
    key        text PRIMARY KEY,
    value      text NOT NULL,
    updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO platform_settings (key, value) VALUES
    ('turnstile_enabled', 'false'),
    ('rate_limit_enabled', 'true'),
    ('ip_reputation_enabled', 'true'),
    ('blocklist_enabled', 'true');

-- +goose Down
DROP TABLE platform_settings;
