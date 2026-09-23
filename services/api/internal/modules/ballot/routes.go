package ballot

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterEventRoutes mounts event-scoped ballot endpoints under
// /organizations/{orgId}/events/{eventId}.
func (h *Handler) RegisterEventRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.With(middleware.RequirePermission(loader, "ballot.manage")).Get("/ballots", h.ListDrawsByEvent)
	r.Route("/categories/{categoryId}/ballot", func(r chi.Router) {
		r.Use(middleware.RequirePermission(loader, "ballot.manage"))
		r.Post("/", h.CreateDraw)
		r.Get("/", h.GetActiveDrawByCategory)
	})
	r.Route("/ballot/{drawId}", func(r chi.Router) {
		r.Use(middleware.RequirePermission(loader, "ballot.manage"))
		r.Get("/", h.GetDraw)
		r.Put("/", h.UpdateDraw)
		r.Post("/open", h.OpenDraw)
		r.Post("/close", h.CloseDraw)
		r.Post("/run", h.RunDraw)
		r.Post("/announce", h.AnnounceDraw)
		r.Get("/results", h.ListResults)
		r.Post("/promote-waitlist", h.PromoteWaitlist)
		r.Get("/export", h.ExportCSV)
		r.Get("/verify", h.VerifyResultHash)
	})
}

// RegisterOrgRoutes mounts organization-scoped ballot endpoints under
// /organizations/{orgId}.
func (h *Handler) RegisterOrgRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Route("/ballot/{drawId}", func(r chi.Router) {
		r.Use(middleware.RequirePermission(loader, "ballot.manage"))
		r.Get("/", h.GetDraw)
		r.Put("/", h.UpdateDraw)
		r.Post("/open", h.OpenDraw)
		r.Post("/close", h.CloseDraw)
		r.Post("/run", h.RunDraw)
		r.Post("/announce", h.AnnounceDraw)
		r.Get("/results", h.ListResults)
		r.Post("/promote-waitlist", h.PromoteWaitlist)
		r.Get("/export", h.ExportCSV)
		r.Get("/verify", h.VerifyResultHash)
	})
}

// RegisterOrganizerRoutes is kept for backward compatibility.
func (h *Handler) RegisterOrganizerRoutes(r chi.Router) {
	r.Post("/categories/{categoryId}/ballot", h.CreateDraw)
	r.Route("/ballot/{drawId}", func(r chi.Router) {
		r.Get("/", h.GetDraw)
		r.Put("/", h.UpdateDraw)
		r.Post("/open", h.OpenDraw)
		r.Post("/close", h.CloseDraw)
		r.Post("/run", h.RunDraw)
		r.Post("/announce", h.AnnounceDraw)
		r.Get("/results", h.ListResults)
		r.Post("/promote-waitlist", h.PromoteWaitlist)
		r.Get("/export", h.ExportCSV)
		r.Get("/verify", h.VerifyResultHash)
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
