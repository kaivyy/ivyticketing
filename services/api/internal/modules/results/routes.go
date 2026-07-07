package results

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts participant self-service result endpoints at the authn
// level (no org membership required). Ownership of the ticket is verified via
// the injected TicketOwnershipFunc, so a participant can read only their own
// result and certificate.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/tickets/{ticketId}/result", h.GetMyResult)
	r.Get("/tickets/{ticketId}/certificate", h.GetMyCertificate)
}

// RegisterEventRoutes mounts result + certificate endpoints under
// /organizations/{orgId}/events/{eventId}/results/...
//
// Organizer-facing management (import, recompute, delete, template CRUD) is
// gated on the results.manage permission. Read endpoints (leaderboard by bib,
// rendered certificate) require only an authenticated org member so on-site
// staff can look up finishers without the manage grant.
func (h *Handler) RegisterEventRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Route("/results", func(r chi.Router) {
		// Management.
		r.With(middleware.RequirePermission(loader, "results.manage")).Post("/import", h.ImportResults)
		r.With(middleware.RequirePermission(loader, "results.manage")).Post("/recompute", h.Recompute)
		r.With(middleware.RequirePermission(loader, "results.manage")).Delete("/", h.DeleteResults)

		// Read.
		r.Get("/", h.ListResults)
		r.Get("/bib/{bib}", h.GetResultByBib)
		r.Get("/certificate/{ticketId}", h.GetCertificate)

		// Certificate templates.
		r.Route("/templates", func(r chi.Router) {
			r.Get("/", h.ListTemplates)
			r.With(middleware.RequirePermission(loader, "results.manage")).Post("/", h.CreateTemplate)
			r.With(middleware.RequirePermission(loader, "results.manage")).Put("/{templateId}", h.UpdateTemplate)
			r.With(middleware.RequirePermission(loader, "results.manage")).Delete("/{templateId}", h.DeleteTemplate)
		})
	})
}
