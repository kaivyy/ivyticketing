package payments

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts participant self-service payment endpoints at the authn level.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/orders/{orderId}/payments", func(r chi.Router) {
		r.Post("/", h.CreatePayment)
		r.Get("/", h.ListByOrder)
	})
	r.Get("/payments/{paymentId}", h.GetMine)
}

// RegisterOrgRoutes mounts organizer-scoped payment endpoints under /organizations/{orgId}.
func (h *Handler) RegisterOrgRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.With(middleware.RequirePermission(loader, "payment.view")).
		Get("/events/{eventId}/payments", h.ListByOrgEvent)
	r.With(middleware.RequirePermission(loader, "payment.manage")).
		Post("/payments/{paymentId}/reconcile", h.Reconcile)
}
