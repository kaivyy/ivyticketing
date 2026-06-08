-- name: CreateTicket :one
INSERT INTO tickets (
    organization_id, event_id, category_id, order_id, participant_id,
    ticket_number, holder_name, holder_email, event_title, category_name, qr_version
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT (order_id) DO NOTHING
RETURNING *;

-- name: GetTicketByID :one
SELECT * FROM tickets WHERE id = $1;

-- name: GetTicketByOrderID :one
SELECT * FROM tickets WHERE order_id = $1;

-- name: ListTicketsByParticipant :many
SELECT * FROM tickets WHERE participant_id = $1 ORDER BY issued_at DESC;

-- name: ListTicketsByEvent :many
SELECT * FROM tickets
WHERE organization_id = $1 AND event_id = $2
ORDER BY issued_at DESC;
