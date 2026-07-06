-- +goose Up
-- Phase 14.1 — Racepack hardening: slot_id linkage + idempotency keys.

-- Add slot_id to pickup records so we can report on slot throughput.
ALTER TABLE racepack_pickup_records
    ADD COLUMN slot_id uuid REFERENCES racepack_pickup_slots(id) ON DELETE SET NULL;

CREATE INDEX idx_racepack_pickup_records_slot ON racepack_pickup_records(slot_id);
CREATE INDEX idx_racepack_pickup_records_event_status ON racepack_pickup_records(event_id, status);

-- Idempotency key storage. Same key + same payload → cached response.
-- Same key + different payload → 409 IDEMPOTENCY_CONFLICT.
CREATE TABLE idempotency_keys (
    key             text NOT NULL,
    scope           text NOT NULL,
    request_hash    text NOT NULL,
    response_status int  NOT NULL,
    response_body   jsonb NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (key, scope)
);
CREATE INDEX idx_idempotency_keys_created_at ON idempotency_keys(created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_idempotency_keys_created_at;
DROP TABLE IF EXISTS idempotency_keys;

DROP INDEX IF EXISTS idx_racepack_pickup_records_event_status;
DROP INDEX IF EXISTS idx_racepack_pickup_records_slot;

ALTER TABLE racepack_pickup_records DROP COLUMN IF EXISTS slot_id;