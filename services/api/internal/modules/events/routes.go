package events

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts event endpoints under an existing /organizations/{orgId}
// router (already behind authn). Each route enforces its own permission.
// An optional mountSubRoutes callback is called inside the /{eventId} scope so
// that sibling modules (e.g. categories) can mount their routes without
// creating a conflicting top-level subrouter for the same path prefix.
func (h *Handler) RegisterRoutes(r chi.Router, loader middleware.PermissionLoader, mountSubRoutes ...func(chi.Router)) {
	r.Route("/events", func(r chi.Router) {
		r.With(middleware.RequirePermission(loader, "event.create")).Post("/", h.Create)
		r.With(middleware.RequirePermission(loader, "event.edit")).Get("/", h.List)
		r.Route("/{eventId}", func(r chi.Router) {
			r.With(middleware.RequirePermission(loader, "event.edit")).Get("/", h.Get)
			r.With(middleware.RequirePermission(loader, "event.edit")).Put("/", h.Update)
			r.With(middleware.RequirePermission(loader, "event.publish")).Post("/publish", h.Publish)
			r.With(middleware.RequirePermission(loader, "event.publish")).Post("/unpublish", h.Unpublish)
			r.With(middleware.RequirePermission(loader, "event.publish")).Post("/archive", h.Archive)
			r.With(middleware.RequirePermission(loader, "event.delete")).Delete("/", h.Delete)
			r.With(middleware.RequirePermission(loader, "event.edit")).Post("/media/{kind}", h.RequestTicket)
			r.With(middleware.RequirePermission(loader, "event.edit")).Post("/media/{kind}/upload", h.UploadDirect)
			r.With(middleware.RequirePermission(loader, "event.edit")).Put("/media/{kind}/confirm", h.ConfirmMedia)
			// Allow sibling modules to mount their routes at this level.
			for _, mount := range mountSubRoutes {
				mount(r)
			}
		})
	})
}
