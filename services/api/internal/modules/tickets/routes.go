package tickets

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts participant self-service ticket endpoints at the authn level.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/tickets", h.ListMine)
	r.Route("/tickets/{ticketId}", func(r chi.Router) {
		r.Get("/", h.GetMine)
		r.Get("/qr", h.GetQR)
	})
	r.Get("/orders/{orderId}/ticket", h.GetByOrder)
	r.Get("/orders/{orderId}/invoice", h.GetInvoice)
}

// RegisterEventRoutes mounts organizer ticket listing under /organizations/{orgId}/events/{eventId}.
func (h *Handler) RegisterEventRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.With(middleware.RequirePermission(loader, "ticket.view")).
		Get("/tickets", h.ListByOrgEvent)
}
