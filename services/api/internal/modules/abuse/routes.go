package abuse

import "github.com/go-chi/chi/v5"

// RegisterAdminRoutes mounts super-admin abuse endpoints.
// RequirePlatformAdmin is applied upstream in server.go.
func (h *Handler) RegisterAdminRoutes(r chi.Router) {
	r.Route("/abuse", func(r chi.Router) {
		r.Get("/settings", h.ListSettings)
		r.Put("/settings", h.SetSetting)
		r.Post("/block", h.Block)
		r.Post("/unblock", h.Unblock)
		r.Get("/blocked", h.ListBlocked)
		r.Get("/log", h.ListLog)
		r.Get("/ip-rules", h.ListIPRules)
		r.Post("/ip-rules", h.AddIPRule)
		r.Delete("/ip-rules/{ruleId}", h.DeleteIPRule)
	})
}
