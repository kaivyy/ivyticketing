package ballot

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

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
		r.Get("/ballot/{drawId}/verify", h.VerifyResultHash)
	})
}

// RegisterParticipantRoutes mounts participant-facing ballot endpoints.
// applyGuard is an abuse-rate middleware — pass abuseGuard.Middleware(CategoryBallotApply).
func (h *Handler) RegisterParticipantRoutes(r chi.Router, applyGuard func(http.Handler) http.Handler) {
	r.Route("/events/{eventId}/categories/{categoryId}/ballot", func(r chi.Router) {
		r.With(applyGuard).Post("/apply", h.Apply)
		r.Get("/my-entry", h.MyEntry)
		r.Delete("/my-entry", h.Withdraw)
	})
}
