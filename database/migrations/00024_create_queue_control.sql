-- +goose Up
CREATE TABLE queue_control (
    event_id             uuid PRIMARY KEY REFERENCES events(id) ON DELETE CASCADE,
    state                text NOT NULL DEFAULT 'RUNNING',
    release_rate         integer NOT NULL DEFAULT 100,
    randomization_seed   text,
    sale_start_at        timestamptz,
    presale_pool_open_at timestamptz,
    updated_at           timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT qc_state_check CHECK (state IN ('RUNNING','PAUSED')),
    CONSTRAINT qc_rate_check CHECK (release_rate >= 0)
);

-- +goose Down
DROP TABLE queue_control;
