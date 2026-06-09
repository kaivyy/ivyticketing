package lifecycle

const (
	StatusDraft     = "DRAFT"
	StatusActive    = "ACTIVE"
	StatusPaused    = "PAUSED"
	StatusCompleted = "COMPLETED"
	StatusCancelled = "CANCELLED"

	PhaseStatusPending   = "PENDING"
	PhaseStatusActive    = "ACTIVE"
	PhaseStatusCompleted = "COMPLETED"
	PhaseStatusSkipped   = "SKIPPED"
)

type WindowClosedReason string

const (
	ReasonWindowNotYetOpen   WindowClosedReason = "WINDOW_NOT_YET_OPEN"
	ReasonWindowExpired      WindowClosedReason = "WINDOW_EXPIRED"
	ReasonModeNotInLifecycle WindowClosedReason = "MODE_NOT_IN_LIFECYCLE"
	ReasonLifecyclePaused    WindowClosedReason = "LIFECYCLE_PAUSED"
)
