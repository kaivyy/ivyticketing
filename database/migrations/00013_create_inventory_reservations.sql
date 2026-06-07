-- +goose Up
CREATE TABLE inventory_reservations (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id     uuid NOT NULL REFERENCES event_categories(id) ON DELETE RESTRICT,
    order_id        uuid NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    participant_id  uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status          text NOT NULL DEFAULT 'ACTIVE',
    expires_at      timestamptz NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT reservations_status_check CHECK (status IN ('ACTIVE','EXPIRED','COMPLETED','RELEASED'))
);
CREATE INDEX idx_reservations_category ON inventory_reservations(category_id);
CREATE INDEX idx_reservations_status ON inventory_reservations(status);
CREATE INDEX idx_reservations_category_status ON inventory_reservations(category_id, status);

-- +goose Down
DROP TABLE inventory_reservations;
