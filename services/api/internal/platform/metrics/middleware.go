package metrics

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// statusRecorder captures the response status code for metrics labelling.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

// Middleware instruments every request: in-flight gauge, request counter, and
// latency histogram. The route label uses chi's matched pattern (not the raw
// path) so high-cardinality IDs never enter Prometheus.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		m.IncInflight()
		defer m.DecInflight()

		rec := &statusRecorder{ResponseWriter: w, status: 0}
		next.ServeHTTP(rec, r)

		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		route := chi.RouteContext(r.Context()).RoutePattern()
		m.ObserveHTTP(r.Method, route, rec.status, time.Since(start))
	})
}
