-- +goose Up
CREATE TABLE tickets (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id     uuid NOT NULL REFERENCES event_categories(id) ON DELETE RESTRICT,
    order_id        uuid NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    participant_id  uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    ticket_number   text NOT NULL UNIQUE,
    status          text NOT NULL DEFAULT 'VALID',
    holder_name     text NOT NULL,
    holder_email    text NOT NULL,
    event_title     text NOT NULL,
    category_name   text NOT NULL,
    qr_version      int NOT NULL DEFAULT 1,
    issued_at       timestamptz NOT NULL DEFAULT now(),
    used_at         timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tickets_status_check CHECK (status IN ('VALID','USED','CANCELLED')),
    CONSTRAINT tickets_order_unique UNIQUE (order_id)
);
CREATE INDEX idx_tickets_participant ON tickets(participant_id);
CREATE INDEX idx_tickets_event ON tickets(event_id);
CREATE INDEX idx_tickets_status ON tickets(status);

-- +goose Down
DROP TABLE tickets;
