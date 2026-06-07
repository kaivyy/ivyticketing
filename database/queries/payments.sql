-- name: CreatePayment :one
INSERT INTO payments (
    organization_id, event_id, order_id, participant_id, gateway, method, channel,
    status, amount, currency, gateway_reference, merchant_reference, pay_url, qr_string,
    va_number, instructions, expires_at
) VALUES (
    $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17
)
RETURNING *;

-- name: GetPaymentByID :one
SELECT * FROM payments WHERE id = $1;

-- name: GetPaymentByMerchantRef :one
SELECT * FROM payments WHERE merchant_reference = $1;

-- name: GetPaymentByMerchantRefForUpdate :one
SELECT * FROM payments WHERE merchant_reference = $1 FOR UPDATE;

-- name: ListPaymentsByOrder :many
SELECT * FROM payments WHERE order_id = $1 ORDER BY created_at DESC;

-- name: ListPaymentsByOrgEvent :many
SELECT * FROM payments WHERE organization_id = $1 AND event_id = $2 ORDER BY created_at DESC;

-- name: GetActivePaymentByOrder :one
SELECT * FROM payments WHERE order_id = $1 AND status IN ('PENDING','PAID') LIMIT 1;

-- name: MarkPaymentPaid :one
UPDATE payments
SET status = 'PAID', paid_at = $2, gateway_reference = COALESCE($3, gateway_reference), updated_at = now()
WHERE id = $1 AND status = 'PENDING'
RETURNING *;

-- name: UpdatePaymentStatus :one
UPDATE payments
SET status = $2, updated_at = now()
WHERE id = $1 AND status = 'PENDING'
RETURNING *;

-- name: GetOrderByIDForUpdate :one
SELECT * FROM orders WHERE id = $1 FOR UPDATE;
