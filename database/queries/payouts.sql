-- name: CreatePayoutAccount :one
INSERT INTO org_payout_accounts (
    organization_id, bank_name, bank_code, account_number, account_name, is_verified
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (organization_id, bank_code, account_number) DO UPDATE SET
    bank_name = EXCLUDED.bank_name,
    account_name = EXCLUDED.account_name,
    updated_at = now()
RETURNING *;

-- name: ListPayoutAccountsByOrg :many
SELECT * FROM org_payout_accounts
WHERE organization_id = $1
ORDER BY created_at DESC;

-- name: GetPayoutAccountByID :one
SELECT * FROM org_payout_accounts WHERE id = $1 AND organization_id = $2;

-- name: DeletePayoutAccount :exec
DELETE FROM org_payout_accounts WHERE id = $1 AND organization_id = $2;

-- name: CreatePayoutRequest :one
INSERT INTO payout_requests (
    organization_id, event_id, payout_account_id, amount, currency, status,
    notes, requested_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListPayoutRequestsByOrg :many
SELECT * FROM payout_requests
WHERE organization_id = $1
ORDER BY created_at DESC;

-- name: ListPayoutRequestsByEvent :many
SELECT * FROM payout_requests
WHERE organization_id = $1 AND event_id = $2
ORDER BY created_at DESC;

-- name: UpdatePayoutRequestStatus :one
UPDATE payout_requests SET
    status = $2,
    rejection_reason = $3,
    processed_by = $4,
    processed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetOrgRefundSummary :one
SELECT COALESCE(SUM(refunded_amount), 0)::bigint AS total_refunded
FROM orders
WHERE organization_id = $1;

