package abuse

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
)

// Fingerprint returns a lightweight server-side hash of UA + client IP +
// Accept-Language. No client-side JS fingerprinting; no PII stored in clear.
func Fingerprint(r *http.Request) string {
	h := sha256.New()
	h.Write([]byte(r.Header.Get("User-Agent")))
	h.Write([]byte("|"))
	h.Write([]byte(ClientIP(r)))
	h.Write([]byte("|"))
	h.Write([]byte(r.Header.Get("Accept-Language")))
	return hex.EncodeToString(h.Sum(nil))
}
