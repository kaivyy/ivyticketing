-- Phase 16 — Reporting & Export.
-- Export-job lifecycle queries plus aggregate + row-level report queries.
-- Every report is org-scoped; event_id is an optional narrowing filter
-- (NULL param = all events in the org).

-- ==========================================================================
-- Export jobs
-- ==========================================================================

-- name: CreateExportJob :one
INSERT INTO export_jobs (organization_id, event_id, requested_by, report_type, format, params)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetExportJob :one
SELECT * FROM export_jobs WHERE id = $1 AND organization_id = $2;

-- name: ListExportJobsByOrg :many
SELECT * FROM export_jobs
WHERE organization_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ClaimPendingExportJob :one
UPDATE export_jobs
SET status = 'PROCESSING', started_at = now(), updated_at = now()
WHERE id = (
    SELECT id FROM export_jobs
    WHERE status = 'PENDING'
    ORDER BY created_at ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING *;

-- name: MarkExportJobReady :one
UPDATE export_jobs
SET status = 'READY', row_count = $2, file_key = $3, file_url = $4,
    completed_at = now(), updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkExportJobFailed :one
UPDATE export_jobs
SET status = 'FAILED', error = $2, completed_at = now(), updated_at = now()
WHERE id = $1
RETURNING *;

-- ==========================================================================
-- Participant report
-- ==========================================================================

-- name: ParticipantReportSummary :one
SELECT
    COUNT(*)                                             AS total_tickets,
    COUNT(*) FILTER (WHERE t.status = 'VALID')           AS valid_tickets,
    COUNT(*) FILTER (WHERE t.status = 'USED')            AS used_tickets,
    COUNT(*) FILTER (WHERE t.status = 'CANCELLED')       AS cancelled_tickets,
    COUNT(DISTINCT t.participant_id)                     AS unique_participants
FROM tickets t
WHERE t.organization_id = $1
  AND (sqlc.narg('event_id')::uuid IS NULL OR t.event_id = sqlc.narg('event_id'));

-- name: ParticipantReportRows :many
SELECT
    t.ticket_number, t.holder_name, t.holder_email, t.event_title,
    t.category_name, t.status, t.bib_number, t.issued_at, t.used_at
FROM tickets t
WHERE t.organization_id = $1
  AND (sqlc.narg('event_id')::uuid IS NULL OR t.event_id = sqlc.narg('event_id'))
ORDER BY t.issued_at ASC;

-- ==========================================================================
-- Sales report (orders)
-- ==========================================================================

-- name: SalesReportSummary :one
SELECT
    COUNT(*)                                              AS total_orders,
    COUNT(*) FILTER (WHERE o.status = 'PAID')             AS paid_orders,
    COUNT(*) FILTER (WHERE o.status = 'PENDING_PAYMENT')  AS pending_orders,
    COUNT(*) FILTER (WHERE o.status = 'EXPIRED')          AS expired_orders,
    COUNT(*) FILTER (WHERE o.status = 'CANCELLED')        AS cancelled_orders,
    COUNT(*) FILTER (WHERE o.status = 'REFUNDED')         AS refunded_orders,
    COALESCE(SUM(o.total) FILTER (WHERE o.status = 'PAID'), 0)::bigint AS gross_paid,
    COALESCE(SUM(o.discount) FILTER (WHERE o.status = 'PAID'), 0)::bigint AS total_discount
FROM orders o
WHERE o.organization_id = $1
  AND (sqlc.narg('event_id')::uuid IS NULL OR o.event_id = sqlc.narg('event_id'));

-- name: SalesReportRows :many
SELECT
    o.order_number, o.status, o.subtotal, o.fee, o.discount, o.total,
    u.full_name AS participant_name, u.email AS participant_email,
    o.created_at
FROM orders o
JOIN users u ON u.id = o.participant_id
WHERE o.organization_id = $1
  AND (sqlc.narg('event_id')::uuid IS NULL OR o.event_id = sqlc.narg('event_id'))
ORDER BY o.created_at ASC;

-- ==========================================================================
-- Payment report
-- ==========================================================================

-- name: PaymentReportSummary :many
SELECT
    p.gateway, p.method, p.status,
    COUNT(*)                        AS count,
    COALESCE(SUM(p.amount), 0)::bigint AS total_amount
FROM payments p
WHERE p.organization_id = $1
  AND (sqlc.narg('event_id')::uuid IS NULL OR p.event_id = sqlc.narg('event_id'))
GROUP BY p.gateway, p.method, p.status
ORDER BY p.gateway, p.method, p.status;

-- name: PaymentReportRows :many
SELECT
    p.merchant_reference, p.gateway, p.method, p.channel, p.status,
    p.amount, p.currency, p.gateway_reference, p.paid_at, p.created_at
FROM payments p
WHERE p.organization_id = $1
  AND (sqlc.narg('event_id')::uuid IS NULL OR p.event_id = sqlc.narg('event_id'))
ORDER BY p.created_at ASC;

-- ==========================================================================
-- Coupon report (access codes)
-- ==========================================================================

-- name: CouponReportSummary :many
SELECT
    ac.code_type,
    COUNT(*)                    AS total_codes,
    COALESCE(SUM(ac.max_uses), 0)::bigint  AS total_capacity,
    COALESCE(SUM(ac.use_count), 0)::bigint AS total_used
FROM access_codes ac
WHERE ac.organization_id = $1
  AND (sqlc.narg('event_id')::uuid IS NULL OR ac.event_id = sqlc.narg('event_id'))
GROUP BY ac.code_type
ORDER BY ac.code_type;

-- name: CouponReportRows :many
SELECT
    ac.code_type, ac.is_single_use, ac.max_uses, ac.use_count,
    ac.valid_from, ac.valid_until, ac.created_at
FROM access_codes ac
WHERE ac.organization_id = $1
  AND (sqlc.narg('event_id')::uuid IS NULL OR ac.event_id = sqlc.narg('event_id'))
ORDER BY ac.created_at ASC;

-- ==========================================================================
-- Queue report
-- ==========================================================================

-- name: QueueReportSummary :one
SELECT
    COUNT(*)                                        AS total_tokens,
    COUNT(*) FILTER (WHERE status = 'WAITING')      AS waiting,
    COUNT(*) FILTER (WHERE status = 'ALLOWED')      AS allowed,
    COUNT(*) FILTER (WHERE status = 'COMPLETED')    AS completed,
    COUNT(*) FILTER (WHERE status = 'EXPIRED')      AS expired,
    COUNT(*) FILTER (WHERE status = 'BLOCKED')      AS blocked
FROM queue_tokens
WHERE organization_id = $1
  AND (sqlc.narg('event_id')::uuid IS NULL OR event_id = sqlc.narg('event_id'));

-- name: QueueReportRows :many
SELECT
    qt.pool, qt.status, qt.score, qt.joined_at, qt.allowed_at,
    qt.completed_at, qt.expired_at,
    u.full_name AS participant_name, u.email AS participant_email
FROM queue_tokens qt
JOIN users u ON u.id = qt.participant_id
WHERE qt.organization_id = $1
  AND (sqlc.narg('event_id')::uuid IS NULL OR qt.event_id = sqlc.narg('event_id'))
ORDER BY qt.joined_at ASC;

-- ==========================================================================
-- Ballot report
-- ==========================================================================

-- name: BallotReportSummary :one
SELECT
    COUNT(*)                                           AS total_entries,
    COUNT(*) FILTER (WHERE status = 'APPLIED')         AS applied,
    COUNT(*) FILTER (WHERE status = 'WINNER')          AS winners,
    COUNT(*) FILTER (WHERE status = 'WAITLISTED')      AS waitlisted,
    COUNT(*) FILTER (WHERE status = 'NOT_SELECTED')    AS not_selected,
    COUNT(*) FILTER (WHERE status = 'CONVERTED')       AS converted,
    COUNT(*) FILTER (WHERE status = 'LAPSED')          AS lapsed
FROM ballot_entries
WHERE organization_id = $1
  AND (sqlc.narg('event_id')::uuid IS NULL OR event_id = sqlc.narg('event_id'));

-- name: BallotReportRows :many
SELECT
    be.status, be.applied_at, be.payment_deadline, be.converted_at,
    be.promoted_round,
    u.full_name AS participant_name, u.email AS participant_email
FROM ballot_entries be
JOIN users u ON u.id = be.participant_id
WHERE be.organization_id = $1
  AND (sqlc.narg('event_id')::uuid IS NULL OR be.event_id = sqlc.narg('event_id'))
ORDER BY be.applied_at ASC;

-- ==========================================================================
-- Racepack report
-- ==========================================================================

-- name: RacepackReportSummary :one
SELECT
    COUNT(*)                                              AS total_pickups,
    COUNT(*) FILTER (WHERE pickup_method = 'SELF')        AS self_pickups,
    COUNT(*) FILTER (WHERE pickup_method = 'PROXY')       AS proxy_pickups,
    COUNT(*) FILTER (WHERE pickup_method = 'MANUAL_OVERRIDE') AS manual_pickups
FROM racepack_pickup_records
WHERE organization_id = $1
  AND status = 'PICKED_UP'
  AND (sqlc.narg('event_id')::uuid IS NULL OR event_id = sqlc.narg('event_id'));

-- name: RacepackReportRows :many
SELECT
    rpr.bib_number, rpr.pickup_method, rpr.pickup_timestamp, rpr.notes,
    u.full_name AS participant_name, u.email AS participant_email,
    s.full_name AS staff_name
FROM racepack_pickup_records rpr
JOIN users u ON u.id = rpr.participant_id
JOIN users s ON s.id = rpr.staff_id
WHERE rpr.organization_id = $1
  AND rpr.status = 'PICKED_UP'
  AND (sqlc.narg('event_id')::uuid IS NULL OR rpr.event_id = sqlc.narg('event_id'))
ORDER BY rpr.pickup_timestamp ASC;

-- ==========================================================================
-- Revenue report (paid orders, daily buckets)
-- ==========================================================================

-- name: RevenueReportSummary :one
SELECT
    COALESCE(SUM(o.total), 0)::bigint     AS gross_revenue,
    COALESCE(SUM(o.discount), 0)::bigint  AS total_discount,
    COALESCE(SUM(o.fee), 0)::bigint       AS total_fee,
    COUNT(*)                              AS paid_orders,
    COALESCE(AVG(o.total), 0)::bigint     AS avg_order_value
FROM orders o
WHERE o.organization_id = $1
  AND o.status = 'PAID'
  AND (sqlc.narg('event_id')::uuid IS NULL OR o.event_id = sqlc.narg('event_id'));

-- name: RevenueReportRows :many
SELECT
    date_trunc('day', o.updated_at)::date AS day,
    COUNT(*)                              AS paid_orders,
    COALESCE(SUM(o.total), 0)::bigint     AS gross_revenue,
    COALESCE(SUM(o.discount), 0)::bigint  AS total_discount
FROM orders o
WHERE o.organization_id = $1
  AND o.status = 'PAID'
  AND (sqlc.narg('event_id')::uuid IS NULL OR o.event_id = sqlc.narg('event_id'))
GROUP BY date_trunc('day', o.updated_at)
ORDER BY day ASC;

-- ==========================================================================
-- Super-admin cross-org aggregate
-- ==========================================================================

-- name: PlatformRevenueByOrg :many
SELECT
    o.organization_id,
    org.name AS organization_name,
    COUNT(*) FILTER (WHERE o.status = 'PAID')          AS paid_orders,
    COALESCE(SUM(o.total) FILTER (WHERE o.status = 'PAID'), 0)::bigint AS gross_revenue
FROM orders o
JOIN organizations org ON org.id = o.organization_id
GROUP BY o.organization_id, org.name
ORDER BY gross_revenue DESC;
