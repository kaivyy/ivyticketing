package orders

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts cross-event participant self-service endpoints at authn level.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/orders", h.List)
	r.Post("/orders/claim", h.ClaimOrder)
	r.Route("/orders/{orderId}", func(r chi.Router) {
		r.Get("/", h.Get)
		r.Delete("/", h.Cancel)
	})
}

// RegisterPublicRoutes mounts unauthenticated guest checkout endpoints.
func (h *Handler) RegisterPublicRoutes(r chi.Router, checkoutGuard func(http.Handler) http.Handler) {
	if checkoutGuard != nil {
		r.With(checkoutGuard).Post("/events/{eventId}/categories/{categoryId}/checkout", h.GuestCheckout)
	} else {
		r.Post("/events/{eventId}/categories/{categoryId}/checkout", h.GuestCheckout)
	}
}

// RegisterEventRoutes mounts event-scoped endpoints under /events/{eventId}.
func (h *Handler) RegisterEventRoutes(r chi.Router, loader middleware.PermissionLoader, checkoutGuard func(http.Handler) http.Handler) {
	r.With(checkoutGuard).Post("/categories/{categoryId}/checkout", h.Checkout)
	r.With(middleware.RequirePermission(loader, "order.view")).Get("/orders", h.ListByOrgEvent)
	r.With(middleware.RequirePermission(loader, "order.refund")).Post("/orders/{orderId}/refund", h.Refund)
}
