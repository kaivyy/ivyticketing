package billing

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterAdminRoutes mounts super-admin billing endpoints under /admin/billing.
// The caller wraps this subtree in RequirePlatformAdmin.
func (h *Handler) RegisterAdminRoutes(r chi.Router) {
	r.Route("/billing", func(r chi.Router) {
		r.Get("/packages", h.ListPackages)
		r.Post("/packages", h.CreatePackage)
		r.Put("/packages/{packageId}", h.UpdatePackage)

		r.Get("/revenue", h.PlatformRevenue)

		r.Put("/organizations/{orgId}/subscription", h.AssignSubscription)
		r.Post("/organizations/{orgId}/invoices", h.GenerateInvoice)
		r.Post("/invoices/{invoiceId}/paid", h.MarkInvoicePaid)
	})
}

// RegisterOrgRoutes mounts organizer-facing billing endpoints under
// /organizations/{orgId}/billing, gated on the billing.view permission.
func (h *Handler) RegisterOrgRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Route("/billing", func(r chi.Router) {
		r.With(middleware.RequirePermission(loader, "billing.view")).Get("/subscription", h.GetSubscription)
		r.With(middleware.RequirePermission(loader, "billing.view")).Get("/packages", h.ListActivePackages)
		r.With(middleware.RequirePermission(loader, "billing.view")).Get("/fees/summary", h.FeeSummary)
		r.With(middleware.RequirePermission(loader, "billing.view")).Get("/invoices", h.ListInvoices)
	})
}
