package registration

type Mode string

const (
	ModeNormal          Mode = "NORMAL"
	ModeWarQueue        Mode = "WAR_QUEUE"
	ModeRandomizedQueue Mode = "RANDOMIZED_QUEUE"
	ModeHybridQueue     Mode = "HYBRID_QUEUE"
	ModeBallot          Mode = "BALLOT"
	ModeInvitationOnly  Mode = "INVITATION_ONLY"
	ModePriorityAccess  Mode = "PRIORITY_ACCESS"
	ModeWaitlistOnly    Mode = "WAITLIST_ONLY"
	ModeClosed          Mode = "CLOSED"
)

func IsQueueMode(m Mode) bool {
	return m == ModeWarQueue || m == ModeRandomizedQueue || m == ModeHybridQueue
}

func Valid(m Mode) bool {
	switch m {
	case ModeNormal, ModeWarQueue, ModeRandomizedQueue, ModeHybridQueue,
		ModeBallot, ModeInvitationOnly, ModePriorityAccess, ModeWaitlistOnly, ModeClosed:
		return true
	}
	return false
}
