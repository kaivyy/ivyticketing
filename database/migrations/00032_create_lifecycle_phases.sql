-- +goose Up
CREATE TABLE lifecycle_phases (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    lifecycle_id      uuid NOT NULL REFERENCES registration_lifecycles(id) ON DELETE CASCADE,
    phase_index       integer NOT NULL,
    registration_mode text NOT NULL,
    label             text NOT NULL,
    opens_at          timestamptz,
    closes_at         timestamptz,
    capacity_override integer CHECK (capacity_override > 0),
    auto_advance      boolean NOT NULL DEFAULT true,
    status            text NOT NULL DEFAULT 'PENDING'
                          CHECK (status IN ('PENDING','ACTIVE','COMPLETED','SKIPPED')),
    activated_at      timestamptz,
    completed_at      timestamptz,
    UNIQUE (lifecycle_id, phase_index)
);
CREATE INDEX lifecycle_phases_auto_advance_idx
    ON lifecycle_phases(status, closes_at)
    WHERE status = 'ACTIVE' AND auto_advance = true;

-- +goose Down
DROP TABLE lifecycle_phases;
