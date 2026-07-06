package status

import (
	"github.com/go-chi/chi/v5"
)

// RegisterRoutes mounts the public, unauthenticated status endpoints under
// /public/status.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/public/status", func(r chi.Router) {
		r.Get("/", h.GetStatus)
		r.Get("/incidents", h.ListIncidents)
	})
}

// RegisterAdminRoutes mounts super-admin status management under /status.
// It is expected to be mounted inside an /admin group already gated by
// RequirePlatformAdmin.
func (h *Handler) RegisterAdminRoutes(r chi.Router) {
	r.Route("/status", func(r chi.Router) {
		r.Get("/incidents", h.ListIncidentsAdmin)
		r.Post("/incidents", h.CreateIncident)
		r.Post("/incidents/{incidentId}/updates", h.AddIncidentUpdate)
		r.Put("/components/{key}", h.UpdateComponent)
	})
}
