DROP INDEX IF EXISTS idx_idempotency_keys_created_at;
DROP TABLE IF EXISTS idempotency_keys;

DROP INDEX IF EXISTS idx_racepack_pickup_records_event_status;
DROP INDEX IF EXISTS idx_racepack_pickup_records_slot;

ALTER TABLE racepack_pickup_records DROP COLUMN IF EXISTS slot_id;