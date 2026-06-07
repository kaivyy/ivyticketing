package roles

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts role + permission endpoints under an existing
// /organizations/{orgId} router. The parent is already behind authn.
func (h *Handler) RegisterRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.With(middleware.RequirePermission(loader, "role.manage")).
		Get("/permissions", h.ListPermissions)
	r.Route("/roles", func(r chi.Router) {
		r.Use(middleware.RequirePermission(loader, "role.manage"))
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Put("/{roleId}", h.Update)
		r.Delete("/{roleId}", h.Delete)
	})
}
