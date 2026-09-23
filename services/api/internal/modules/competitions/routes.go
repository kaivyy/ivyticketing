package competitions

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterPublicRoutes mounts public discovery and viewing endpoints.
func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Get("/sports", h.ListSports)
	r.Get("/sports/{sportId}/disciplines", h.ListDisciplines)
	r.Get("/events/{eventId}/competition", h.GetCompetitionConfig)
	r.Get("/events/{eventId}/teams", h.ListTeams)
	r.Get("/events/{eventId}/teams/{teamId}/roster", h.ListTeamRoster)
	r.Get("/events/{eventId}/stages", h.ListStages)
	r.Get("/events/{eventId}/stages/{stageId}/matches", h.ListMatches)
	r.Get("/events/{eventId}/stages/{stageId}/standings", h.GetStageStandings)
}

// RegisterEventRoutes mounts organizer management endpoints under
// /organizations/{orgId}/events/{eventId}/competition/...
func (h *Handler) RegisterEventRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Route("/competition", func(r chi.Router) {
		r.Use(middleware.RequirePermission(loader, "results.manage"))

		r.Put("/config", h.ConfigureCompetition)
		r.Post("/teams", h.CreateTeam)
		r.Post("/teams/{teamId}/roster", h.AddRosterMember)
		r.Post("/stages", h.CreateStage)
		r.Post("/matches/{matchId}/score", h.RecordMatchScore)
		r.Post("/stages/{stageId}/entries/{entryId}/adjudicate", h.AdjudicateEntry)
		r.Post("/stages/{stageId}/process-cycling", h.ProcessCyclingStage)
	})
}
