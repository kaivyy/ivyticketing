package ballot

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterOrganizerRoutes(r chi.Router) {
	r.Route("/org/{orgId}", func(r chi.Router) {
		r.Post("/events/{eventId}/categories/{categoryId}/ballot", h.CreateDraw)
		r.Put("/ballot/{drawId}", h.UpdateDraw)
		r.Post("/ballot/{drawId}/open", h.OpenDraw)
		r.Post("/ballot/{drawId}/close", h.CloseDraw)
		r.Post("/ballot/{drawId}/run", h.RunDraw)
		r.Post("/ballot/{drawId}/announce", h.AnnounceDraw)
		r.Get("/ballot/{drawId}/results", h.ListResults)
		r.Post("/ballot/{drawId}/promote-waitlist", h.PromoteWaitlist)
		r.Get("/ballot/{drawId}/export", h.ExportCSV)
	})
}
