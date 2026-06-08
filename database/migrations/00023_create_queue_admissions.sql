-- +goose Up
CREATE TABLE queue_admissions (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_id           uuid NOT NULL REFERENCES queue_tokens(id) ON DELETE CASCADE,
    event_id           uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    participant_id     uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    checkout_expires_at timestamptz NOT NULL,
    status             text NOT NULL DEFAULT 'ACTIVE',
    created_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT qa_status_check CHECK (status IN ('ACTIVE','CONSUMED','EXPIRED'))
);
CREATE INDEX idx_queue_admissions_expiry ON queue_admissions(event_id, status, checkout_expires_at);
CREATE UNIQUE INDEX uq_admission_active ON queue_admissions(token_id) WHERE status = 'ACTIVE';

-- +goose Down
DROP TABLE queue_admissions;
