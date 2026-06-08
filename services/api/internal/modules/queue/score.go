package queue

import (
	"crypto/sha256"
	"encoding/binary"
	"time"

	"github.com/google/uuid"
)

// FifoScore returns a monotonic score from wall-clock time (FIFO ordering).
func FifoScore(now time.Time) int64 {
	return now.UnixNano()
}

// PresaleScore returns a deterministic, seed-based pseudo-random score for a
// participant. Same (seed, participant) → same score (reproducible/auditable).
func PresaleScore(seed string, participantID uuid.UUID) int64 {
	h := sha256.New()
	h.Write([]byte(seed))
	h.Write(participantID[:])
	sum := h.Sum(nil)
	v := binary.BigEndian.Uint64(sum[:8])
	return int64(v >> 1) // ensure non-negative
}
