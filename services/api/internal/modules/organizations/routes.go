package organizations

import "github.com/go-chi/chi/v5"

// RegisterRoutes mounts org-level endpoints. The parent router must already be
// behind authn. Member/role sub-routes are mounted by their own modules.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/organizations", func(r chi.Router) {
		r.Post("/", h.Create)
		r.Get("/", h.List)
		r.Get("/{orgId}", h.Get)
	})
}
