package waitlist

import (
	"crypto/sha256"
	"encoding/binary"
	"time"

	"github.com/google/uuid"
)

func FIFORank(joinedAt time.Time) int64 { return joinedAt.UnixMicro() }

func RandomizedRank(seed string, participantID uuid.UUID) int64 {
	h := sha256.Sum256([]byte(seed + "|" + participantID.String()))
	return int64(binary.BigEndian.Uint64(h[:8]))
}
