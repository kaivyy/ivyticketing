package queue

import "github.com/go-chi/chi/v5"

// RegisterRoutes mounts participant queue endpoints (authn already applied by parent router).
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.With(EntryGuard).Post("/events/{eventId}/queue/join", h.Join)
	r.Get("/events/{eventId}/queue/status", h.Status)
}
