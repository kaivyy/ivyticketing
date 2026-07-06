-- +goose Up
-- Phase 18 — White Label & Custom Domain: per-org branding overrides and
-- custom domains with DNS TXT verification. Gated behind the Enterprise package
-- (Phase 17 PackageGate: whitelabel / custom_domain features).

CREATE TABLE org_branding (
    organization_id     uuid PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    logo_object_key     text NOT NULL DEFAULT '',
    theme_color         text NOT NULL DEFAULT '#2563eb',   -- hex; drives accent color on public pages
    email_from_name     text NOT NULL DEFAULT '',
    email_from_address  text NOT NULL DEFAULT '',
    terms_text          text NOT NULL DEFAULT '',
    footer_text         text NOT NULL DEFAULT '',
    whitelabel_enabled  boolean NOT NULL DEFAULT false,    -- hides platform branding when true
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT org_branding_theme_color_check CHECK (theme_color ~ '^#[0-9A-Fa-f]{6}$')
);

CREATE TABLE custom_domains (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    domain              text NOT NULL UNIQUE,
    verification_token  text NOT NULL,                     -- value expected in the DNS TXT record
    status              text NOT NULL DEFAULT 'PENDING',
    verified_at         timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT custom_domains_status_check CHECK (status IN ('PENDING','VERIFIED','FAILED'))
);
CREATE INDEX idx_custom_domains_org ON custom_domains(organization_id, created_at DESC);

-- Organizer-facing permission: manage branding + custom domains.
INSERT INTO permissions (key, description) VALUES
    ('branding.manage', 'Manage organization branding and custom domains')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.organization_id IS NULL
  AND r.slug = 'owner'
  AND p.key = 'branding.manage'
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions WHERE permission_id IN
    (SELECT id FROM permissions WHERE key = 'branding.manage');
DELETE FROM permissions WHERE key = 'branding.manage';
DROP TABLE IF EXISTS custom_domains;
DROP TABLE IF EXISTS org_branding;
