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

// RegisterEventRoutes mounts organizer ticket listing AND BIB management endpoints under
// /organizations/{orgId}/events/{eventId}.
//
// Two permissions are in play:
//   - ticket.view: read-only listing
//   - bib.manage : write operations (assign / override / clear / bulk / export)
//
// Reads remain gated by ticket.view so existing dashboards keep working; writes
// require bib.manage per phase 13 decision D2.
func (h *Handler) RegisterEventRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.With(middleware.RequirePermission(loader, "ticket.view")).
		Get("/tickets", h.ListByOrgEvent)

	// BIB management — all gated by bib.manage.
	r.Route("/tickets/bib", func(r chi.Router) {
		r.Use(middleware.RequirePermission(loader, "bib.manage"))
		r.Post("/bulk-assign", h.BulkAssignBibs)
		r.Get("/export", h.ExportBibsCSV)
		r.Get("/next", h.PreviewNextBib)
	})

	r.With(middleware.RequirePermission(loader, "bib.manage")).
		Route("/tickets/{ticketId}/bib", func(r chi.Router) {
			r.Post("/assign", h.AssignBibNext)
			r.Put("/", h.SetBib)
			r.Delete("/", h.ClearTicketBib)
		})
}