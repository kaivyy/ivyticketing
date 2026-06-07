package publiccatalog

import "github.com/go-chi/chi/v5"

// RegisterRoutes mounts public read-only catalog endpoints. No auth.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/public/organizations/{orgSlug}/events", func(r chi.Router) {
		r.Get("/", h.ListEvents)
		r.Get("/{eventSlug}", h.GetEvent)
	})
}
