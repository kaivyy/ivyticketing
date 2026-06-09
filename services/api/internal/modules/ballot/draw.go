package ballot

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
)

type DrawEntry struct {
	ID string // ballot_entry UUID as string
}

type DrawResult struct {
	EntryID    string
	Outcome    string
	Rank       int
	ResultHash string
}

// Shuffle performs deterministic Fisher-Yates shuffle using seed.
// entries must be ordered by id ASC before calling (deterministic input).
func Shuffle(seed string, entries []DrawEntry) []DrawEntry {
	h := sha256.Sum256([]byte(seed))
	src := rand.NewSource(int64(binary.BigEndian.Uint64(h[:8])))
	r := rand.New(src)
	out := make([]DrawEntry, len(entries))
	copy(out, entries)
	for i := len(out) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// Assign distributes outcomes to shuffled entries and computes per-entry result hashes.
func Assign(seed string, shuffled []DrawEntry, quota, waitlistSize int) []DrawResult {
	results := make([]DrawResult, len(shuffled))
	for i, e := range shuffled {
		outcome := OutcomeNotSelected
		switch {
		case i < quota:
			outcome = OutcomeWinner
		case i < quota+waitlistSize:
			outcome = OutcomeWaitlisted
		}
		raw := fmt.Sprintf("%s|%d|%s", seed, i, e.ID)
		hash := sha256.Sum256([]byte(raw))
		results[i] = DrawResult{
			EntryID:    e.ID,
			Outcome:    outcome,
			Rank:       i,
			ResultHash: hex.EncodeToString(hash[:]),
		}
	}
	return results
}
