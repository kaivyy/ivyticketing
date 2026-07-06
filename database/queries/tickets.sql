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

-- name: AssignBib :one
UPDATE tickets SET
    bib_number            = $2,
    bib_assigned_at       = now(),
    bib_assigned_by       = $3,
    bib_assignment_method = $4
WHERE id = $1
RETURNING *;

-- name: ClearBib :one
UPDATE tickets SET
    bib_number            = NULL,
    bib_assigned_at       = NULL,
    bib_assigned_by       = NULL,
    bib_assignment_method = NULL
WHERE id = $1
RETURNING *;

-- name: GetNextBibNumeric :one
-- Returns MAX(bib_number)::bigint among tickets with bib_number set for the event,
-- where bib_number is a purely numeric string (so non-numeric prefixed values are excluded).
-- Returns 0 if no numeric BIB exists yet for the event.
SELECT COALESCE(MAX(NULLIF(regexp_replace(bib_number, '[^0-9]', '', 'g'), '')::bigint), 0)::bigint AS next
FROM tickets
WHERE event_id = $1
  AND bib_number IS NOT NULL
  AND bib_number ~ '^[0-9]+$';

-- name: ListUnassignedTicketsByEvent :many
SELECT * FROM tickets
WHERE event_id = $1
  AND bib_number IS NULL
  AND status = 'VALID'
ORDER BY issued_at ASC;
-- name: MarkTicketUsed :one
-- Guarded VALID -> USED transition for event check-in. Only affects a ticket
-- that is currently VALID (idempotent no-op returns no row when already USED or
-- CANCELLED). used_at defaults to now() unless an original scan time is passed
-- (offline-synced check-ins carry their original scannedAt).
UPDATE tickets SET
    status  = 'USED',
    used_at = COALESCE(sqlc.narg('used_at'), now())
WHERE id = $1 AND status = 'VALID'
RETURNING *;

-- name: GetTicketDisplayInfo :one
-- Non-sensitive display fields for the scanner PWA. Projects ONLY the
-- whitelisted columns (participant name, BIB number, category name, ticket
-- status) so no sensitive data (email, phone, payment, passwords) can leak to
-- the scanner client. BIB number is NULL when unassigned.
SELECT
    holder_name   AS participant_name,
    bib_number    AS bib_number,
    category_name AS category_name,
    status        AS ticket_status
FROM tickets
WHERE id = $1;
