-- name: CreateOrder :one
INSERT INTO orders (organization_id, event_id, category_id, participant_id,
    order_number, status, subtotal, fee, discount, total, expired_at, form_answers)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: CreateGuestOrder :one
INSERT INTO orders (
    organization_id, event_id, category_id, participant_id,
    order_number, status, subtotal, fee, discount, total, expired_at,
    guest_email, guest_name, guest_phone,
    terms_accepted_at, terms_version, waiver_accepted_at, waiver_version,
    form_answers
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8, $9, $10, $11,
    $12, $13, $14,
    $15, $16, $17, $18,
    $19
)
RETURNING *;

-- name: LinkOrderToParticipant :one
UPDATE orders
SET participant_id = $2, updated_at = now()
WHERE id = $1 AND participant_id IS NULL
RETURNING *;

-- name: GetOrderByID :one
SELECT * FROM orders WHERE id = $1;

-- name: GetOrderByNumber :one
SELECT * FROM orders WHERE order_number = $1;

-- name: ListOrdersByParticipant :many
SELECT * FROM orders WHERE participant_id = $1 ORDER BY created_at DESC;

-- name: ListOrdersByOrgEvent :many
SELECT * FROM orders WHERE organization_id = $1 AND event_id = $2 ORDER BY created_at DESC;

-- name: UpdateOrderStatus :one
UPDATE orders SET status = $2, updated_at = now()
WHERE id = $1 AND status = $3
RETURNING *;

-- name: RecordOrderRefund :one
UPDATE orders SET
    status = 'REFUNDED',
    refunded_amount = $2,
    refund_reason = $3,
    refunded_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CountActiveOrdersForUserCategory :one
SELECT count(*) FROM orders
WHERE category_id = $1 AND participant_id = $2
  AND status IN ('PENDING_PAYMENT','PAID');

-- name: CountActiveOrdersForGuestCategory :one
SELECT count(*) FROM orders
WHERE category_id = $1 AND guest_email = $2
  AND status IN ('PENDING_PAYMENT','PAID');

-- name: CountPaidByCategory :one
SELECT count(*) FROM orders WHERE category_id = $1 AND status = 'PAID';

-- name: ListExpiredPendingOrders :many
SELECT id FROM orders
WHERE status = 'PENDING_PAYMENT' AND expired_at < now()
ORDER BY expired_at
FOR UPDATE SKIP LOCKED
LIMIT $1;
