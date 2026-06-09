-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS membership_id text;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS membership_id;
