package racepack

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	abusemod "github.com/varin/ivyticketing/services/api/internal/modules/abuse"
	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterEventRoutes mounts racepack endpoints under
// /organizations/{orgId}/events/{eventId}/racepack/...
//
// Three permissions are in play:
//   - racepack.manage     : counter/slot CRUD
//   - racepack.execute    : pickup execution + dashboard
//   - racepack.problemdesk: problem-case CRUD
//
// All endpoints require authentication AND the route-level
// middleware.RequirePermission enforces org membership.
func (h *Handler) RegisterEventRoutes(r chi.Router, loader middleware.PermissionLoader) {
	h.RegisterEventRoutesWithLimits(r, loader, nil, nil)
}

// RegisterEventRoutesWithLimits is the same as RegisterEventRoutes but
// allows the caller to inject per-route rate-limit middlewares. The pickup
// limit applies to POST /pickups; the problem-case limit applies to POST
// /problem-cases. Both middleware arguments are of type func(string)
// matching abuse.RateChecker.Middleware.
func (h *Handler) RegisterEventRoutesWithLimits(
	r chi.Router,
	loader middleware.PermissionLoader,
	pickupLimit func(category string) func(http.Handler) http.Handler,
	problemLimit func(category string) func(http.Handler) http.Handler,
) {
	r.Route("/racepack", func(r chi.Router) {
		// Counters.
		r.With(middleware.RequirePermission(loader, "racepack.manage")).Route("/counters", func(r chi.Router) {
			r.Post("/", h.CreateCounter)
			r.Get("/", h.ListCounters)
			r.Put("/{counterId}", h.UpdateCounter)
		})
		r.With(middleware.RequirePermission(loader, "racepack.manage")).Post("/counters/{counterId}/activate", h.SetCounterActive)

		// Slots.
		r.With(middleware.RequirePermission(loader, "racepack.manage")).Route("/slots", func(r chi.Router) {
			r.Post("/", h.CreateSlot)
			r.Get("/", h.ListSlots)
			r.Put("/{slotId}", h.UpdateSlot)
		})
		r.With(middleware.RequirePermission(loader, "racepack.manage")).Post("/slots/{slotId}/activate", h.UpdateSlot)

		// Pickups (idempotency-Key supported on POST).
		pickupChain := func(r chi.Router) {
			if pickupLimit != nil {
				r.Use(pickupLimit(abusemod.CategoryRacepackPickup))
			}
			r.Post("/", h.CreatePickup)
			r.Get("/", h.ListPickups)
			r.Get("/status", h.GetPickupStatus)
		}
		r.With(middleware.RequirePermission(loader, "racepack.execute")).Route("/pickups", pickupChain)

		// Proxy authorizations.
		r.With(middleware.RequirePermission(loader, "racepack.execute")).Route("/proxy-authorizations", func(r chi.Router) {
			r.Post("/", h.CreateProxyAuthorization)
			r.Get("/", h.ListProxyAuthorizations)
		})

		// Problem cases (idempotency-Key supported on POST).
		problemChain := func(r chi.Router) {
			if problemLimit != nil {
				r.Use(problemLimit(abusemod.CategoryRacepackProblem))
			}
			r.Post("/", h.CreateProblemCase)
			r.Get("/", h.ListProblemCases)
			r.Put("/{caseId}", h.UpdateProblemCase)
		}
		r.With(middleware.RequirePermission(loader, "racepack.problemdesk")).Route("/problem-cases", problemChain)

		// Dashboard.
		r.With(middleware.RequirePermission(loader, "racepack.execute")).Get("/dashboard", h.GetDashboard)
	})
}

// RegisterParticipantRoutes mounts participant-facing racepack endpoints under
// /events/{eventId}/racepack/... — these require only an authenticated
// participant, not org membership.
func (h *Handler) RegisterParticipantRoutes(r chi.Router, requireAuth func(http.Handler) http.Handler) {
	r.Route("/events/{eventId}/racepack", func(r chi.Router) {
		r.With(requireAuth).Get("/slots", h.ListActiveSlotsForParticipant)
		r.With(requireAuth).Post("/slots/{slotId}/reserve", h.ReserveSlotForParticipant)
	})
}