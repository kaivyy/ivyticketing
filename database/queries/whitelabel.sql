-- name: GetOrgBranding :one
SELECT * FROM org_branding WHERE organization_id = $1;

-- name: UpsertOrgBranding :one
INSERT INTO org_branding (
    organization_id, logo_object_key, theme_color, email_from_name,
    email_from_address, terms_text, footer_text, whitelabel_enabled
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (organization_id) DO UPDATE
SET logo_object_key    = EXCLUDED.logo_object_key,
    theme_color        = EXCLUDED.theme_color,
    email_from_name    = EXCLUDED.email_from_name,
    email_from_address = EXCLUDED.email_from_address,
    terms_text         = EXCLUDED.terms_text,
    footer_text        = EXCLUDED.footer_text,
    whitelabel_enabled = EXCLUDED.whitelabel_enabled,
    updated_at         = now()
RETURNING *;

-- name: ListCustomDomainsByOrg :many
SELECT * FROM custom_domains
WHERE organization_id = $1
ORDER BY created_at DESC;

-- name: GetCustomDomain :one
SELECT * FROM custom_domains WHERE id = $1;

-- name: GetCustomDomainByName :one
SELECT * FROM custom_domains WHERE domain = $1;

-- name: CreateCustomDomain :one
INSERT INTO custom_domains (organization_id, domain, verification_token, status)
VALUES ($1, $2, $3, 'PENDING')
RETURNING *;

-- name: MarkCustomDomainVerified :one
UPDATE custom_domains
SET status = 'VERIFIED', verified_at = now(), updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkCustomDomainFailed :one
UPDATE custom_domains
SET status = 'FAILED', updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteCustomDomain :exec
DELETE FROM custom_domains WHERE id = $1 AND organization_id = $2;
