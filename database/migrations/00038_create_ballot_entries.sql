-- +goose Up
CREATE TABLE ballot_entries (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    draw_id           uuid NOT NULL REFERENCES ballot_draws(id) ON DELETE CASCADE,
    organization_id   uuid NOT NULL REFERENCES organizations(id),
    event_id          uuid NOT NULL REFERENCES events(id),
    category_id       uuid NOT NULL REFERENCES event_categories(id),
    participant_id    uuid NOT NULL REFERENCES users(id),
    status            text NOT NULL DEFAULT 'APPLIED'
                          CHECK (status IN ('APPLIED','WINNER','WAITLISTED','NOT_SELECTED',
                                           'LAPSED','CONVERTED','WITHDRAWN')),
    applied_at        timestamptz NOT NULL DEFAULT now(),
    payment_deadline  timestamptz,
    converted_at      timestamptz,
    promoted_round    integer NOT NULL DEFAULT 0,
    access_grant_id   uuid REFERENCES access_grants(id),
    UNIQUE (draw_id, participant_id)
);
CREATE INDEX ballot_entries_draw_status_idx ON ballot_entries(draw_id, status);
CREATE INDEX ballot_entries_participant_idx ON ballot_entries(participant_id, event_id, status);

-- +goose Down
DROP TABLE ballot_entries;
