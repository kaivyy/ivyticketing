-- +goose Up
CREATE TABLE queue_tokens (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    participant_id  uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status          text NOT NULL DEFAULT 'WAITING',
    pool            text NOT NULL DEFAULT 'FIFO',
    score           bigint NOT NULL,
    joined_at       timestamptz NOT NULL DEFAULT now(),
    allowed_at      timestamptz,
    expired_at      timestamptz,
    completed_at    timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT qt_status_check CHECK (status IN ('WAITING','ALLOWED','EXPIRED','COMPLETED','BLOCKED')),
    CONSTRAINT qt_pool_check CHECK (pool IN ('PRESALE','FIFO')),
    CONSTRAINT qt_event_participant_unique UNIQUE (event_id, participant_id)
);
CREATE INDEX idx_queue_tokens_event_status ON queue_tokens(event_id, status);
CREATE INDEX idx_queue_tokens_event_pool_score ON queue_tokens(event_id, pool, score);

-- +goose Down
DROP TABLE queue_tokens;
