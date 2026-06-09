-- +goose Up
CREATE TABLE ballot_draws (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id         uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id                uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id             uuid NOT NULL REFERENCES event_categories(id) ON DELETE CASCADE,
    status                  text NOT NULL DEFAULT 'PENDING'
                                CHECK (status IN ('PENDING','OPEN','CLOSED','DRAWN','ANNOUNCED')),
    quota                   integer NOT NULL CHECK (quota > 0),
    waitlist_size           integer,
    payment_window_hours    integer NOT NULL DEFAULT 48,
    application_opens_at    timestamptz,
    application_closes_at   timestamptz,
    draw_at                 timestamptz,
    announced_at            timestamptz,
    seed                    text,
    draw_nonce              uuid,
    winner_pool_id          uuid REFERENCES access_pools(id),
    waitlist_id             uuid REFERENCES waitlists(id),
    created_by              uuid NOT NULL REFERENCES users(id),
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ballot_draws_dates_check CHECK (application_opens_at < application_closes_at)
);
CREATE INDEX ballot_draws_category_idx ON ballot_draws(event_id, category_id, status);

-- +goose Down
DROP TABLE ballot_draws;
