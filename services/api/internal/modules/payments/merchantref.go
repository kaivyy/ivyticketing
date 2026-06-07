package payments

import (
	"crypto/rand"
	"fmt"
	"time"
)

const refAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateMerchantReference(now time.Time) (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	suffix := make([]byte, 6)
	for i := range b {
		suffix[i] = refAlphabet[int(b[i])%len(refAlphabet)]
	}
	return fmt.Sprintf("PAY-%s-%s", now.Format("20060102"), string(suffix)), nil
}
