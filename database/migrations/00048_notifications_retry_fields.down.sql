DROP INDEX IF EXISTS idx_notifications_retry;
ALTER TABLE notifications DROP COLUMN IF EXISTS last_error;
ALTER TABLE notifications DROP COLUMN IF EXISTS next_retry_at;
