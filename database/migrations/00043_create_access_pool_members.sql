-- +goose Up
CREATE TABLE access_pool_members (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    pool_id             uuid NOT NULL REFERENCES access_pools(id) ON DELETE CASCADE,
    user_id             uuid REFERENCES users(id),
    email               text NOT NULL,
    member_status       text NOT NULL DEFAULT 'PENDING'
                            CHECK (member_status IN ('PENDING','ACTIVE','REGISTERED','EXPIRED','REVOKED')),
    eligibility_meta    jsonb,
    access_grant_id     uuid REFERENCES access_grants(id),
    invited_at          timestamptz NOT NULL DEFAULT now(),
    registered_at       timestamptz,
    revoked_at          timestamptz,
    UNIQUE (pool_id, email)
);
CREATE INDEX access_pool_members_pool_status_idx ON access_pool_members(pool_id, member_status);
CREATE INDEX access_pool_members_user_idx ON access_pool_members(user_id) WHERE user_id IS NOT NULL;

-- +goose Down
DROP TABLE access_pool_members;
