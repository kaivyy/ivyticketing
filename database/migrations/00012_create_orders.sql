-- +goose Up
CREATE TABLE orders (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id     uuid NOT NULL REFERENCES event_categories(id) ON DELETE RESTRICT,
    participant_id  uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    order_number    text NOT NULL UNIQUE,
    status          text NOT NULL DEFAULT 'DRAFT',
    subtotal        bigint NOT NULL,
    fee             bigint NOT NULL DEFAULT 0,
    discount        bigint NOT NULL DEFAULT 0,
    total           bigint NOT NULL,
    expired_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT orders_status_check CHECK (status IN ('DRAFT','PENDING_PAYMENT','PAID','EXPIRED','CANCELLED','REFUNDED')),
    CONSTRAINT orders_amounts_check CHECK (subtotal >= 0 AND fee >= 0 AND discount >= 0 AND total >= 0)
);
CREATE INDEX idx_orders_org_event ON orders(organization_id, event_id);
CREATE INDEX idx_orders_participant ON orders(participant_id);
CREATE INDEX idx_orders_status_expired ON orders(status, expired_at);
CREATE INDEX idx_orders_category ON orders(category_id);

-- +goose Down
DROP TABLE orders;
