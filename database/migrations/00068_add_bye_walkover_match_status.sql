-- +goose Up
-- 1. Update competition_matches match_status check constraint to support first-class WALKOVER and BYE states
ALTER TABLE competition_matches
    DROP CONSTRAINT IF EXISTS competition_matches_match_status_check;

ALTER TABLE competition_matches
    ADD CONSTRAINT competition_matches_match_status_check
    CHECK (match_status IN ('SCHEDULED', 'LIVE', 'COMPLETED', 'SUSPENDED', 'CANCELLED', 'FORFEIT', 'WALKOVER', 'BYE'));

-- 2. Backfill historical bye matches that were recorded as COMPLETED with a single participant
UPDATE competition_matches
SET match_status = 'BYE'
WHERE match_status = 'COMPLETED'
  AND home_entry_id IS NOT NULL
  AND away_entry_id IS NULL;

-- +goose Down
-- Revert BYE to COMPLETED and WALKOVER to FORFEIT before restoring strict constraint
UPDATE competition_matches
SET match_status = 'COMPLETED'
WHERE match_status = 'BYE';

UPDATE competition_matches
SET match_status = 'FORFEIT'
WHERE match_status = 'WALKOVER';

ALTER TABLE competition_matches
    DROP CONSTRAINT IF EXISTS competition_matches_match_status_check;

ALTER TABLE competition_matches
    ADD CONSTRAINT competition_matches_match_status_check
    CHECK (match_status IN ('SCHEDULED', 'LIVE', 'COMPLETED', 'SUSPENDED', 'CANCELLED', 'FORFEIT'));
