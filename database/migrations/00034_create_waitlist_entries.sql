-- +goose Up
CREATE TABLE waitlist_entries (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    waitlist_id             uuid NOT NULL REFERENCES waitlists(id) ON DELETE CASCADE,
    participant_id          uuid NOT NULL REFERENCES users(id),
    event_id                uuid NOT NULL REFERENCES events(id),
    category_id             uuid NOT NULL REFERENCES event_categories(id),
    source                  text NOT NULL CHECK (source IN ('BALLOT','QUOTA_RELEASE','MANUAL')),
    source_ref_id           uuid,
    status                  text NOT NULL DEFAULT 'WAITING'
                                CHECK (status IN ('WAITING','PROMOTED','EXPIRED','WITHDRAWN')),
    rank                    bigint NOT NULL,
    notified_at             timestamptz,
    promoted_at             timestamptz,
    access_grant_id         uuid,
    promotion_window_hours  integer,
    created_at              timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX waitlist_entries_active_idx
    ON waitlist_entries(waitlist_id, participant_id)
    WHERE status NOT IN ('WITHDRAWN','EXPIRED');
CREATE INDEX waitlist_entries_rank_idx ON waitlist_entries(waitlist_id, status, rank)
    WHERE status = 'WAITING';

-- +goose Down
DROP TABLE waitlist_entries;
