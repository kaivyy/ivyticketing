-- +goose Up
ALTER TABLE notifications ADD COLUMN next_retry_at TIMESTAMPTZ;
ALTER TABLE notifications ADD COLUMN last_error TEXT;

CREATE INDEX idx_notifications_retry ON notifications(status, next_retry_at)
  WHERE status IN ('pending','failed');

-- +goose Down
DROP INDEX IF EXISTS idx_notifications_retry;
ALTER TABLE notifications DROP COLUMN IF EXISTS last_error;
ALTER TABLE notifications DROP COLUMN IF EXISTS next_retry_at;
