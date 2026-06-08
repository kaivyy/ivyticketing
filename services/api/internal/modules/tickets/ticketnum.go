package tickets

import (
	"crypto/rand"
	"time"
)

const ticketNumAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateTicketNumber(now time.Time) (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	suffix := make([]byte, 6)
	for i := range b {
		suffix[i] = ticketNumAlphabet[int(b[i])%len(ticketNumAlphabet)]
	}
	return "TIX-" + now.Format("20060102") + "-" + string(suffix), nil
}
