package members

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts member endpoints under an existing
// /organizations/{orgId} router (already behind authn).
func (h *Handler) RegisterRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Route("/members", func(r chi.Router) {
		r.Use(middleware.RequirePermission(loader, "member.manage"))
		r.Get("/", h.List)
		r.Post("/", h.Add)
		r.Delete("/{memberId}", h.Remove)
		r.Put("/{memberId}/roles", h.UpdateRoles)
	})
}
