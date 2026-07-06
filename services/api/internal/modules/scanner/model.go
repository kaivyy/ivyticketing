package scanner

// Ticket status values reused from the tickets module. They mirror the literal
// strings persisted in tickets.status so the scanner can match on them without
// crossing module boundaries.
const (
	TicketStatusValid     = "VALID"
	TicketStatusUsed      = "USED"
	TicketStatusCancelled = "CANCELLED"
)

// Racepack pickup record status. Mirrors racepack.PickupRecordStatusPickedUp;
// an active PICKED_UP record means the participant has already collected their
// racepack.
const (
	PickupRecordStatusPickedUp = "PICKED_UP"
)

// Audit actions written by the scanner module.
const (
	AuditActionCheckinCompleted = "SCANNER_CHECKIN_COMPLETED"
	AuditActionQRRejected       = "SCANNER_QR_REJECTED"
)

// Idempotency scope for the check-in endpoint.
const IdempotencyScopeCheckin = "scanner.checkin"
