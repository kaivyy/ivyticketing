package orders

import (
	"crypto/rand"
	"time"
)

const orderNumAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateOrderNumber(now time.Time) (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	suffix := make([]byte, 6)
	for i := range b {
		suffix[i] = orderNumAlphabet[int(b[i])%len(orderNumAlphabet)]
	}
	return "ORD-" + now.Format("20060102") + "-" + string(suffix), nil
}
