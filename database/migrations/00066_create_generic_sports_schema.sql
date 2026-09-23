-- +goose Up
-- Migration 00066: Generic Multi-Sport Competition Engine Schema

-- 1. Master Sports Catalog
CREATE TABLE sports (
    id             text PRIMARY KEY,
    name           text NOT NULL,
    governing_body text NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now()
);

-- 2. Sport Disciplines
CREATE TABLE sport_disciplines (
    id             text PRIMARY KEY,
    sport_id       text NOT NULL REFERENCES sports(id) ON DELETE CASCADE,
    name           text NOT NULL,
    default_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_sport_disciplines_sport ON sport_disciplines(sport_id);

-- 3. Extend Events table to associate with sport and discipline
ALTER TABLE events
    ADD COLUMN IF NOT EXISTS sport_id text REFERENCES sports(id),
    ADD COLUMN IF NOT EXISTS discipline_id text REFERENCES sport_disciplines(id);

-- 4. Extend race_results status check to allow PENDING_REVIEW
ALTER TABLE race_results DROP CONSTRAINT IF EXISTS race_results_status_check;
ALTER TABLE race_results ADD CONSTRAINT race_results_status_check
    CHECK (status IN ('FINISHED', 'DNF', 'DNS', 'DSQ', 'OTL', 'PENDING_REVIEW'));

-- 5. Competition Configurations (Capabilities & Rules per Event or Category)
CREATE TABLE competition_configs (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id         uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id      uuid REFERENCES event_categories(id) ON DELETE CASCADE,
    sport_id         text NOT NULL REFERENCES sports(id),
    discipline_id    text NOT NULL REFERENCES sport_disciplines(id),
    has_timing       boolean NOT NULL DEFAULT false,
    has_scoring      boolean NOT NULL DEFAULT false,
    has_matches      boolean NOT NULL DEFAULT false,
    has_heats_lanes  boolean NOT NULL DEFAULT false,
    has_stages       boolean NOT NULL DEFAULT false,
    team_based       boolean NOT NULL DEFAULT false,
    min_team_size    integer NOT NULL DEFAULT 1,
    max_team_size    integer NOT NULL DEFAULT 1,
    ranking_strategy text NOT NULL DEFAULT 'TIME_ASC',
    time_precision   text NOT NULL DEFAULT 's',
    rules_config     jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (event_id, category_id)
);
CREATE INDEX idx_competition_configs_event ON competition_configs(event_id);
CREATE INDEX idx_competition_configs_org ON competition_configs(organization_id);

-- 6. Teams (for squad, relay, doubles, and club competitions)
CREATE TABLE teams (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id     uuid REFERENCES event_categories(id) ON DELETE SET NULL,
    name            text NOT NULL,
    short_name      text,
    manager_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    logo_url        text,
    seed_number     integer,
    status          text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'DISQUALIFIED', 'WITHDRAWN')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_teams_event ON teams(event_id);
CREATE INDEX idx_teams_org ON teams(organization_id);

-- 7. Team Rosters (linking athletes / tickets to teams)
CREATE TABLE team_rosters (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id       uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    ticket_id     uuid REFERENCES tickets(id) ON DELETE SET NULL,
    user_id       uuid REFERENCES users(id) ON DELETE SET NULL,
    player_name   text NOT NULL,
    jersey_number integer,
    position      text,
    is_captain    boolean NOT NULL DEFAULT false,
    status        text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'SUBSTITUTE', 'INJURED', 'SUSPENDED')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_team_rosters_team ON team_rosters(team_id);
CREATE INDEX idx_team_rosters_ticket ON team_rosters(ticket_id);

-- 8. Competition Stages (Group Stage, Heats, Knockout Brackets, Peloton Stages)
CREATE TABLE competition_stages (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id     uuid REFERENCES event_categories(id) ON DELETE CASCADE,
    name            text NOT NULL,
    stage_type      text NOT NULL CHECK (stage_type IN ('single_race', 'group_round_robin', 'single_elimination', 'double_elimination', 'heats', 'peloton_stage')),
    sequence_order  integer NOT NULL DEFAULT 1,
    status          text NOT NULL DEFAULT 'SCHEDULED' CHECK (status IN ('SCHEDULED', 'LIVE', 'COMPLETED', 'ABANDONED')),
    stage_config    jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_competition_stages_event ON competition_stages(event_id);

-- 9. Competition Matches (Fixtures, Courts, and 1v1 / Team Matches)
CREATE TABLE competition_matches (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id      uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id             uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    stage_id             uuid NOT NULL REFERENCES competition_stages(id) ON DELETE CASCADE,
    round_number         integer NOT NULL DEFAULT 1,
    match_number         integer NOT NULL DEFAULT 1,
    home_team_id         uuid REFERENCES teams(id) ON DELETE SET NULL,
    away_team_id         uuid REFERENCES teams(id) ON DELETE SET NULL,
    home_entry_id        uuid,
    away_entry_id        uuid,
    scheduled_start_time timestamptz,
    venue_court_name     text,
    home_score           integer NOT NULL DEFAULT 0,
    away_score           integer NOT NULL DEFAULT 0,
    match_status         text NOT NULL DEFAULT 'SCHEDULED' CHECK (match_status IN ('SCHEDULED', 'LIVE', 'COMPLETED', 'SUSPENDED', 'CANCELLED', 'FORFEIT')),
    winner_id            uuid,
    score_details        jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_competition_matches_stage ON competition_matches(stage_id);
CREATE INDEX idx_competition_matches_event ON competition_matches(event_id);

-- 10. Participant Entries (Unified Identity bridging individual tickets or teams to a competition)
CREATE TABLE participant_entries (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id     uuid REFERENCES event_categories(id) ON DELETE CASCADE,
    ticket_id       uuid REFERENCES tickets(id) ON DELETE SET NULL,
    team_id         uuid REFERENCES teams(id) ON DELETE SET NULL,
    entry_type      text NOT NULL DEFAULT 'INDIVIDUAL' CHECK (entry_type IN ('INDIVIDUAL', 'TEAM', 'PAIR', 'RELAY', 'SQUAD')),
    display_name    text NOT NULL,
    identifier_code text,
    seed_number     integer,
    lane_number     integer,
    heat_number     integer,
    status          text NOT NULL DEFAULT 'REGISTERED' CHECK (status IN ('REGISTERED', 'CHECKED_IN', 'ACTIVE', 'COMPLETED', 'WITHDRAWN', 'DISQUALIFIED', 'DNS', 'DNF', 'PENDING_REVIEW')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_participant_entries_event ON participant_entries(event_id);
CREATE INDEX idx_participant_entries_ticket ON participant_entries(ticket_id);
CREATE INDEX idx_participant_entries_team ON participant_entries(team_id);

-- 11. Competition Observations (Immutable log of events: splits, goals, points, cards, fouls)
CREATE TABLE competition_observations (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id         uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    stage_id         uuid REFERENCES competition_stages(id) ON DELETE CASCADE,
    match_id         uuid REFERENCES competition_matches(id) ON DELETE CASCADE,
    entry_id         uuid REFERENCES participant_entries(id) ON DELETE CASCADE,
    observation_type text NOT NULL,
    source           text NOT NULL DEFAULT 'MANUAL',
    numeric_value    numeric(12, 4),
    text_value       text,
    observed_at_ms   bigint NOT NULL,
    metadata         jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_competition_obs_event ON competition_observations(event_id);
CREATE INDEX idx_competition_obs_match ON competition_observations(match_id);
CREATE INDEX idx_competition_obs_entry ON competition_observations(entry_id);

-- 12. Competition Results (Unified standings, times, points, rankings)
CREATE TABLE competition_results (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id          uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    stage_id          uuid REFERENCES competition_stages(id) ON DELETE CASCADE,
    category_id       uuid REFERENCES event_categories(id) ON DELETE CASCADE,
    entry_id          uuid NOT NULL REFERENCES participant_entries(id) ON DELETE CASCADE,
    rank_overall      integer,
    rank_category     integer,
    status            text NOT NULL DEFAULT 'FINISHED' CHECK (status IN (
        'REGISTERED', 'CHECKED_IN', 'STARTED', 'ACTIVE', 'COMPLETED', 'FINISHED', 
        'DNF', 'DNS', 'DSQ', 'OTL', 'WITHDRAWN', 'RETIRED', 'NO_SHOW', 
        'ELIMINATED', 'QUALIFIED', 'DISQUALIFIED', 'FORFEIT', 'WALKOVER', 'PENDING_REVIEW'
    )),
    primary_time_ms   bigint,
    secondary_time_ms bigint,
    points_scored     numeric(10, 2) NOT NULL DEFAULT 0,
    metrics           jsonb NOT NULL DEFAULT '{}'::jsonb,
    notes             text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (stage_id, entry_id)
);
CREATE INDEX idx_competition_results_event ON competition_results(event_id);
CREATE INDEX idx_competition_results_stage ON competition_results(stage_id);
CREATE INDEX idx_competition_results_entry ON competition_results(entry_id);

-- 13. Idempotent Master Seeds for Sports & Disciplines
INSERT INTO sports (id, name, governing_body) VALUES
    ('running', 'Running / Athletics', 'World Athletics'),
    ('cycling', 'Cycling', 'UCI'),
    ('swimming', 'Swimming / Aquatics', 'World Aquatics'),
    ('football', 'Football / Soccer', 'FIFA / IFAB'),
    ('badminton', 'Badminton', 'BWF')
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    governing_body = EXCLUDED.governing_body;

INSERT INTO sport_disciplines (id, sport_id, name, default_config) VALUES
    ('road_running', 'running', 'Road Running', '{"has_timing": true, "has_scoring": false, "time_precision": "ceil_s", "ranking_strategy": "TIME_ASC"}'::jsonb),
    ('trail_running', 'running', 'Trail Running', '{"has_timing": true, "has_scoring": false, "time_precision": "ceil_s", "ranking_strategy": "TIME_ASC"}'::jsonb),
    ('track_running', 'running', 'Track Running', '{"has_timing": true, "has_scoring": false, "time_precision": "cs", "has_heats_lanes": true, "ranking_strategy": "TIME_ASC"}'::jsonb),
    ('road_cycling', 'cycling', 'Road Cycling', '{"has_timing": true, "has_scoring": true, "has_stages": true, "ranking_strategy": "TIME_ASC"}'::jsonb),
    ('mtb_cycling', 'cycling', 'Mountain Bike (MTB)', '{"has_timing": true, "has_scoring": false, "ranking_strategy": "TIME_ASC"}'::jsonb),
    ('pool_swimming', 'swimming', 'Pool Swimming', '{"has_timing": true, "has_scoring": false, "has_heats_lanes": true, "time_precision": "cs", "ranking_strategy": "TIME_ASC"}'::jsonb),
    ('open_water_swimming', 'swimming', 'Open Water Swimming', '{"has_timing": true, "has_scoring": false, "time_precision": "s", "ranking_strategy": "TIME_ASC"}'::jsonb),
    ('football_11v11', 'football', 'Association Football (11v11)', '{"has_timing": false, "has_scoring": true, "has_matches": true, "team_based": true, "min_team_size": 11, "max_team_size": 25, "ranking_strategy": "POINTS_DESC"}'::jsonb),
    ('futsal', 'football', 'Futsal (5v5)', '{"has_timing": false, "has_scoring": true, "has_matches": true, "team_based": true, "min_team_size": 5, "max_team_size": 14, "ranking_strategy": "POINTS_DESC"}'::jsonb),
    ('badminton_singles', 'badminton', 'Badminton Singles', '{"has_timing": false, "has_scoring": true, "has_matches": true, "team_based": false, "ranking_strategy": "WIN_COUNT"}'::jsonb),
    ('badminton_doubles', 'badminton', 'Badminton Doubles', '{"has_timing": false, "has_scoring": true, "has_matches": true, "team_based": true, "min_team_size": 2, "max_team_size": 2, "ranking_strategy": "WIN_COUNT"}'::jsonb),
    ('badminton_mixed_doubles', 'badminton', 'Badminton Mixed Doubles', '{"has_timing": false, "has_scoring": true, "has_matches": true, "team_based": true, "min_team_size": 2, "max_team_size": 2, "ranking_strategy": "WIN_COUNT"}'::jsonb)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    default_config = EXCLUDED.default_config;

-- Backfill existing running events to have sport_id = 'running' and discipline_id = 'road_running' if null
UPDATE events
SET sport_id = 'running', discipline_id = 'road_running'
WHERE sport_id IS NULL;

-- +goose Down
DROP TABLE IF EXISTS competition_results;
DROP TABLE IF EXISTS competition_observations;
DROP TABLE IF EXISTS participant_entries;
DROP TABLE IF EXISTS competition_matches;
DROP TABLE IF EXISTS competition_stages;
DROP TABLE IF EXISTS team_rosters;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS competition_configs;
ALTER TABLE events DROP COLUMN IF EXISTS discipline_id;
ALTER TABLE events DROP COLUMN IF EXISTS sport_id;
DROP TABLE IF EXISTS sport_disciplines;
DROP TABLE IF EXISTS sports;
