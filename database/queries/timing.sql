-- Timing Configs

-- name: UpsertTimingConfig :one
INSERT INTO timing_configs (
    organization_id, event_id, provider, transport, sync_mode, policy,
    ingestion_token_hash, ingestion_token_prefix, encrypted_api_key,
    external_race_id, is_active, settings
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9,
    $10, $11, $12
)
ON CONFLICT (event_id) DO UPDATE SET
    provider               = EXCLUDED.provider,
    transport              = EXCLUDED.transport,
    sync_mode              = EXCLUDED.sync_mode,
    policy                 = EXCLUDED.policy,
    ingestion_token_hash   = EXCLUDED.ingestion_token_hash,
    ingestion_token_prefix = EXCLUDED.ingestion_token_prefix,
    encrypted_api_key      = EXCLUDED.encrypted_api_key,
    external_race_id       = EXCLUDED.external_race_id,
    is_active              = EXCLUDED.is_active,
    settings               = EXCLUDED.settings,
    updated_at             = now()
RETURNING *;

-- name: GetTimingConfigByEvent :one
SELECT * FROM timing_configs WHERE event_id = $1;

-- name: GetTimingConfigByTokenHash :one
SELECT * FROM timing_configs
WHERE event_id = $1 AND ingestion_token_hash = $2 AND is_active = true;

-- Race Waves

-- name: CreateRaceWave :one
INSERT INTO race_waves (
    organization_id, event_id, category_id, code, name, start_at, order_index
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateRaceWave :one
UPDATE race_waves
SET name = $3, start_at = $4, order_index = $5, updated_at = now()
WHERE id = $1 AND event_id = $2
RETURNING *;

-- name: ListRaceWavesByEvent :many
SELECT * FROM race_waves
WHERE event_id = $1
ORDER BY order_index ASC, created_at ASC;

-- name: GetRaceWaveByID :one
SELECT * FROM race_waves WHERE id = $1;

-- name: DeleteRaceWave :exec
DELETE FROM race_waves WHERE id = $1 AND event_id = $2;

-- Timing Checkpoints

-- name: UpsertTimingCheckpoint :one
INSERT INTO timing_checkpoints (
    organization_id, event_id, code, name, checkpoint_type, order_index, distance_meters,
    aliases, provider_aliases
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (event_id, code) DO UPDATE SET
    name             = EXCLUDED.name,
    checkpoint_type  = EXCLUDED.checkpoint_type,
    order_index      = EXCLUDED.order_index,
    distance_meters  = EXCLUDED.distance_meters,
    aliases          = EXCLUDED.aliases,
    provider_aliases = EXCLUDED.provider_aliases
RETURNING *;

-- name: ListTimingCheckpointsByEvent :many
SELECT * FROM timing_checkpoints
WHERE event_id = $1
ORDER BY order_index ASC;

-- name: GetTimingCheckpointByCode :one
SELECT * FROM timing_checkpoints
WHERE event_id = $1 AND code = $2;

-- name: DeleteTimingCheckpoint :exec
DELETE FROM timing_checkpoints WHERE id = $1 AND event_id = $2;

-- BIB Transponder Mappings

-- name: InsertBibTransponderMapping :one
INSERT INTO bib_transponder_mappings (
    organization_id, event_id, bib_number, transponder_code, is_active, status, notes
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: DeactivateBibMappings :exec
UPDATE bib_transponder_mappings
SET is_active = false, status = 'REPLACED', updated_at = now()
WHERE event_id = $1 AND bib_number = $2 AND is_active = true;

-- name: DeactivateChipMappings :exec
UPDATE bib_transponder_mappings
SET is_active = false, status = 'REPLACED', updated_at = now()
WHERE event_id = $1 AND transponder_code = $2 AND is_active = true;

-- name: GetActiveMappingByChip :one
SELECT * FROM bib_transponder_mappings
WHERE event_id = $1 AND transponder_code = $2 AND is_active = true;

-- name: GetActiveMappingByBib :one
SELECT * FROM bib_transponder_mappings
WHERE event_id = $1 AND bib_number = $2 AND is_active = true;

-- name: ListMappingsByEvent :many
SELECT * FROM bib_transponder_mappings
WHERE event_id = $1
ORDER BY bib_number ASC, created_at DESC;

-- Timing Passings

-- name: InsertTimingPassing :one
INSERT INTO timing_passings (
    organization_id, event_id, checkpoint_code, provider,
    external_read_id, chip_code, raw_chip_code, bib_number, observed_at,
    raw_payload, metadata
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8, $9,
    $10, $11
)
ON CONFLICT (event_id, provider, external_read_id) WHERE external_read_id IS NOT NULL
DO NOTHING
RETURNING *;

-- name: InsertTimingPassingFallback :one
INSERT INTO timing_passings (
    organization_id, event_id, checkpoint_code, provider,
    external_read_id, chip_code, raw_chip_code, bib_number, observed_at,
    raw_payload, metadata
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8, $9,
    $10, $11
)
ON CONFLICT (event_id, provider, chip_code, checkpoint_code, observed_at) WHERE external_read_id IS NULL
DO NOTHING
RETURNING *;

-- name: ListUnprocessedPassings :many
SELECT * FROM timing_passings
WHERE event_id = $1 AND NOT processed
ORDER BY observed_at ASC
LIMIT $2;

-- name: MarkPassingsProcessed :exec
UPDATE timing_passings
SET processed = true, processed_at = now()
WHERE id = ANY($1::bigint[]);

-- name: CountPassingsByEvent :one
SELECT
    count(*) AS total,
    count(*) FILTER (WHERE processed) AS processed_count
FROM timing_passings
WHERE event_id = $1;

-- Race Split Times

-- name: UpsertRaceSplitTime :one
INSERT INTO race_split_times (
    race_result_id, checkpoint_id, passing_id, split_time_ms,
    split_pace_ms_km, passing_time, order_index
) VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (race_result_id, checkpoint_id) DO UPDATE SET
    passing_id       = EXCLUDED.passing_id,
    split_time_ms    = EXCLUDED.split_time_ms,
    split_pace_ms_km = EXCLUDED.split_pace_ms_km,
    passing_time     = EXCLUDED.passing_time,
    order_index      = EXCLUDED.order_index
RETURNING *;

-- name: ListSplitsByResult :many
SELECT s.*, c.code AS checkpoint_code, c.name AS checkpoint_name, c.distance_meters
FROM race_split_times s
JOIN timing_checkpoints c ON s.checkpoint_id = c.id
WHERE s.race_result_id = $1
ORDER BY s.order_index ASC;

-- Tickets Wave linking

-- name: SetTicketWave :exec
UPDATE tickets
SET wave_id = $2, updated_at = now()
WHERE id = $1;
