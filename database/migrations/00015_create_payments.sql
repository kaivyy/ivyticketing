-- +goose Up
CREATE TABLE payments (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id            uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    order_id            uuid NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    participant_id      uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    gateway             text NOT NULL,
    method              text NOT NULL,
    channel             text,
    status              text NOT NULL DEFAULT 'PENDING',
    amount              bigint NOT NULL,
    currency            text NOT NULL DEFAULT 'IDR',
    gateway_reference   text,
    merchant_reference  text NOT NULL,
    pay_url             text,
    qr_string           text,
    va_number           text,
    instructions        jsonb,
    expires_at          timestamptz,
    paid_at             timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT payments_gateway_check CHECK (gateway IN ('duitku','xendit')),
    CONSTRAINT payments_method_check CHECK (method IN ('qris','va','ewallet')),
    CONSTRAINT payments_status_check CHECK (status IN ('PENDING','PAID','EXPIRED','FAILED')),
    CONSTRAINT payments_amount_check CHECK (amount >= 0),
    CONSTRAINT payments_merchant_ref_unique UNIQUE (merchant_reference)
);
CREATE INDEX idx_payments_order ON payments(order_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_gateway_ref ON payments(gateway, gateway_reference);
CREATE UNIQUE INDEX uq_payments_order_active ON payments(order_id) WHERE status IN ('PENDING','PAID');

-- +goose Down
DROP TABLE payments;
