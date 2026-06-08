-- +goose Up
CREATE TABLE waitlists (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id         uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id                uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id             uuid NOT NULL REFERENCES event_categories(id) ON DELETE CASCADE,
    pool_id                 uuid,
    mode                    text NOT NULL DEFAULT 'FIFO'
                                CHECK (mode IN ('FIFO','RANDOMIZED','HYBRID')),
    status                  text NOT NULL DEFAULT 'ACTIVE'
                                CHECK (status IN ('ACTIVE','PAUSED','CLOSED')),
    max_promotion_batch     integer NOT NULL DEFAULT 10,
    promotion_window_hours  integer NOT NULL DEFAULT 48,
    auto_promote            boolean NOT NULL DEFAULT true,
    seed                    text,
    created_at              timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX waitlists_category_idx ON waitlists(event_id, category_id, status);

-- +goose Down
DROP TABLE waitlists;
