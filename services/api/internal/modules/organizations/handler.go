package organizations

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	id, _ := authctx.FromContext(r.Context())
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "name is required"))
		return
	}
	org, err := h.svc.Create(r.Context(), id.UserID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, org)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	id, _ := authctx.FromContext(r.Context())
	if id.IsPlatformAdmin {
		stats, err := h.svc.ListAllWithStats(r.Context())
		if err == nil {
			out := make([]Response, 0, len(stats))
			for _, s := range stats {
				out = append(out, Response{ID: s.ID, Name: s.Name, Slug: s.Slug, CreatedAt: s.CreatedAt})
			}
			apperr.WriteJSON(w, http.StatusOK, out)
			return
		}
	}
	orgs, err := h.svc.ListForUser(r.Context(), id.UserID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, orgs)
}

func (h *Handler) RegisterAdminRoutes(r chi.Router) {
	r.Get("/organizers", h.AdminListOrganizers)
	r.Post("/organizers", h.AdminCreateOrganizer)
}

func (h *Handler) AdminListOrganizers(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListAllWithStats(r.Context())
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) AdminCreateOrganizer(w http.ResponseWriter, r *http.Request) {
	var req AdminCreateOrganizerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	out, err := h.svc.AdminCreateOrganizer(r.Context(), req)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "CREATE_ORGANIZER_FAILED", err.Error()))
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, out)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, _ := authctx.FromContext(r.Context())
	rawOrg := chi.URLParam(r, "orgId")
	org, err := h.svc.GetByIdentifier(r.Context(), rawOrg, id.UserID, id.IsPlatformAdmin)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, org)
}

func (h *Handler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id"))
		return
	}
	limit := int32(50)
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = int32(l)
		}
	}
	logs, err := h.svc.ListAuditLogs(r.Context(), orgID, limit)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, logs)
}
