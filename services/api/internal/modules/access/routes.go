package access

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterParticipantRoutes mounts participant-facing access routes.
// These must be mounted inside an authn middleware group.
func (h *Handler) RegisterParticipantRoutes(r chi.Router) {
	r.Route("/events/{eventId}/access", func(r chi.Router) {
		r.Post("/redeem", h.Redeem)
		r.Get("/my-grants", h.MyGrants)
		r.Get("/priority-window", h.PriorityWindow)
	})
	r.Post("/events/{eventId}/categories/{categoryId}/waitlist/join", h.WaitlistJoin)
	r.Get("/events/{eventId}/categories/{categoryId}/waitlist/my-position", h.WaitlistPosition)
}

// RegisterEventRoutes mounts event-level access management routes under
// /organizations/{orgId}/events/{eventId}.
func (h *Handler) RegisterEventRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Route("/access", func(r chi.Router) {
		r.Use(middleware.RequirePermission(loader, "access.manage"))
		r.Post("/codes", h.CreateCode)
		r.Get("/codes", h.ListCodes)
		r.Get("/pools", h.ListPools)
	})
}

// RegisterOrgRoutes mounts organization-level access management routes under
// /organizations/{orgId}.
func (h *Handler) RegisterOrgRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Route("/access", func(r chi.Router) {
		r.Use(middleware.RequirePermission(loader, "access.manage"))
		r.Delete("/codes/{codeId}", h.RevokeCode)
		r.Put("/pools/{poolId}", h.AdjustPool)

		// Corporate account management (called by web /org/[orgId]/corporate.astro)
		r.Post("/corporate", h.CreateCorporateAccount)
		r.Get("/corporate", h.ListCorporateAccounts)
		r.Post("/corporate/{accountId}/approve", h.ApproveCorporateAccount)
		r.Get("/corporate/{accountId}/invoice", h.GetInvoice)

		// Pool member management
		r.Post("/pools/{poolId}/members", h.BulkUploadMembers)
		r.Get("/pools/{poolId}/members", h.ListMembers)
	})
}

// RegisterOrganizerRoutes mounts organizer-facing access management routes.
// Kept for backward compatibility.
func (h *Handler) RegisterOrganizerRoutes(r chi.Router) {
	r.Route("/access", func(r chi.Router) {
		r.Post("/codes", h.CreateCode)
		r.Get("/codes", h.ListCodes)
		r.Delete("/codes/{codeId}", h.RevokeCode)
		r.Get("/pools", h.ListPools)
		r.Put("/pools/{poolId}", h.AdjustPool)
		r.Post("/corporate", h.CreateCorporateAccount)
		r.Get("/corporate", h.ListCorporateAccounts)
		r.Post("/corporate/{accountId}/approve", h.ApproveCorporateAccount)
		r.Get("/corporate/{accountId}/invoice", h.GetInvoice)
		r.Post("/pools/{poolId}/members", h.BulkUploadMembers)
		r.Get("/pools/{poolId}/members", h.ListMembers)
	})
}
