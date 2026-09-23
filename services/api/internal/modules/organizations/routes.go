package organizations

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts org-level endpoints. The parent router must already be
// behind authn. Member/role sub-routes are mounted by their own modules.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/organizations", func(r chi.Router) {
		r.Post("/", h.Create)
		r.Get("/", h.List)
		r.Get("/{orgId}", h.Get)
	})
}

// RegisterOrgRoutes mounts organization-scoped sub-routes.
func (h *Handler) RegisterOrgRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Get("/", h.Get)
	r.With(middleware.RequirePermission(loader, "organization.manage")).Get("/audit-logs", h.ListAuditLogs)
}
