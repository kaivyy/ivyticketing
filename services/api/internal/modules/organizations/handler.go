package organizations

import (
	"encoding/json"
	"net/http"

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
	orgs, err := h.svc.ListForUser(r.Context(), id.UserID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, orgs)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, _ := authctx.FromContext(r.Context())
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id"))
		return
	}
	org, err := h.svc.Get(r.Context(), orgID, id.UserID, id.IsPlatformAdmin)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, org)
}
