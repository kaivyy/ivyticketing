package queue

import "net/http"

// EntryGuard is a no-op middleware placeholder for Phase 9 (Turnstile, rate limit,
// duplicate detection). It currently passes every request through unchanged.
func EntryGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Phase 9: verify Turnstile token, apply rate limit, duplicate detection.
		next.ServeHTTP(w, r)
	})
}
