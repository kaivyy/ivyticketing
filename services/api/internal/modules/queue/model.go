package queue

const (
	StatusWaiting   = "WAITING"
	StatusAllowed   = "ALLOWED"
	StatusExpired   = "EXPIRED"
	StatusCompleted = "COMPLETED"
	StatusBlocked   = "BLOCKED"

	PoolPresale = "PRESALE"
	PoolFifo    = "FIFO"

	AdmissionActive   = "ACTIVE"
	AdmissionConsumed = "CONSUMED"
	AdmissionExpired  = "EXPIRED"

	StateRunning = "RUNNING"
	StatePaused  = "PAUSED"
)
