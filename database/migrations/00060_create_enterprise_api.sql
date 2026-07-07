-- +goose Up
-- Phase 23 — Enterprise API & Integration: per-org API keys (hashed),
-- outbound webhook subscriptions, and an idempotent delivery ledger.

CREATE TABLE api_keys (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name              text NOT NULL,
    key_prefix        text NOT NULL,                 -- first 8 chars of the raw key, shown in UI
    key_hash          text NOT NULL,                 -- sha256(raw key) hex; raw key shown once at creation
    scopes            jsonb NOT NULL DEFAULT '[]',    -- array of scope strings, e.g. ["events:read","orders:read"]
    rate_limit_per_min integer NOT NULL DEFAULT 120,
    last_used_at      timestamptz,
    revoked_at        timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT api_keys_rate_check CHECK (rate_limit_per_min > 0 AND rate_limit_per_min <= 10000)
);
CREATE UNIQUE INDEX idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_org ON api_keys(organization_id, created_at DESC);

CREATE TABLE webhook_endpoints (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    url               text NOT NULL,
    secret            text NOT NULL,                 -- HMAC-SHA256 signing secret
    events            jsonb NOT NULL DEFAULT '[]',    -- subscribed event types, e.g. ["order.paid","result.imported"]
    is_active         boolean NOT NULL DEFAULT true,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_webhook_endpoints_org ON webhook_endpoints(organization_id, created_at DESC);

-- Idempotent delivery ledger. event_key is a caller-stable dedupe key
-- (typically "<event_type>:<resource_id>"); the UNIQUE(endpoint_id, event_key)
-- constraint guarantees at-most-once enqueue per endpoint per business event.
CREATE TABLE webhook_deliveries (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint_id       uuid NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    organization_id   uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_type        text NOT NULL,
    event_key         text NOT NULL,
    payload           jsonb NOT NULL,
    status            text NOT NULL DEFAULT 'PENDING',
    attempts          integer NOT NULL DEFAULT 0,
    last_error        text,
    next_attempt_at   timestamptz NOT NULL DEFAULT now(),
    delivered_at      timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT webhook_deliveries_status_check CHECK (status IN ('PENDING','DELIVERED','FAILED','DEAD')),
    CONSTRAINT webhook_deliveries_dedupe UNIQUE (endpoint_id, event_key)
);
CREATE INDEX idx_webhook_deliveries_due ON webhook_deliveries(next_attempt_at) WHERE status = 'PENDING';
CREATE INDEX idx_webhook_deliveries_org ON webhook_deliveries(organization_id, created_at DESC);

-- Organizer-facing permission to manage API keys + webhooks.
INSERT INTO permissions (key, description) VALUES
    ('apikey.manage', 'Manage organization API keys and outbound webhooks')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.organization_id IS NULL
  AND r.slug = 'owner'
  AND p.key = 'apikey.manage'
ON CONFLICT DO NOTHING;

-- Back-fill existing per-org owner roles so the permission is granted retroactively.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.organization_id IS NOT NULL
  AND r.slug = 'owner'
  AND p.key = 'apikey.manage'
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions WHERE permission_id IN
    (SELECT id FROM permissions WHERE key = 'apikey.manage');
DELETE FROM permissions WHERE key = 'apikey.manage';
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_endpoints;
DROP TABLE IF EXISTS api_keys;
