package registration

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterEventRoutes mounts under /organizations/{orgId}/events/{eventId}.
func (h *Handler) RegisterEventRoutes(r chi.Router, loader middleware.PermissionLoader) {
	g := r.With(middleware.RequirePermission(loader, "registration.manage"))
	g.Get("/registration", h.GetEventSettings)
	g.Put("/registration", h.SetEventSettings)
	g.Put("/registration/category", h.SetCategorySettings)
}
