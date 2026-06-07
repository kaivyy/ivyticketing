package categories

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts category endpoints under an existing
// /organizations/{orgId}/events/{eventId} router (already behind authn).
func (h *Handler) RegisterRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Route("/categories", func(r chi.Router) {
		r.Use(middleware.RequirePermission(loader, "category.manage"))
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Get("/{categoryId}", h.Get)
		r.Put("/{categoryId}", h.Update)
		r.Delete("/{categoryId}", h.Delete)
	})
}
