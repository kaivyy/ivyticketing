-- +goose Up
-- 1. Index on competition_observations(stage_id) to optimize ListObservationsByStage
CREATE INDEX IF NOT EXISTS idx_competition_obs_stage ON competition_observations(stage_id);

-- 2. Unique index on participant_entries(event_id, ticket_id) WHERE ticket_id IS NOT NULL
-- Prevents duplicate competition entries for the same ticket within an event while allowing non-ticket entries
CREATE UNIQUE INDEX IF NOT EXISTS idx_participant_entries_event_ticket
ON participant_entries(event_id, ticket_id)
WHERE ticket_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_participant_entries_event_ticket;
DROP INDEX IF EXISTS idx_competition_obs_stage;
