-- Multi-Sport Generic Competition Queries

-- 1. Sports & Disciplines
-- name: ListSports :many
SELECT * FROM sports ORDER BY name ASC;

-- name: GetSportByID :one
SELECT * FROM sports WHERE id = $1;

-- name: ListSportDisciplinesBySport :many
SELECT * FROM sport_disciplines WHERE sport_id = $1 ORDER BY name ASC;

-- name: GetSportDisciplineByID :one
SELECT * FROM sport_disciplines WHERE id = $1;

-- 2. Competition Configs
-- name: UpsertCompetitionConfig :one
INSERT INTO competition_configs (
    organization_id, event_id, category_id, sport_id, discipline_id,
    has_timing, has_scoring, has_matches, has_heats_lanes, has_stages, team_based,
    min_team_size, max_team_size, ranking_strategy, time_precision, rules_config
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10, $11,
    $12, $13, $14, $15, $16
)
ON CONFLICT (event_id, category_id) DO UPDATE SET
    sport_id = EXCLUDED.sport_id,
    discipline_id = EXCLUDED.discipline_id,
    has_timing = EXCLUDED.has_timing,
    has_scoring = EXCLUDED.has_scoring,
    has_matches = EXCLUDED.has_matches,
    has_heats_lanes = EXCLUDED.has_heats_lanes,
    has_stages = EXCLUDED.has_stages,
    team_based = EXCLUDED.team_based,
    min_team_size = EXCLUDED.min_team_size,
    max_team_size = EXCLUDED.max_team_size,
    ranking_strategy = EXCLUDED.ranking_strategy,
    time_precision = EXCLUDED.time_precision,
    rules_config = EXCLUDED.rules_config,
    updated_at = now()
RETURNING *;

-- name: GetCompetitionConfigByEvent :one
SELECT * FROM competition_configs
WHERE event_id = $1 AND category_id IS NULL;

-- name: GetCompetitionConfigByCategory :one
SELECT * FROM competition_configs
WHERE event_id = $1 AND category_id = $2;

-- 3. Teams & Rosters
-- name: CreateTeam :one
INSERT INTO teams (
    organization_id, event_id, category_id, name, short_name,
    manager_user_id, logo_url, seed_number, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetTeamByID :one
SELECT * FROM teams WHERE id = $1;

-- name: ListTeamsByEvent :many
SELECT * FROM teams
WHERE event_id = $1
ORDER BY seed_number ASC NULLS LAST, name ASC;

-- name: UpdateTeamStatus :one
UPDATE teams
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateTeamRosterMember :one
INSERT INTO team_rosters (
    team_id, ticket_id, user_id, player_name, jersey_number, position, is_captain, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListTeamRosterByTeam :many
SELECT * FROM team_rosters
WHERE team_id = $1
ORDER BY is_captain DESC, jersey_number ASC NULLS LAST, player_name ASC;

-- name: DeleteTeamRosterMember :exec
DELETE FROM team_rosters WHERE id = $1;

-- 4. Stages & Matches
-- name: CreateCompetitionStage :one
INSERT INTO competition_stages (
    organization_id, event_id, category_id, name, stage_type, sequence_order, status, stage_config
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetCompetitionStageByID :one
SELECT * FROM competition_stages WHERE id = $1;

-- name: ListCompetitionStagesByEvent :many
SELECT * FROM competition_stages
WHERE event_id = $1
ORDER BY sequence_order ASC, name ASC;

-- name: UpdateCompetitionStageStatus :one
UPDATE competition_stages
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateCompetitionMatch :one
INSERT INTO competition_matches (
    organization_id, event_id, stage_id, round_number, match_number,
    home_team_id, away_team_id, home_entry_id, away_entry_id,
    scheduled_start_time, venue_court_name, home_score, away_score,
    match_status, winner_id, score_details
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING *;

-- name: GetCompetitionMatchByID :one
SELECT * FROM competition_matches WHERE id = $1;

-- name: ListCompetitionMatchesByStage :many
SELECT * FROM competition_matches
WHERE stage_id = $1
ORDER BY round_number ASC, match_number ASC;

-- name: UpdateCompetitionMatchScore :one
UPDATE competition_matches
SET home_score = $2,
    away_score = $3,
    match_status = $4,
    winner_id = $5,
    score_details = $6,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateCompetitionMatchScoreScoped :one
UPDATE competition_matches cm
SET home_score = $2,
    away_score = $3,
    match_status = $4,
    winner_id = $5,
    score_details = $6,
    updated_at = now()
FROM competition_stages cs
JOIN events e ON e.id = cs.event_id
WHERE cm.id = $1
  AND cm.stage_id = cs.id
  AND cs.event_id = $7
  AND e.organization_id = $8
RETURNING cm.*;

-- name: GetCompetitionMatchScoped :one
SELECT cm.*
FROM competition_matches cm
JOIN competition_stages cs ON cs.id = cm.stage_id
JOIN events e ON e.id = cs.event_id
WHERE cm.id = $1
  AND cs.event_id = $2
  AND e.organization_id = $3;

-- name: UpdateCompetitionMatchParticipants :exec
UPDATE competition_matches
SET home_entry_id = COALESCE($2, home_entry_id),
    away_entry_id = COALESCE($3, away_entry_id),
    updated_at = now()
WHERE stage_id = $1 AND round_number = $4 AND match_number = $5;

-- name: GetCompetitionMatchByRoundAndNumber :one
SELECT * FROM competition_matches
WHERE stage_id = $1 AND round_number = $2 AND match_number = $3;

-- name: UpdateCompetitionMatchParticipantsScheduled :execrows
UPDATE competition_matches
SET home_entry_id = COALESCE($2, home_entry_id),
    away_entry_id = COALESCE($3, away_entry_id),
    updated_at = now()
WHERE stage_id = $1 AND round_number = $4 AND match_number = $5 AND match_status = 'SCHEDULED';

-- 5. Participant Entries
-- name: CreateParticipantEntry :one
INSERT INTO participant_entries (
    organization_id, event_id, category_id, ticket_id, team_id,
    entry_type, display_name, identifier_code, seed_number, lane_number, heat_number, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetParticipantEntryByID :one
SELECT * FROM participant_entries WHERE id = $1;

-- name: GetParticipantEntryByTicket :one
SELECT * FROM participant_entries WHERE ticket_id = $1;

-- name: ListParticipantEntriesByEvent :many
SELECT * FROM participant_entries
WHERE event_id = $1
ORDER BY heat_number ASC NULLS LAST, lane_number ASC NULLS LAST, seed_number ASC NULLS LAST, display_name ASC;

-- name: UpdateParticipantEntryStatus :one
UPDATE participant_entries
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- 6. Observations
-- name: CreateCompetitionObservation :one
INSERT INTO competition_observations (
    organization_id, event_id, stage_id, match_id, entry_id,
    observation_type, source, numeric_value, text_value, observed_at_ms, metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListObservationsByMatch :many
SELECT * FROM competition_observations
WHERE match_id = $1
ORDER BY observed_at_ms ASC, created_at ASC;

-- name: ListObservationsByEntry :many
SELECT * FROM competition_observations
WHERE entry_id = $1
ORDER BY observed_at_ms ASC, created_at ASC;

-- name: ListObservationsByStage :many
SELECT * FROM competition_observations
WHERE stage_id = $1
ORDER BY observed_at_ms ASC, created_at ASC;

-- 7. Competition Results
-- name: UpsertCompetitionResult :one
INSERT INTO competition_results (
    organization_id, event_id, stage_id, category_id, entry_id,
    rank_overall, rank_category, status, primary_time_ms, secondary_time_ms,
    points_scored, metrics, notes
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13
)
ON CONFLICT (stage_id, entry_id) DO UPDATE SET
    category_id = EXCLUDED.category_id,
    rank_overall = EXCLUDED.rank_overall,
    rank_category = EXCLUDED.rank_category,
    status = EXCLUDED.status,
    primary_time_ms = EXCLUDED.primary_time_ms,
    secondary_time_ms = EXCLUDED.secondary_time_ms,
    points_scored = EXCLUDED.points_scored,
    metrics = EXCLUDED.metrics,
    notes = EXCLUDED.notes,
    updated_at = now()
RETURNING *;

-- name: GetCompetitionResultByEntry :one
SELECT * FROM competition_results
WHERE stage_id = $1 AND entry_id = $2;

-- name: ListCompetitionResultsByStage :many
SELECT cr.*, pe.display_name, pe.identifier_code, pe.entry_type, pe.team_id, pe.lane_number, pe.heat_number
FROM competition_results cr
JOIN participant_entries pe ON pe.id = cr.entry_id
WHERE cr.stage_id = $1
ORDER BY cr.rank_overall ASC NULLS LAST, cr.primary_time_ms ASC NULLS LAST, cr.points_scored DESC, pe.display_name ASC;

-- name: ListCompetitionResultsByEvent :many
SELECT cr.*, pe.display_name, pe.identifier_code, pe.entry_type, pe.team_id
FROM competition_results cr
JOIN participant_entries pe ON pe.id = cr.entry_id
WHERE cr.event_id = $1
ORDER BY cr.rank_overall ASC NULLS LAST, cr.points_scored DESC, cr.primary_time_ms ASC NULLS LAST;

-- name: RankCompetitionStageByTime :exec
UPDATE competition_results r
SET rank_overall = s.rnk, updated_at = now()
FROM (
    SELECT cr.id, RANK() OVER (
        ORDER BY COALESCE(cr.primary_time_ms, cr.secondary_time_ms) ASC, cr.secondary_time_ms ASC NULLS LAST
    ) AS rnk
    FROM competition_results cr
    WHERE cr.stage_id = $1 AND cr.status = 'FINISHED'
      AND COALESCE(cr.primary_time_ms, cr.secondary_time_ms) IS NOT NULL
) s
WHERE r.id = s.id;

-- name: RankCompetitionStageByPoints :exec
UPDATE competition_results r
SET rank_overall = s.rnk, updated_at = now()
FROM (
    SELECT cr.id, RANK() OVER (
        ORDER BY cr.points_scored DESC, (cr.metrics->>'goal_difference')::numeric DESC NULLS LAST, (cr.metrics->>'goals_for')::numeric DESC NULLS LAST
    ) AS rnk
    FROM competition_results cr
    WHERE cr.stage_id = $1 AND cr.status = 'FINISHED'
) s
WHERE r.id = s.id;
