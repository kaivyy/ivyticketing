package enterprise

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
	"github.com/varin/ivyticketing/services/api/internal/platform/ratelimit"
)

// RegisterOrgRoutes mounts organizer-facing enterprise management endpoints
// under /organizations/{orgId}/enterprise, gated on the apikey.manage
// permission. These live inside the authenticated (session/JWT) subtree.
func (h *Handler) RegisterOrgRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Route("/enterprise", func(r chi.Router) {
		manage := middleware.RequirePermission(loader, "apikey.manage")

		r.With(manage).Get("/api-keys", h.ListAPIKeys)
		r.With(manage).Post("/api-keys", h.CreateAPIKey)
		r.With(manage).Delete("/api-keys/{keyId}", h.RevokeAPIKey)

		r.With(manage).Get("/webhooks", h.ListWebhooks)
		r.With(manage).Post("/webhooks", h.CreateWebhook)
		r.With(manage).Get("/webhooks/deliveries", h.ListDeliveries)
		r.With(manage).Delete("/webhooks/{webhookId}", h.DeleteWebhook)
	})
}

// RegisterPublicAPIRoutes mounts the versioned, API-key-authenticated public
// read API. This is a SEPARATE auth domain from the session/JWT routes: every
// request is authenticated by API key (X-API-Key or Bearer) via Authenticator,
// then scoped per endpoint via RequireScope. Handlers additionally enforce org
// ownership on single-resource lookups against the key's org (APIContext.OrgID).
//
// Mounted at the router root (not under /api/v1) so integrators hit a stable,
// versioned surface: /api/public/v1/...
func RegisterPublicAPIRoutes(r chi.Router, svc *Service, api *PublicAPI, limiter *ratelimit.Limiter) {
	r.Route("/api/public/v1", func(r chi.Router) {
		r.Use(svc.Authenticator(limiter))

		r.With(RequireScope("events:read")).Get("/events", api.ListEvents)
		r.With(RequireScope("events:read")).Get("/events/{eventId}", api.GetEvent)
		r.With(RequireScope("orders:read")).Get("/events/{eventId}/orders", api.ListOrders)
		r.With(RequireScope("orders:read")).Get("/orders/{orderId}", api.GetOrder)
		r.With(RequireScope("payments:read")).Get("/payments/{paymentId}", api.GetPayment)
	})
}
