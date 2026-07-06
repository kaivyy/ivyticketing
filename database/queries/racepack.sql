-- name: CreateRacepackCounter :one
INSERT INTO racepack_counters (organization_id, event_id, name, location, active)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListRacepackCountersByEvent :many
SELECT * FROM racepack_counters WHERE event_id = $1 ORDER BY name ASC;

-- name: GetRacepackCounterByID :one
SELECT * FROM racepack_counters WHERE id = $1;

-- name: UpdateRacepackCounter :one
UPDATE racepack_counters
SET name = $2, location = $3, active = $4, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetRacepackCounterActive :one
UPDATE racepack_counters
SET active = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateRacepackPickupSlot :one
INSERT INTO racepack_pickup_slots (organization_id, event_id, name, pickup_date, start_time, end_time, capacity)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListRacepackPickupSlotsByEvent :many
SELECT * FROM racepack_pickup_slots
WHERE event_id = $1
ORDER BY pickup_date ASC, start_time ASC;

-- name: ListRacepackPickupSlotsActiveByEvent :many
SELECT * FROM racepack_pickup_slots
WHERE event_id = $1 AND active = true
ORDER BY pickup_date ASC, start_time ASC;

-- name: GetRacepackPickupSlotByID :one
SELECT * FROM racepack_pickup_slots WHERE id = $1;

-- name: IncrementRacepackPickupSlotReserved :one
UPDATE racepack_pickup_slots
SET reserved_count = reserved_count + 1, updated_at = now()
WHERE id = $1 AND reserved_count < capacity
RETURNING *;

-- name: DecrementRacepackPickupSlotReserved :exec
UPDATE racepack_pickup_slots
SET reserved_count = reserved_count - 1, updated_at = now()
WHERE id = $1 AND reserved_count > 0;

-- name: UpdateRacepackPickupSlot :one
UPDATE racepack_pickup_slots
SET name = $2, pickup_date = $3, start_time = $4, end_time = $5,
    capacity = $6, active = $7, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateRacepackPickupRecord :one
INSERT INTO racepack_pickup_records (
    organization_id, event_id, ticket_id, participant_id, bib_number,
    counter_id, slot_id, staff_id, pickup_method, notes
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetRacepackPickupRecordByTicket :one
SELECT * FROM racepack_pickup_records
WHERE ticket_id = $1 AND status = 'PICKED_UP';

-- name: GetRacepackPickupRecordByID :one
SELECT * FROM racepack_pickup_records WHERE id = $1;

-- name: ListRacepackPickupRecordsByEvent :many
SELECT * FROM racepack_pickup_records
WHERE event_id = $1 AND status = 'PICKED_UP'
ORDER BY pickup_timestamp DESC
LIMIT $2 OFFSET $3;

-- name: CountRacepackPickupRecordsByCounter :many
SELECT counter_id, COUNT(*) AS pickup_count
FROM racepack_pickup_records
WHERE event_id = $1 AND status = 'PICKED_UP'
  AND pickup_timestamp >= $2 AND pickup_timestamp < $3
GROUP BY counter_id;

-- name: CountRacepackPickupRecordsByEvent :one
SELECT COUNT(*) FROM racepack_pickup_records
WHERE event_id = $1 AND status = 'PICKED_UP';

-- name: CreateRacepackProxyAuthorization :one
INSERT INTO racepack_proxy_authorizations (
    organization_id, event_id, ticket_id, pickup_record_id,
    proxy_name, proxy_phone, proxy_identity, authorization_document, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListRacepackProxyAuthorizationsByTicket :many
SELECT * FROM racepack_proxy_authorizations
WHERE ticket_id = $1
ORDER BY created_at DESC;

-- name: GetRacepackProxyAuthorizationByID :one
SELECT * FROM racepack_proxy_authorizations WHERE id = $1;

-- name: CreateRacepackProblemCase :one
INSERT INTO racepack_problem_cases (
    organization_id, event_id, ticket_id, participant_id, status, reason, created_by
) VALUES ($1, $2, $3, $4, 'OPEN', $5, $6)
RETURNING *;

-- name: UpdateRacepackProblemCaseStatus :one
UPDATE racepack_problem_cases
SET status = $2,
    resolution = CASE WHEN $2 IN ('RESOLVED','ESCALATED') THEN $3 ELSE resolution END,
    resolved_by = CASE WHEN $2 IN ('RESOLVED','ESCALATED') THEN $4 ELSE resolved_by END,
    resolved_at = CASE WHEN $2 IN ('RESOLVED','ESCALATED') THEN now() ELSE resolved_at END,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListRacepackProblemCasesByEvent :many
SELECT * FROM racepack_problem_cases
WHERE event_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountRacepackProblemCasesByEventAndStatus :one
SELECT COUNT(*) FROM racepack_problem_cases
WHERE event_id = $1 AND status = $2;

-- name: GetRacepackProblemCaseByID :one
SELECT * FROM racepack_problem_cases WHERE id = $1;

-- name: LockTicketForUpdate :one
-- Acquires a row-level lock on the ticket row inside a transaction.
-- Used to close the TOCTOU window between eligibility check and pickup insert.
SELECT * FROM tickets WHERE id = $1 FOR UPDATE;

-- name: GetEventOrganizationID :one
SELECT organization_id FROM events WHERE id = $1;

-- name: CheckOrganizationMembership :one
SELECT EXISTS (
    SELECT 1 FROM organization_members
    WHERE organization_id = $1 AND user_id = $2 AND removed_at IS NULL
) AS is_member;

-- name: GetUserTicketByID :one
-- Returns ticket + order status in one query for the eligibility path.
SELECT t.id, t.event_id, t.participant_id, t.status, t.bib_number,
       o.status AS order_status
FROM tickets t
JOIN orders o ON o.id = t.order_id
WHERE t.id = $1;

-- name: GetIdempotencyKey :one
SELECT key, request_hash, response_status, response_body, created_at
FROM idempotency_keys
WHERE key = $1 AND scope = $2;

-- name: InsertIdempotencyKey :one
INSERT INTO idempotency_keys (key, scope, request_hash, response_status, response_body)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (key, scope) DO NOTHING
RETURNING *;