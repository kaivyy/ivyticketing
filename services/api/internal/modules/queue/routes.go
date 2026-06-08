package queue

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts participant queue endpoints (authn already applied by parent router).
func (h *Handler) RegisterRoutes(r chi.Router, joinGuard func(http.Handler) http.Handler) {
	r.With(joinGuard).Post("/events/{eventId}/queue/join", h.Join)
	r.Get("/events/{eventId}/queue/status", h.Status)
}

// RegisterOrgRoutes mounts organizer queue controls under /organizations/{orgId}/events/{eventId}.
func (h *Handler) RegisterOrgRoutes(r chi.Router, loader middleware.PermissionLoader) {
	g := r.With(middleware.RequirePermission(loader, "queue.manage"))
	g.Post("/queue/pause", h.Pause)
	g.Post("/queue/resume", h.Resume)
	g.Put("/queue/release-rate", h.SetRate)
	g.Get("/queue/stats", h.QueueStats)
	g.Put("/queue/schedule", h.SetSchedule)
}
