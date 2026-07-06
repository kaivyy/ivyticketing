package email

import "strings"

// MaskAddress redacts an email address for safe logging: it keeps the first
// character of the local part and the domain, masking the rest
// (e.g. "jane.doe@example.com" -> "j***@example.com"). PII must never appear
// verbatim in logs (Phase 22 acceptance criterion).
func MaskAddress(addr string) string {
	at := strings.LastIndex(addr, "@")
	if at <= 0 {
		// No local part to reveal; mask entirely.
		return "***"
	}
	local := addr[:at]
	domain := addr[at:]
	if len(local) == 1 {
		return "*" + domain
	}
	return local[:1] + "***" + domain
}
