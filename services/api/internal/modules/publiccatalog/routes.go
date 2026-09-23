package publiccatalog

import "github.com/go-chi/chi/v5"

// RegisterRoutes mounts public read-only catalog endpoints. No auth.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/public", func(r chi.Router) {
		r.Get("/events", h.ListAllEvents)
		r.Get("/events/{idOrSlug}", h.GetEventByIDOrSlug)
		r.Route("/organizations/{orgSlug}/events", func(r chi.Router) {
			r.Get("/", h.ListEvents)
			r.Get("/{eventSlug}", h.GetEvent)
		})
	})
}
