package reporting

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterOrgRoutes mounts reporting endpoints under
// /organizations/{orgId}/reports/...
//
// Two permissions are in play:
//   - report.view   : read on-screen summaries + list/get export jobs
//   - report.export : request a new async export
func (h *Handler) RegisterOrgRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Route("/reports", func(r chi.Router) {
		r.With(middleware.RequirePermission(loader, "report.view")).Get("/{reportType}/summary", h.GetSummary)

		r.With(middleware.RequirePermission(loader, "report.view")).Get("/exports", h.ListExports)
		r.With(middleware.RequirePermission(loader, "report.view")).Get("/exports/{jobId}", h.GetExport)
		r.With(middleware.RequirePermission(loader, "report.export")).Post("/exports", h.CreateExport)
	})
}

// RegisterAdminRoutes mounts the super-admin cross-org revenue aggregate under
// /admin/reports/... (caller wraps this in RequirePlatformAdmin).
func (h *Handler) RegisterAdminRoutes(r chi.Router) {
	r.Route("/reports", func(r chi.Router) {
		r.Get("/platform-revenue", h.PlatformRevenue)
	})
}
