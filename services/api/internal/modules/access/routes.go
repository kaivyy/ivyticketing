package access

import "github.com/go-chi/chi/v5"

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

// RegisterOrganizerRoutes mounts organizer-facing access management routes.
// These must be mounted inside an authn + org middleware group.
func (h *Handler) RegisterOrganizerRoutes(r chi.Router) {
	r.Post("/events/{eventId}/access/codes", h.CreateCode)
	r.Get("/events/{eventId}/access/codes", h.ListCodes)
	r.Delete("/access/codes/{codeId}", h.RevokeCode)
	r.Get("/events/{eventId}/access/pools", h.ListPools)
	r.Put("/access/pools/{poolId}", h.AdjustPool)

	// Corporate account management
	r.Post("/access/corporate", h.CreateCorporateAccount)
	r.Get("/access/corporate", h.ListCorporateAccounts)
	r.Post("/access/corporate/{accountId}/approve", h.ApproveCorporateAccount)
	r.Get("/access/corporate/{accountId}/invoice", h.GetInvoice)

	// Pool member management
	r.Post("/access/pools/{poolId}/members", h.BulkUploadMembers)
	r.Get("/access/pools/{poolId}/members", h.ListMembers)
}
