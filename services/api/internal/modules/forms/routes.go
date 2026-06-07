package forms

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts form-builder endpoints under an existing
// /organizations/{orgId}/events/{eventId} router (already behind authn).
// All routes require form.manage.
func (h *Handler) RegisterRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Route("/form", func(r chi.Router) {
		r.Use(middleware.RequirePermission(loader, "form.manage"))
		r.Get("/", h.GetForm)
		r.Put("/", h.UpdateForm)
		r.Get("/fields", h.ListFields)
		r.Post("/fields", h.AddField)
		r.Put("/fields/reorder", h.Reorder)
		r.Put("/fields/{fieldId}", h.UpdateField)
		r.Delete("/fields/{fieldId}", h.DeleteField)
		r.Get("/preview", h.Preview)
		r.Post("/preview/validate", h.PreviewValidate)
	})
}
