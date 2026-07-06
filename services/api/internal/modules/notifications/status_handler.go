package notifications

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// StatusHandler serves GET /admin/notifications/status.
// Must be mounted inside an authn + RequirePlatformAdmin middleware group.
type StatusHandler struct {
	driver      string
	configured  bool
	maxAttempts int32
}

// NewStatusHandler creates a StatusHandler.
func NewStatusHandler(driver string, configured bool, maxAttempts int32) *StatusHandler {
	return &StatusHandler{driver: driver, configured: configured, maxAttempts: maxAttempts}
}

// RegisterRoutes registers the status endpoint under the supplied chi router.
func (h *StatusHandler) RegisterRoutes(r chi.Router) {
	r.Get("/admin/notifications/status", http.HandlerFunc(h.handleStatus))
}

func (h *StatusHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(StatusResponse{
		Driver:      h.driver,
		Configured:  h.configured,
		MaxAttempts: h.maxAttempts,
	})
}

// StatusResponse is the response shape for the status endpoint.
type StatusResponse struct {
	Driver      string `json:"driver"`
	Configured  bool   `json:"configured"`
	MaxAttempts int32  `json:"maxAttempts"`
}