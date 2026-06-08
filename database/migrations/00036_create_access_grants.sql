-- +goose Up
CREATE TABLE access_grants (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    pool_id         uuid REFERENCES access_pools(id),
    participant_id  uuid NOT NULL REFERENCES users(id),
    event_id        uuid NOT NULL REFERENCES events(id),
    category_id     uuid NOT NULL REFERENCES event_categories(id),
    code_id         uuid,
    status          text NOT NULL DEFAULT 'ACTIVE'
                        CHECK (status IN ('ACTIVE','CONSUMED','EXPIRED')),
    granted_at      timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NOT NULL,
    consumed_at     timestamptz,
    order_id        uuid,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX access_grants_participant_idx ON access_grants(participant_id, event_id, status);
CREATE INDEX access_grants_expiry_idx ON access_grants(expires_at)
    WHERE status = 'ACTIVE';

-- +goose Down
DROP TABLE access_grants;
