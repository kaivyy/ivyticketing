-- +goose Up
CREATE TABLE payment_webhooks (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    gateway              text NOT NULL,
    event_type           text,
    merchant_reference   text,
    gateway_reference    text,
    signature            text,
    signature_valid      boolean NOT NULL DEFAULT false,
    payload              jsonb NOT NULL,
    dedupe_key           text,
    processing_status    text NOT NULL DEFAULT 'RECEIVED',
    processed_payment_id uuid REFERENCES payments(id),
    error_detail         text,
    received_at          timestamptz NOT NULL DEFAULT now(),
    processed_at         timestamptz,
    CONSTRAINT payment_webhooks_gateway_check CHECK (gateway IN ('duitku','xendit')),
    CONSTRAINT payment_webhooks_status_check CHECK (processing_status IN ('RECEIVED','PROCESSED','REJECTED','DUPLICATE','FAILED'))
);
CREATE UNIQUE INDEX uq_payment_webhooks_dedupe ON payment_webhooks(dedupe_key) WHERE dedupe_key IS NOT NULL;
CREATE INDEX idx_payment_webhooks_ref ON payment_webhooks(merchant_reference);
CREATE INDEX idx_payment_webhooks_status ON payment_webhooks(processing_status);

-- +goose Down
DROP TABLE payment_webhooks;
