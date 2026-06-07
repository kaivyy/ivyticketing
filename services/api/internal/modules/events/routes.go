package events

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts event endpoints under an existing /organizations/{orgId}
// router (already behind authn). Each route enforces its own permission.
func (h *Handler) RegisterRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Route("/events", func(r chi.Router) {
		r.With(middleware.RequirePermission(loader, "event.create")).Post("/", h.Create)
		r.With(middleware.RequirePermission(loader, "event.edit")).Get("/", h.List)
		r.With(middleware.RequirePermission(loader, "event.edit")).Get("/{eventId}", h.Get)
		r.With(middleware.RequirePermission(loader, "event.edit")).Put("/{eventId}", h.Update)
		r.With(middleware.RequirePermission(loader, "event.publish")).Post("/{eventId}/publish", h.Publish)
		r.With(middleware.RequirePermission(loader, "event.publish")).Post("/{eventId}/unpublish", h.Unpublish)
		r.With(middleware.RequirePermission(loader, "event.publish")).Post("/{eventId}/archive", h.Archive)
		r.With(middleware.RequirePermission(loader, "event.delete")).Delete("/{eventId}", h.Delete)
		r.With(middleware.RequirePermission(loader, "event.edit")).Post("/{eventId}/media/{kind}", h.RequestTicket)
		r.With(middleware.RequirePermission(loader, "event.edit")).Post("/{eventId}/media/{kind}/upload", h.UploadDirect)
		r.With(middleware.RequirePermission(loader, "event.edit")).Put("/{eventId}/media/{kind}/confirm", h.ConfirmMedia)
	})
}
