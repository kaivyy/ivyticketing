-- name: ListSubscriptionPackages :many
SELECT * FROM subscription_packages
ORDER BY sort_order, price_monthly;

-- name: ListActiveSubscriptionPackages :many
SELECT * FROM subscription_packages
WHERE is_active = true
ORDER BY sort_order, price_monthly;

-- name: GetSubscriptionPackage :one
SELECT * FROM subscription_packages WHERE id = $1;

-- name: GetSubscriptionPackageBySlug :one
SELECT * FROM subscription_packages WHERE slug = $1;

-- name: CreateSubscriptionPackage :one
INSERT INTO subscription_packages (slug, name, description, price_monthly, max_events, fee_bps, features, sort_order)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateSubscriptionPackage :one
UPDATE subscription_packages
SET name = $2, description = $3, price_monthly = $4, max_events = $5,
    fee_bps = $6, features = $7, is_active = $8, sort_order = $9, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetOrgSubscription :one
SELECT sqlc.embed(os), sqlc.embed(sp)
FROM org_subscriptions os
JOIN subscription_packages sp ON sp.id = os.package_id
WHERE os.organization_id = $1;

-- name: UpsertOrgSubscription :one
INSERT INTO org_subscriptions (organization_id, package_id, status, started_at, expires_at)
VALUES ($1, $2, 'ACTIVE', now(), $3)
ON CONFLICT (organization_id) DO UPDATE
SET package_id = EXCLUDED.package_id,
    status = 'ACTIVE',
    started_at = now(),
    expires_at = EXCLUDED.expires_at,
    updated_at = now()
RETURNING *;

-- name: CountEventsByOrg :one
SELECT COUNT(*)::bigint AS count FROM events WHERE organization_id = $1;

-- name: InsertPlatformFee :one
INSERT INTO platform_fee_ledger (organization_id, order_id, order_total, fee_bps, fee_amount)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (order_id) DO NOTHING
RETURNING *;

-- name: ListPlatformFeesByOrg :many
SELECT * FROM platform_fee_ledger
WHERE organization_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: PlatformFeeSummary :one
SELECT
    COALESCE(COUNT(*), 0)::bigint         AS entries,
    COALESCE(SUM(order_total), 0)::bigint AS gross_orders,
    COALESCE(SUM(fee_amount), 0)::bigint  AS total_fees
FROM platform_fee_ledger
WHERE organization_id = $1;

-- name: PlatformRevenueSummary :many
SELECT
    o.id                                    AS organization_id,
    o.name                                  AS organization_name,
    COALESCE(SUM(l.fee_amount), 0)::bigint  AS total_fees,
    COALESCE(SUM(l.order_total), 0)::bigint AS gross_orders,
    COALESCE(COUNT(l.id), 0)::bigint        AS fee_entries
FROM organizations o
LEFT JOIN platform_fee_ledger l ON l.organization_id = o.id
GROUP BY o.id, o.name
ORDER BY total_fees DESC;

-- name: CreatePlatformInvoice :one
INSERT INTO platform_invoices (organization_id, invoice_number, period_start, period_end, subscription_amount, fee_amount, total_amount, status, issued_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'ISSUED', now())
RETURNING *;

-- name: GetPlatformInvoice :one
SELECT * FROM platform_invoices WHERE id = $1;

-- name: ListPlatformInvoicesByOrg :many
SELECT * FROM platform_invoices
WHERE organization_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: MarkPlatformInvoicePaid :one
UPDATE platform_invoices
SET status = 'PAID', paid_at = now(), updated_at = now()
WHERE id = $1 AND status = 'ISSUED'
RETURNING *;
