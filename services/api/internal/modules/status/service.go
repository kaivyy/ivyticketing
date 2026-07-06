package status

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

var (
	validComponentStatus = map[string]bool{
		CompOperational: true, CompDegraded: true, CompDown: true,
	}
	validImpact = map[string]bool{
		ImpactNone: true, ImpactMinor: true, ImpactMajor: true, ImpactCritical: true,
	}
	validIncidentStatus = map[string]bool{
		IncInvestigating: true, IncIdentified: true, IncMonitoring: true, IncResolved: true,
	}
)

// Service coordinates the public status page and the incident timeline.
type Service struct {
	repo  Repository
	audit *audit.Logger
	log   *slog.Logger
}

// NewService constructs a status Service.
func NewService(repo Repository, auditLog *audit.Logger, log *slog.Logger) *Service {
	return &Service{repo: repo, audit: auditLog, log: log}
}

// --- public ---

// GetPublicStatus assembles the public status page: overall health derived from
// components and active incidents, each component, and active incidents with
// their update timelines.
func (s *Service) GetPublicStatus(ctx context.Context) (StatusPageResponse, error) {
	comps, err := s.repo.ListComponents(ctx)
	if err != nil {
		return StatusPageResponse{}, err
	}
	incidents, err := s.repo.ListActiveIncidents(ctx)
	if err != nil {
		return StatusPageResponse{}, err
	}

	incResp, lastUpdated, err := s.hydrateIncidents(ctx, incidents)
	if err != nil {
		return StatusPageResponse{}, err
	}

	components := make([]ComponentResponse, 0, len(comps))
	worst := CompOperational
	for _, c := range comps {
		components = append(components, ComponentResponse{
			Key:       c.Key,
			Name:      c.Name,
			Status:    c.Status,
			SortOrder: c.SortOrder,
			UpdatedAt: tsStr(c.UpdatedAt),
		})
		worst = worseComponent(worst, c.Status)
		if u := c.UpdatedAt; u.Valid && u.Time.After(lastUpdated) {
			lastUpdated = u.Time
		}
	}

	overall := worst
	if len(incResp) > 0 && overall == CompOperational {
		overall = CompDegraded
	}

	last := ""
	if !lastUpdated.IsZero() {
		last = lastUpdated.UTC().Format(time.RFC3339)
	}
	return StatusPageResponse{
		Overall:     overall,
		Components:  components,
		Incidents:   incResp,
		LastUpdated: last,
	}, nil
}

// ListRecentIncidents returns the incident history (newest first), paginated.
func (s *Service) ListRecentIncidents(ctx context.Context, limit, offset int32) ([]IncidentResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	incidents, err := s.repo.ListRecentIncidents(ctx, db.ListRecentIncidentsParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	out, _, err := s.hydrateIncidents(ctx, incidents)
	return out, err
}

// --- super-admin ---

// UpdateComponent sets a single component's status.
func (s *Service) UpdateComponent(ctx context.Context, actor uuid.UUID, key, status string) (ComponentResponse, error) {
	key = strings.TrimSpace(key)
	status = strings.ToUpper(strings.TrimSpace(status))
	if key == "" || !validComponentStatus[status] {
		return ComponentResponse{}, ErrInvalidComponent
	}
	c, err := s.repo.UpdateComponent(ctx, db.UpdateStatusComponentParams{Key: key, Status: status})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ComponentResponse{}, ErrInvalidComponent
		}
		return ComponentResponse{}, err
	}
	s.record(ctx, actor, "status.component_updated", "status_component", key, map[string]any{"status": status})
	return ComponentResponse{
		Key:       c.Key,
		Name:      c.Name,
		Status:    c.Status,
		SortOrder: c.SortOrder,
		UpdatedAt: tsStr(c.UpdatedAt),
	}, nil
}

// CreateIncident opens a new incident with an initial update entry.
func (s *Service) CreateIncident(ctx context.Context, actor uuid.UUID, req CreateIncidentRequest) (IncidentResponse, error) {
	title := strings.TrimSpace(req.Title)
	body := strings.TrimSpace(req.Body)
	impact := strings.ToUpper(strings.TrimSpace(req.Impact))
	if impact == "" {
		impact = ImpactMinor
	}
	if title == "" || body == "" || !validImpact[impact] {
		return IncidentResponse{}, ErrInvalidIncident
	}
	inc, err := s.repo.CreateIncident(ctx, db.CreateIncidentParams{
		Title:  title,
		Impact: impact,
		Status: IncInvestigating,
	})
	if err != nil {
		return IncidentResponse{}, err
	}
	if _, err := s.repo.CreateIncidentUpdate(ctx, db.CreateIncidentUpdateParams{
		IncidentID: inc.ID,
		Status:     IncInvestigating,
		Body:       body,
	}); err != nil {
		return IncidentResponse{}, err
	}
	s.record(ctx, actor, "status.incident_created", "incident", inc.ID.String(), map[string]any{"title": title, "impact": impact})
	return s.loadIncident(ctx, inc.ID)
}

// AddIncidentUpdate appends an update to an incident and advances its status.
// When the new status is RESOLVED the incident is closed.
func (s *Service) AddIncidentUpdate(ctx context.Context, actor, incidentID uuid.UUID, req AddIncidentUpdateRequest) (IncidentResponse, error) {
	body := strings.TrimSpace(req.Body)
	status := strings.ToUpper(strings.TrimSpace(req.Status))
	if body == "" || !validIncidentStatus[status] {
		return IncidentResponse{}, ErrInvalidIncident
	}
	if _, err := s.repo.GetIncident(ctx, incidentID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return IncidentResponse{}, ErrIncidentNotFound
		}
		return IncidentResponse{}, err
	}
	if _, err := s.repo.CreateIncidentUpdate(ctx, db.CreateIncidentUpdateParams{
		IncidentID: incidentID,
		Status:     status,
		Body:       body,
	}); err != nil {
		return IncidentResponse{}, err
	}
	if _, err := s.repo.UpdateIncidentStatus(ctx, db.UpdateIncidentStatusParams{ID: incidentID, Status: status}); err != nil {
		return IncidentResponse{}, err
	}
	s.record(ctx, actor, "status.incident_updated", "incident", incidentID.String(), map[string]any{"status": status})
	return s.loadIncident(ctx, incidentID)
}

// --- helpers ---

func (s *Service) loadIncident(ctx context.Context, id uuid.UUID) (IncidentResponse, error) {
	inc, err := s.repo.GetIncident(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return IncidentResponse{}, ErrIncidentNotFound
		}
		return IncidentResponse{}, err
	}
	updates, err := s.repo.ListIncidentUpdates(ctx, id)
	if err != nil {
		return IncidentResponse{}, err
	}
	return toIncidentResponse(inc, updates), nil
}

// hydrateIncidents batch-loads updates for a set of incidents and returns the
// assembled responses plus the most recent update/incident timestamp seen.
func (s *Service) hydrateIncidents(ctx context.Context, incidents []db.Incident) ([]IncidentResponse, time.Time, error) {
	var lastUpdated time.Time
	if len(incidents) == 0 {
		return []IncidentResponse{}, lastUpdated, nil
	}
	ids := make([]uuid.UUID, 0, len(incidents))
	for _, inc := range incidents {
		ids = append(ids, inc.ID)
		if inc.UpdatedAt.Valid && inc.UpdatedAt.Time.After(lastUpdated) {
			lastUpdated = inc.UpdatedAt.Time
		}
	}
	updates, err := s.repo.ListUpdatesForIncidents(ctx, ids)
	if err != nil {
		return nil, lastUpdated, err
	}
	byIncident := make(map[uuid.UUID][]db.IncidentUpdate, len(incidents))
	for _, u := range updates {
		byIncident[u.IncidentID] = append(byIncident[u.IncidentID], u)
		if u.CreatedAt.Valid && u.CreatedAt.Time.After(lastUpdated) {
			lastUpdated = u.CreatedAt.Time
		}
	}
	out := make([]IncidentResponse, 0, len(incidents))
	for _, inc := range incidents {
		out = append(out, toIncidentResponse(inc, byIncident[inc.ID]))
	}
	return out, lastUpdated, nil
}

func (s *Service) record(ctx context.Context, actor uuid.UUID, action, targetType, targetID string, meta map[string]any) {
	if s.audit == nil {
		return
	}
	a := actor
	s.audit.Record(ctx, audit.Entry{
		ActorUserID: &a,
		Action:      action,
		TargetType:  targetType,
		TargetID:    targetID,
		Metadata:    meta,
	})
}

// worseComponent returns the more severe of two component statuses.
func worseComponent(a, b string) string {
	rank := map[string]int{CompOperational: 0, CompDegraded: 1, CompDown: 2}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

func toIncidentResponse(inc db.Incident, updates []db.IncidentUpdate) IncidentResponse {
	ups := make([]IncidentUpdateResponse, 0, len(updates))
	for _, u := range updates {
		ups = append(ups, IncidentUpdateResponse{
			ID:        u.ID.String(),
			Status:    u.Status,
			Body:      u.Body,
			CreatedAt: tsStr(u.CreatedAt),
		})
	}
	return IncidentResponse{
		ID:         inc.ID.String(),
		Title:      inc.Title,
		Impact:     inc.Impact,
		Status:     inc.Status,
		StartedAt:  tsStr(inc.StartedAt),
		ResolvedAt: tsPtr(inc.ResolvedAt),
		CreatedAt:  tsStr(inc.CreatedAt),
		UpdatedAt:  tsStr(inc.UpdatedAt),
		Updates:    ups,
	}
}
