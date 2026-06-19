package notifications

import (
	"encoding/json"
	"net/http"
)

// StatusHandler serves GET /admin/notifications/status.
type StatusHandler struct {
	driver     string
	configured bool
	maxAttempts int32
}

// NewStatusHandler creates a StatusHandler.
func NewStatusHandler(driver string, configured bool, maxAttempts int32) *StatusHandler {
	return &StatusHandler{driver: driver, configured: configured, maxAttempts: maxAttempts}
}

// RegisterRoutes registers the status endpoint.
func (h *StatusHandler) RegisterRoutes(r http.Handler) {
	if mux, ok := r.(interface{ Get(pattern string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) }); ok {
		mux.Get("/admin/notifications/status", http.HandlerFunc(h.handleStatus))
	}
}

func (h *StatusHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(StatusResponse{
		Driver:     h.driver,
		Configured: h.configured,
		MaxAttempts: h.maxAttempts,
	})
}

// StatusResponse is the response shape for the status endpoint.
type StatusResponse struct {
	Driver     string `json:"driver"`
	Configured bool   `json:"configured"`
	MaxAttempts int32 `json:"maxAttempts"`
}
