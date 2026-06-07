package orders

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts cross-event participant self-service endpoints at authn level.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/orders", h.List)
	r.Route("/orders/{orderId}", func(r chi.Router) {
		r.Get("/", h.Get)
		r.Delete("/", h.Cancel)
	})
}

// RegisterEventRoutes mounts event-scoped endpoints under /events/{eventId}.
func (h *Handler) RegisterEventRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Post("/categories/{categoryId}/checkout", h.Checkout)
	r.With(middleware.RequirePermission(loader, "order.view")).Get("/orders", h.ListByOrgEvent)
}
