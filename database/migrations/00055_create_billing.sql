-- +goose Up
-- Phase 17 — Super Admin Platform Billing: subscription packages, per-org
-- subscriptions, platform fee ledger, platform invoices, and RBAC.

CREATE TABLE subscription_packages (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug          text NOT NULL UNIQUE,
    name          text NOT NULL,
    description   text NOT NULL DEFAULT '',
    price_monthly bigint NOT NULL DEFAULT 0,
    max_events    integer,                          -- NULL = unlimited
    fee_bps       integer NOT NULL DEFAULT 0,       -- platform fee in basis points (250 = 2.5%)
    features      jsonb NOT NULL DEFAULT '[]',      -- array of feature keys this package unlocks
    is_active     boolean NOT NULL DEFAULT true,
    sort_order    integer NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT subscription_packages_fee_check CHECK (fee_bps >= 0 AND fee_bps <= 10000),
    CONSTRAINT subscription_packages_price_check CHECK (price_monthly >= 0)
);

CREATE TABLE org_subscriptions (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
    package_id      uuid NOT NULL REFERENCES subscription_packages(id) ON DELETE RESTRICT,
    status          text NOT NULL DEFAULT 'ACTIVE',
    started_at      timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz,                     -- NULL = no expiry
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT org_subscriptions_status_check CHECK (status IN ('ACTIVE','CANCELLED','EXPIRED'))
);
CREATE INDEX idx_org_subscriptions_package ON org_subscriptions(package_id);

CREATE TABLE platform_fee_ledger (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    order_id        uuid NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    order_total     bigint NOT NULL,
    fee_bps         integer NOT NULL,
    fee_amount      bigint NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT platform_fee_ledger_amounts_check CHECK (order_total >= 0 AND fee_amount >= 0)
);
CREATE INDEX idx_platform_fee_ledger_org ON platform_fee_ledger(organization_id, created_at DESC);

CREATE TABLE platform_invoices (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    invoice_number      text NOT NULL UNIQUE,
    period_start        date NOT NULL,
    period_end          date NOT NULL,
    subscription_amount bigint NOT NULL DEFAULT 0,
    fee_amount          bigint NOT NULL DEFAULT 0,
    total_amount        bigint NOT NULL DEFAULT 0,
    status              text NOT NULL DEFAULT 'DRAFT',
    issued_at           timestamptz,
    paid_at             timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT platform_invoices_status_check CHECK (status IN ('DRAFT','ISSUED','PAID','VOID')),
    CONSTRAINT platform_invoices_amounts_check CHECK (subscription_amount >= 0 AND fee_amount >= 0 AND total_amount >= 0)
);
CREATE INDEX idx_platform_invoices_org ON platform_invoices(organization_id, created_at DESC);
CREATE INDEX idx_platform_invoices_status ON platform_invoices(status);

-- Seed the three default packages from the masterplan.
INSERT INTO subscription_packages (slug, name, description, price_monthly, max_events, fee_bps, features, sort_order) VALUES
    ('starter', 'Starter', 'Registrasi & pembayaran dasar untuk 1-3 event.', 0, 3, 500,
        '["basic_registration","basic_payment"]', 1),
    ('professional', 'Professional', 'Antrean, ballot, racepack, dan branding kustom.', 49900000, NULL, 300,
        '["basic_registration","basic_payment","queue","ballot","racepack","custom_branding"]', 2),
    ('enterprise', 'Enterprise', 'White label, custom domain, dukungan khusus.', 199900000, NULL, 150,
        '["basic_registration","basic_payment","queue","ballot","racepack","custom_branding","whitelabel","custom_domain","custom_payment","dedicated_queue","api"]', 3)
ON CONFLICT (slug) DO NOTHING;

-- Organizer-facing permission: view own subscription + request upgrade.
INSERT INTO permissions (key, description) VALUES
    ('billing.view', 'View organization subscription and invoices')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.organization_id IS NULL
  AND r.slug IN ('owner', 'finance')
  AND p.key = 'billing.view'
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions WHERE permission_id IN
    (SELECT id FROM permissions WHERE key = 'billing.view');
DELETE FROM permissions WHERE key = 'billing.view';
DROP TABLE IF EXISTS platform_invoices;
DROP TABLE IF EXISTS platform_fee_ledger;
DROP TABLE IF EXISTS org_subscriptions;
DROP TABLE IF EXISTS subscription_packages;
