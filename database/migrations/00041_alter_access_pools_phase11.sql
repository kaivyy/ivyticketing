-- +goose Up
ALTER TABLE access_pools
    ADD COLUMN IF NOT EXISTS owner_account_id            uuid,
    ADD COLUMN IF NOT EXISTS is_visible_to_participants  boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS eligibility_rule            jsonb;

CREATE INDEX IF NOT EXISTS access_pools_owner_idx ON access_pools(owner_account_id)
    WHERE owner_account_id IS NOT NULL;

-- +goose Down
ALTER TABLE access_pools
    DROP COLUMN IF EXISTS owner_account_id,
    DROP COLUMN IF EXISTS is_visible_to_participants,
    DROP COLUMN IF EXISTS eligibility_rule;
DROP INDEX IF EXISTS access_pools_owner_idx;
