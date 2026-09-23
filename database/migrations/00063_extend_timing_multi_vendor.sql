-- +goose Up
-- Phase 26: Multi-Vendor Timing Extensions, Guest Checkout, and Consent Tracking

-- 1. Orders: Support Guest Checkout identity and real T&C / waiver consent
ALTER TABLE orders
    ALTER COLUMN participant_id DROP NOT NULL;

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS guest_email citext,
    ADD COLUMN IF NOT EXISTS guest_name text,
    ADD COLUMN IF NOT EXISTS guest_phone text;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'orders_identity_check'
    ) THEN
        ALTER TABLE orders
            ADD CONSTRAINT orders_identity_check
            CHECK (participant_id IS NOT NULL OR (guest_email IS NOT NULL AND guest_name IS NOT NULL));
    END IF;
END $$;

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS terms_accepted_at timestamptz,
    ADD COLUMN IF NOT EXISTS terms_version text,
    ADD COLUMN IF NOT EXISTS waiver_accepted_at timestamptz,
    ADD COLUMN IF NOT EXISTS waiver_version text;

-- 2. Tickets: Support guest ticket holders
ALTER TABLE tickets
    ALTER COLUMN participant_id DROP NOT NULL;

-- 3. Timing Configs: Provider constraint, transport, sync_mode, policy
ALTER TABLE timing_configs
    DROP CONSTRAINT IF EXISTS timing_configs_provider_check;

ALTER TABLE timing_configs
    ADD CONSTRAINT timing_configs_provider_check
    CHECK (provider IN ('RACE_RESULT', 'GENERIC_CSV', 'VENDOR_API', 'NATIVE_RFID', 'MANUAL'));

ALTER TABLE timing_configs
    ADD COLUMN IF NOT EXISTS transport text NOT NULL DEFAULT 'HTTP_PUSH',
    ADD COLUMN IF NOT EXISTS sync_mode text NOT NULL DEFAULT 'RAW_PASSINGS',
    ADD COLUMN IF NOT EXISTS policy jsonb NOT NULL DEFAULT '{}'::jsonb;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'timing_configs_transport_check'
    ) THEN
        ALTER TABLE timing_configs
            ADD CONSTRAINT timing_configs_transport_check
            CHECK (transport IN ('HTTP_PUSH', 'HTTP_PULL', 'CSV_UPLOAD', 'LOCAL_AGENT', 'SFTP'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'timing_configs_sync_mode_check'
    ) THEN
        ALTER TABLE timing_configs
            ADD CONSTRAINT timing_configs_sync_mode_check
            CHECK (sync_mode IN ('RAW_PASSINGS', 'FINAL_RESULTS'));
    END IF;
END $$;

-- 4. Timing Checkpoints: global aliases and provider-specific aliases
ALTER TABLE timing_checkpoints
    ADD COLUMN IF NOT EXISTS aliases text[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS provider_aliases jsonb NOT NULL DEFAULT '{}'::jsonb;

-- 5. Timing Passings: raw chip code from hardware
ALTER TABLE timing_passings
    ADD COLUMN IF NOT EXISTS raw_chip_code text;

-- 6. Race Results: Status (DSQ, OTL) and multi-provider source
ALTER TABLE race_results
    DROP CONSTRAINT IF EXISTS race_results_status_check;

ALTER TABLE race_results
    ADD CONSTRAINT race_results_status_check
    CHECK (status IN ('FINISHED', 'DNF', 'DNS', 'DSQ', 'OTL'));

ALTER TABLE race_results
    DROP CONSTRAINT IF EXISTS race_results_source_check;

ALTER TABLE race_results
    ADD CONSTRAINT race_results_source_check
    CHECK (source IN ('CSV', 'RACE_RESULT', 'VENDOR_API', 'NATIVE_RFID', 'MANUAL', 'TIMING_API'));

-- 7. RBAC: timing.manage permission and timing-operator role
INSERT INTO permissions (key, description) VALUES
    ('timing.manage', 'Configure timing providers, mat checkpoints, waves, and process passings')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.organization_id IS NULL AND r.slug IN ('owner', 'manager') AND p.key = 'timing.manage'
ON CONFLICT DO NOTHING;

INSERT INTO roles (organization_id, name, slug, is_system) VALUES
    (NULL, 'Timing Operator', 'timing-operator', true)
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.key IN (
    'timing.manage', 'results.manage', 'participant.view'
)
WHERE r.organization_id IS NULL AND r.slug = 'timing-operator'
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions WHERE role_id IN (
    SELECT id FROM roles WHERE organization_id IS NULL AND slug = 'timing-operator'
);
DELETE FROM roles WHERE organization_id IS NULL AND slug = 'timing-operator';
DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE key = 'timing.manage'
);
DELETE FROM permissions WHERE key = 'timing.manage';

ALTER TABLE race_results
    DROP CONSTRAINT IF EXISTS race_results_source_check;
ALTER TABLE race_results
    ADD CONSTRAINT race_results_source_check
    CHECK (source IN ('CSV', 'TIMING_API'));

ALTER TABLE race_results
    DROP CONSTRAINT IF EXISTS race_results_status_check;
ALTER TABLE race_results
    ADD CONSTRAINT race_results_status_check
    CHECK (status IN ('FINISHED', 'DNF', 'DNS'));

ALTER TABLE timing_passings
    DROP COLUMN IF EXISTS raw_chip_code;

ALTER TABLE timing_checkpoints
    DROP COLUMN IF EXISTS provider_aliases,
    DROP COLUMN IF EXISTS aliases;

ALTER TABLE timing_configs
    DROP CONSTRAINT IF EXISTS timing_configs_sync_mode_check,
    DROP CONSTRAINT IF EXISTS timing_configs_transport_check,
    DROP COLUMN IF EXISTS policy,
    DROP COLUMN IF EXISTS sync_mode,
    DROP COLUMN IF EXISTS transport;

ALTER TABLE timing_configs
    DROP CONSTRAINT IF EXISTS timing_configs_provider_check;
ALTER TABLE timing_configs
    ADD CONSTRAINT timing_configs_provider_check
    CHECK (provider IN ('RACE_RESULT', 'CSV', 'NATIVE'));

ALTER TABLE tickets
    ALTER COLUMN participant_id SET NOT NULL;

ALTER TABLE orders
    DROP COLUMN IF EXISTS waiver_version,
    DROP COLUMN IF EXISTS waiver_accepted_at,
    DROP COLUMN IF EXISTS terms_version,
    DROP COLUMN IF EXISTS terms_accepted_at,
    DROP CONSTRAINT IF EXISTS orders_identity_check,
    DROP COLUMN IF EXISTS guest_phone,
    DROP COLUMN IF EXISTS guest_name,
    DROP COLUMN IF EXISTS guest_email;

ALTER TABLE orders
    ALTER COLUMN participant_id SET NOT NULL;
