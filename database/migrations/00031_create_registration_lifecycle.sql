-- +goose Up
CREATE TABLE registration_lifecycles (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id             uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id          uuid NOT NULL REFERENCES event_categories(id) ON DELETE CASCADE,
    status               text NOT NULL DEFAULT 'DRAFT'
                             CHECK (status IN ('DRAFT','ACTIVE','PAUSED','COMPLETED','CANCELLED')),
    current_phase_index  integer NOT NULL DEFAULT 0,
    created_by           uuid NOT NULL REFERENCES users(id),
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX registration_lifecycles_active_idx
    ON registration_lifecycles(event_id, category_id)
    WHERE status NOT IN ('COMPLETED','CANCELLED');

-- +goose Down
DROP TABLE registration_lifecycles;
