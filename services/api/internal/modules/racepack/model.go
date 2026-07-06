package racepack

// Counter / slot active flags.
const (
	CounterStatusActive = true
	SlotStatusActive    = true
)

// Pickup methods. These are also the values persisted in racepack_pickup_records.pickup_method.
const (
	PickupMethodSelf           = "SELF"
	PickupMethodProxy          = "PROXY"
	PickupMethodManualOverride = "MANUAL_OVERRIDE"
)

// Pickup record status values. "PICKED_UP" is the steady state, "CANCELLED" is
// reserved for soft-cancellations (not currently produced by Phase 14 code, but
// available for future dispute / reversal flows).
const (
	PickupRecordStatusPickedUp  = "PICKED_UP"
	PickupRecordStatusCancelled = "CANCELLED"
)

// Problem-case status values. The transition graph is enforced by
// ValidateStateTransition in eligibility.go.
const (
	ProblemCaseStatusOpen        = "OPEN"
	ProblemCaseStatusUnderReview = "UNDER_REVIEW"
	ProblemCaseStatusResolved    = "RESOLVED"
	ProblemCaseStatusEscalated   = "ESCALATED"
)

// Status values reused from other modules. They mirror the values in the
// tickets / orders packages so eligibility checks can match on the same
// literal strings without crossing module boundaries.
const (
	OrderStatusPaid       = "PAID"
	TicketStatusValid     = "VALID"
	TicketStatusCancelled = "CANCELLED"
)
