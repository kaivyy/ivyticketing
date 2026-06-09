-- +goose Up
CREATE TABLE access_codes (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id         uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id      uuid REFERENCES event_categories(id) ON DELETE CASCADE,
    code_type        text NOT NULL
                         CHECK (code_type IN ('INVITATION','PRIORITY','COMMUNITY','CORPORATE',
                                             'COUPON','PARTNER','SPONSOR','VIP','ELITE')),
    code_value_hash  text NOT NULL,
    is_single_use    boolean NOT NULL DEFAULT true,
    max_uses         integer NOT NULL DEFAULT 1 CHECK (max_uses > 0),
    use_count        integer NOT NULL DEFAULT 0,
    valid_from       timestamptz NOT NULL,
    valid_until      timestamptz NOT NULL,
    pool_id          uuid REFERENCES access_pools(id),
    eligibility_rule jsonb,
    created_by       uuid NOT NULL REFERENCES users(id),
    created_at       timestamptz NOT NULL DEFAULT now(),
    metadata         jsonb,
    UNIQUE (event_id, code_value_hash),
    CONSTRAINT access_codes_dates_check CHECK (valid_from < valid_until),
    CONSTRAINT access_codes_use_count_check CHECK (use_count <= max_uses)
);
CREATE INDEX access_codes_event_type_idx ON access_codes(event_id, code_type, valid_until);
CREATE INDEX access_codes_active_idx ON access_codes(event_id, code_value_hash);

-- +goose Down
DROP TABLE access_codes;
