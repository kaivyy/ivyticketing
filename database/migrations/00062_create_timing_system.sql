-- +goose Up
-- Phase 25: RFID Race Timing, Waves, Splits, and Timing Provider Integration

-- 1. timing_configs: per-event integration configuration (RACE_RESULT, CSV, NATIVE)
CREATE TABLE timing_configs (
    id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id        uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id               uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    provider               text NOT NULL DEFAULT 'RACE_RESULT'
        CHECK (provider IN ('RACE_RESULT', 'CSV', 'NATIVE')),
    ingestion_token_hash   text NOT NULL,
    ingestion_token_prefix text NOT NULL,
    encrypted_api_key      text,
    external_race_id       text,
    is_active              boolean NOT NULL DEFAULT true,
    settings               jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),
    UNIQUE (event_id)
);
CREATE INDEX idx_timing_configs_event ON timing_configs(event_id);

-- 2. race_waves: wave / start corrals with official gun flag-off times
CREATE TABLE race_waves (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id         uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id      uuid REFERENCES event_categories(id) ON DELETE SET NULL,
    code             text NOT NULL,
    name             text NOT NULL,
    start_at         timestamptz,
    order_index      integer NOT NULL DEFAULT 0,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (event_id, code)
);
CREATE INDEX idx_race_waves_event ON race_waves(event_id, order_index);

-- Link existing tickets and race_results to race_waves without duplicating participant data
ALTER TABLE tickets
    ADD COLUMN wave_id uuid REFERENCES race_waves(id) ON DELETE SET NULL;
CREATE INDEX idx_tickets_wave ON tickets(wave_id) WHERE wave_id IS NOT NULL;

ALTER TABLE race_results
    ADD COLUMN wave_id uuid REFERENCES race_waves(id) ON DELETE SET NULL;
CREATE INDEX idx_race_results_wave ON race_results(wave_id) WHERE wave_id IS NOT NULL;

-- 3. timing_checkpoints: physical detection locations (START, splits, FINISH)
CREATE TABLE timing_checkpoints (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id         uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    code             text NOT NULL,
    name             text NOT NULL,
    checkpoint_type  text NOT NULL DEFAULT 'SPLIT'
        CHECK (checkpoint_type IN ('START', 'SPLIT', 'FINISH')),
    order_index      integer NOT NULL DEFAULT 0,
    distance_meters  integer,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (event_id, code)
);
CREATE INDEX idx_timing_checkpoints_event ON timing_checkpoints(event_id, order_index);

-- 4. bib_transponder_mappings: decoupled BIB to RFID chip/transponder mapping
-- Ensures tickets.bib_number remains the single source of truth for participant BIB.
CREATE TABLE bib_transponder_mappings (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id         uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    bib_number       text NOT NULL,
    transponder_code text NOT NULL,
    is_active        boolean NOT NULL DEFAULT true,
    status           text NOT NULL DEFAULT 'ACTIVE'
        CHECK (status IN ('ACTIVE', 'REPLACED', 'REVOKED')),
    notes            text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

-- Exactly one active mapping per BIB within an event
CREATE UNIQUE INDEX uniq_bib_transponder_active_bib
    ON bib_transponder_mappings (event_id, bib_number)
    WHERE is_active;

-- Exactly one active mapping per transponder within an event
CREATE UNIQUE INDEX uniq_bib_transponder_active_chip
    ON bib_transponder_mappings (event_id, transponder_code)
    WHERE is_active;

CREATE INDEX idx_bib_transponder_lookup
    ON bib_transponder_mappings (event_id, transponder_code);

-- 5. timing_passings: raw timing telemetry buffer (idempotent ingestion)
CREATE TABLE timing_passings (
    id                bigserial PRIMARY KEY,
    organization_id   uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id          uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    checkpoint_code   text NOT NULL,
    provider          text NOT NULL,
    external_read_id  text,
    chip_code         text NOT NULL,
    bib_number        text,
    observed_at       timestamptz NOT NULL,
    received_at       timestamptz NOT NULL DEFAULT now(),
    raw_payload       text,
    metadata          jsonb NOT NULL DEFAULT '{}'::jsonb,
    processed         boolean NOT NULL DEFAULT false,
    processed_at      timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now()
);

-- Primary idempotency: deduplicate on provider's external passing ID (e.g. PassingNo)
CREATE UNIQUE INDEX uniq_timing_passings_external
    ON timing_passings (event_id, provider, external_read_id)
    WHERE external_read_id IS NOT NULL;

-- Fallback idempotency for providers without external read ID
CREATE UNIQUE INDEX uniq_timing_passings_fallback
    ON timing_passings (event_id, provider, chip_code, checkpoint_code, observed_at)
    WHERE external_read_id IS NULL;

CREATE INDEX idx_timing_passings_unprocessed
    ON timing_passings (event_id, processed)
    WHERE NOT processed;

-- 6. race_split_times: calculated intermediate split times
CREATE TABLE race_split_times (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    race_result_id    uuid NOT NULL REFERENCES race_results(id) ON DELETE CASCADE,
    checkpoint_id     uuid NOT NULL REFERENCES timing_checkpoints(id) ON DELETE CASCADE,
    passing_id        bigint REFERENCES timing_passings(id) ON DELETE SET NULL,
    split_time_ms     bigint NOT NULL CHECK (split_time_ms >= 0),
    split_pace_ms_km  bigint,
    passing_time      timestamptz NOT NULL,
    order_index       integer NOT NULL DEFAULT 0,
    created_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (race_result_id, checkpoint_id)
);
CREATE INDEX idx_race_split_times_result ON race_split_times(race_result_id, order_index);

-- +goose Down
DROP TABLE IF EXISTS race_split_times;
DROP TABLE IF EXISTS timing_passings;
DROP TABLE IF EXISTS bib_transponder_mappings;
DROP TABLE IF EXISTS timing_checkpoints;
ALTER TABLE race_results DROP COLUMN IF EXISTS wave_id;
ALTER TABLE tickets DROP COLUMN IF EXISTS wave_id;
DROP TABLE IF EXISTS race_waves;
DROP TABLE IF EXISTS timing_configs;
