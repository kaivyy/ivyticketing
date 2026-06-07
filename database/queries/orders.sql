-- name: CreateOrder :one
INSERT INTO orders (organization_id, event_id, category_id, participant_id,
    order_number, status, subtotal, fee, discount, total, expired_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
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

-- name: CountActiveOrdersForUserCategory :one
SELECT count(*) FROM orders
WHERE category_id = $1 AND participant_id = $2
  AND status IN ('PENDING_PAYMENT','PAID');

-- name: CountPaidByCategory :one
SELECT count(*) FROM orders WHERE category_id = $1 AND status = 'PAID';

-- name: ListExpiredPendingOrders :many
SELECT id FROM orders
WHERE status = 'PENDING_PAYMENT' AND expired_at < now()
ORDER BY expired_at
FOR UPDATE SKIP LOCKED
LIMIT $1;
