-- +goose Up
CREATE TABLE abuse_log (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_type  text,
    subject_value text,
    action        text NOT NULL,
    category      text,
    fingerprint   text,
    ip            text,
    user_id       uuid,
    detail        jsonb,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_abuse_log_created ON abuse_log(created_at DESC);
CREATE INDEX idx_abuse_log_subject ON abuse_log(subject_type, subject_value);

-- +goose Down
DROP TABLE abuse_log;
