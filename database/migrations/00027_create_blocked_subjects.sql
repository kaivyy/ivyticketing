-- +goose Up
CREATE TABLE blocked_subjects (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_type  text NOT NULL,
    subject_value text NOT NULL,
    reason        text,
    blocked_by    uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz,
    CONSTRAINT bs_type_check CHECK (subject_type IN ('user','ip')),
    CONSTRAINT bs_unique UNIQUE (subject_type, subject_value)
);
CREATE INDEX idx_blocked_subjects_lookup ON blocked_subjects(subject_type, subject_value);

-- +goose Down
DROP TABLE blocked_subjects;
