-- name: CreateAccessPool :one
INSERT INTO access_pools
    (organization_id, event_id, category_id, pool_type, name, total_slots,
     is_visible_to_participants, eligibility_rule, valid_from, valid_until, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetAccessPool :one
SELECT * FROM access_pools WHERE id = $1;

-- name: ReservePoolSlot :one
UPDATE access_pools
SET reserved_slots = reserved_slots + 1
WHERE id = $1 AND reserved_slots + used_slots < total_slots
RETURNING *;

-- name: ConsumePoolSlot :exec
UPDATE access_pools
SET reserved_slots = reserved_slots - 1, used_slots = used_slots + 1
WHERE id = $1;

-- name: ReleasePoolSlot :exec
UPDATE access_pools
SET reserved_slots = reserved_slots - 1, released_slots = released_slots + 1
WHERE id = $1;

-- name: CreateAccessGrant :one
INSERT INTO access_grants
    (pool_id, participant_id, event_id, category_id, code_id, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAccessGrant :one
SELECT * FROM access_grants WHERE id = $1;

-- name: GetActiveGrantForParticipant :one
SELECT * FROM access_grants
WHERE participant_id = $1 AND category_id = $2 AND status = 'ACTIVE'
ORDER BY granted_at DESC
LIMIT 1;

-- name: ExpireGrant :exec
UPDATE access_grants SET status = 'EXPIRED' WHERE id = $1;

-- name: ConsumeGrant :exec
UPDATE access_grants
SET status = 'CONSUMED', consumed_at = now(), order_id = $2
WHERE id = $1;

-- name: ListExpiredActiveGrants :many
SELECT * FROM access_grants
WHERE status = 'ACTIVE' AND expires_at < now()
LIMIT $1;

-- name: CreateCorporateAccount :one
INSERT INTO corporate_accounts (organization_id, name, billing_email, invoice_required, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetCorporateAccount :one
SELECT * FROM corporate_accounts WHERE id = $1;

-- name: ListCorporateAccounts :many
SELECT * FROM corporate_accounts WHERE organization_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: ApproveCorporateAccount :one
UPDATE corporate_accounts
SET status = 'ACTIVE', approved_at = now(), approved_by = $2
WHERE id = $1 AND status = 'PENDING'
RETURNING *;

-- name: AddPoolMember :one
INSERT INTO access_pool_members (pool_id, user_id, email, eligibility_meta)
VALUES ($1, $2, $3, $4)
ON CONFLICT (pool_id, email) DO NOTHING
RETURNING *;

-- name: ListPoolMembers :many
SELECT * FROM access_pool_members
WHERE pool_id = $1 AND member_status != 'REVOKED'
ORDER BY invited_at DESC
LIMIT $2 OFFSET $3;

-- name: GetPoolMemberByEmail :one
SELECT * FROM access_pool_members WHERE pool_id = $1 AND email = $2;

-- name: UpdatePoolMemberStatus :one
UPDATE access_pool_members
SET member_status = $2,
    registered_at = CASE WHEN $2 = 'REGISTERED' THEN now() ELSE registered_at END,
    revoked_at    = CASE WHEN $2 = 'REVOKED' THEN now() ELSE revoked_at END,
    access_grant_id = COALESCE($3, access_grant_id)
WHERE id = $1
RETURNING *;

-- name: UpdateAccessPoolColumns :one
UPDATE access_pools
SET is_visible_to_participants = COALESCE($2, is_visible_to_participants),
    eligibility_rule = COALESCE($3, eligibility_rule),
    owner_account_id = COALESCE($4, owner_account_id)
WHERE id = $1
RETURNING *;

-- name: ListVisiblePoolsByCategory :many
SELECT * FROM access_pools
WHERE event_id = $1 AND category_id = $2
  AND is_visible_to_participants = true
  AND (valid_until IS NULL OR valid_until > now());

-- name: TransferPoolSlots :one
UPDATE access_pools SET total_slots = total_slots + $2 WHERE id = $1
  AND ($2 > 0 OR (total_slots + $2 >= reserved_slots + used_slots))
RETURNING *;

-- name: CountPaidOrdersByUserInOrg :one
SELECT count(*) FROM orders
WHERE participant_id = $1 AND organization_id = $2 AND status = 'PAID';

-- name: GetUserMembershipID :one
SELECT COALESCE(membership_id, '') FROM users WHERE id = $1;

-- name: HasPaidOrderForEvent :one
SELECT EXISTS(
    SELECT 1 FROM orders WHERE participant_id = $1 AND event_id = $2 AND status = 'PAID'
) AS exists;
