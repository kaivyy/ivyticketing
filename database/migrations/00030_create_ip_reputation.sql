-- +goose Up
CREATE TABLE ip_reputation (
    subject_type  text NOT NULL,
    subject_value text NOT NULL,
    score         integer NOT NULL DEFAULT 0,
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (subject_type, subject_value),
    CONSTRAINT rep_type_check CHECK (subject_type IN ('ip','user'))
);

-- +goose Down
DROP TABLE ip_reputation;
