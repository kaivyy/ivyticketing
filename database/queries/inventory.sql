-- name: LockCategoryForUpdate :one
SELECT * FROM event_categories WHERE id = $1 FOR UPDATE;

-- name: CountActiveReservationsByCategory :one
SELECT count(*) FROM inventory_reservations
WHERE category_id = $1 AND status = 'ACTIVE';

-- name: CreateReservation :one
INSERT INTO inventory_reservations (organization_id, event_id, category_id,
    order_id, participant_id, status, expires_at)
VALUES ($1, $2, $3, $4, $5, 'ACTIVE', $6)
RETURNING *;

-- name: GetReservationByOrder :one
SELECT * FROM inventory_reservations WHERE order_id = $1;

-- name: UpdateReservationStatusByOrder :exec
UPDATE inventory_reservations SET status = $2
WHERE order_id = $1 AND status = 'ACTIVE';

-- name: ExpireReservationsForOrder :exec
UPDATE inventory_reservations SET status = 'EXPIRED'
WHERE order_id = $1 AND status = 'ACTIVE';
