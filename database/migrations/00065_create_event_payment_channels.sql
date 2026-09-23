-- +goose Up
CREATE TABLE IF NOT EXISTS event_payment_channels (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    channel_code    text NOT NULL,
    is_enabled      boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (event_id, channel_code)
);
CREATE INDEX IF NOT EXISTS idx_event_payment_channels_event ON event_payment_channels(event_id);

-- +goose Down
DROP TABLE IF EXISTS event_payment_channels;
