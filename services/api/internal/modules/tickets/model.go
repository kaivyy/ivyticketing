package tickets

// Ticket statuses.
const (
	StatusValid     = "VALID"
	StatusUsed      = "USED"
	StatusCancelled = "CANCELLED"
)

// Order status constants needed for invoice gating (mirror orders module values).
const orderStatusPaid = "PAID"
