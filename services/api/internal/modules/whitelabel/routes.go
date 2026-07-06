package whitelabel

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterOrgRoutes mounts organizer-facing white-label endpoints under
// /organizations/{orgId}/branding, gated on the branding.manage permission.
func (h *Handler) RegisterOrgRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Route("/branding", func(r chi.Router) {
		r.Use(middleware.RequirePermission(loader, "branding.manage"))

		r.Get("/", h.GetBranding)
		r.Put("/", h.UpsertBranding)

		r.Get("/domains", h.ListDomains)
		r.Post("/domains", h.AddDomain)
		r.Post("/domains/{domainId}/verify", h.VerifyDomain)
		r.Delete("/domains/{domainId}", h.DeleteDomain)
	})
}
