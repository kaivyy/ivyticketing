package status

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// Handler is the HTTP entry point for the status module.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) actorID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return uuid.Nil, false
	}
	return id.UserID, true
}

func parseIncidentID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "incidentId"))
	if err != nil {
		apperr.WriteError(w, r, ErrInvalidID)
		return uuid.Nil, false
	}
	return id, true
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(dst); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body"))
		return false
	}
	return true
}

// --- public ---

// GetStatus returns the public status page payload.
// GET /public/status
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetPublicStatus(r.Context())
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, resp)
}

// ListIncidents returns the incident history (newest first), paginated.
// GET /public/status/incidents?limit=&offset=
func (h *Handler) ListIncidents(w http.ResponseWriter, r *http.Request) {
	limit := int32(20)
	offset := int32(0)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = int32(n)
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = int32(n)
		}
	}
	incidents, err := h.svc.ListRecentIncidents(r.Context(), limit, offset)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, incidents)
}

// --- super-admin ---

// UpdateComponent sets a single component's status.
// PUT /admin/status/components/{key}
func (h *Handler) UpdateComponent(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actorID(w, r)
	if !ok {
		return
	}
	key := chi.URLParam(r, "key")
	var body UpdateComponentRequest
	if !decode(w, r, &body) {
		return
	}
	c, err := h.svc.UpdateComponent(r.Context(), actor, key, body.Status)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, c)
}

// CreateIncident opens a new incident.
// POST /admin/status/incidents
func (h *Handler) CreateIncident(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actorID(w, r)
	if !ok {
		return
	}
	var body CreateIncidentRequest
	if !decode(w, r, &body) {
		return
	}
	inc, err := h.svc.CreateIncident(r.Context(), actor, body)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, inc)
}

// AddIncidentUpdate appends an update to an incident and advances its status.
// POST /admin/status/incidents/{incidentId}/updates
func (h *Handler) AddIncidentUpdate(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.actorID(w, r)
	if !ok {
		return
	}
	incidentID, ok := parseIncidentID(w, r)
	if !ok {
		return
	}
	var body AddIncidentUpdateRequest
	if !decode(w, r, &body) {
		return
	}
	inc, err := h.svc.AddIncidentUpdate(r.Context(), actor, incidentID, body)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, inc)
}

// ListIncidentsAdmin returns the incident history for the admin console.
// GET /admin/status/incidents
func (h *Handler) ListIncidentsAdmin(w http.ResponseWriter, r *http.Request) {
	h.ListIncidents(w, r)
}
