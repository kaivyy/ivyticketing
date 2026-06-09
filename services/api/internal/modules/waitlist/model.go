package waitlist

const (
	StatusWaiting   = "WAITING"
	StatusPromoted  = "PROMOTED"
	StatusExpired   = "EXPIRED"
	StatusWithdrawn = "WITHDRAWN"

	ModeFIFO       = "FIFO"
	ModeRandomized = "RANDOMIZED"
	ModeHybrid     = "HYBRID"

	SourceBallot       = "BALLOT"
	SourceQuotaRelease = "QUOTA_RELEASE"
	SourceManual       = "MANUAL"
)
