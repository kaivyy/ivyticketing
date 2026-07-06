package scanner

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterEventRoutes mounts scanner endpoints under
// /organizations/{orgId}/events/{eventId}/scan/...
//
// Two permissions are in play:
//   - racepack.execute : racepack pickup staff (read participant info)
//   - checkin.execute  : event check-in staff (VALID -> USED transition)
//
// /scan/verify is readable by either role (both pickup and check-in staff must
// read participant info before confirming), so it is gated by
// RequireAnyPermission. /scan/check-in performs the VALID->USED transition and
// requires checkin.execute specifically.
func (h *Handler) RegisterEventRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Route("/scan", func(r chi.Router) {
		r.With(middleware.RequireAnyPermission(loader, "racepack.execute", "checkin.execute")).
			Post("/verify", h.Verify)
		r.With(middleware.RequirePermission(loader, "checkin.execute")).
			Post("/check-in", h.CheckIn)
	})
}

// RegisterUserRoutes mounts scanner endpoints that are authenticated but NOT
// org-scoped. GET /scan/events returns the staff member's Permitted_Events
// across every organization they belong to (Req 1.2), so it cannot live under
// the /organizations/{orgId}/events/{eventId} group. The caller mounts this on
// a router that already carries the authn middleware (task 7.2 wires it into
// server.go); per-operation authorization for a specific event is enforced by
// the service guard (Service.AssertEventPermitted) plus the org-scoped route
// middleware on /scan/verify and /scan/check-in.
func (h *Handler) RegisterUserRoutes(r chi.Router) {
	r.Get("/scan/events", h.ListEvents)
}
