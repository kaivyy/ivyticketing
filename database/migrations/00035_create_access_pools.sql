-- +goose Up
CREATE TABLE access_pools (
    id                          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id             uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id                    uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id                 uuid NOT NULL REFERENCES event_categories(id) ON DELETE CASCADE,
    pool_type                   text NOT NULL
                                    CHECK (pool_type IN ('RESERVED','COMMUNITY','CORPORATE',
                                           'SPONSOR','VIP','PARTNER','PRIORITY','ELITE')),
    name                        text NOT NULL,
    total_slots                 integer NOT NULL CHECK (total_slots > 0),
    reserved_slots              integer NOT NULL DEFAULT 0,
    used_slots                  integer NOT NULL DEFAULT 0,
    released_slots              integer NOT NULL DEFAULT 0,
    owner_account_id            uuid,
    is_visible_to_participants  boolean NOT NULL DEFAULT false,
    eligibility_rule            jsonb,
    valid_from                  timestamptz,
    valid_until                 timestamptz,
    created_by                  uuid NOT NULL REFERENCES users(id),
    created_at                  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT access_pools_slots_check
        CHECK (reserved_slots + used_slots <= total_slots),
    CONSTRAINT access_pools_non_negative
        CHECK (reserved_slots >= 0 AND used_slots >= 0 AND released_slots >= 0)
);
CREATE INDEX access_pools_category_idx ON access_pools(event_id, category_id, pool_type);

-- +goose Down
DROP TABLE access_pools;
