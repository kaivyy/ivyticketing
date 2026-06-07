-- +goose Up
CREATE TABLE schema_health (
    id          SMALLINT PRIMARY KEY DEFAULT 1,
    checked_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT schema_health_singleton CHECK (id = 1)
);
INSERT INTO schema_health (id) VALUES (1);

-- +goose Down
DROP TABLE schema_health;
