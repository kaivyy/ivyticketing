package registration

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func parseOrgAndEvent(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		return uuid.Nil, uuid.Nil, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id")
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		return uuid.Nil, uuid.Nil, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id")
	}
	return orgID, eventID, nil
}

func (h *Handler) SetEventSettings(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, err := parseOrgAndEvent(r)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	var req EventSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	if err := h.svc.SetEventSettings(r.Context(), orgID, eventID, req); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) SetCategorySettings(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, err := parseOrgAndEvent(r)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	var req CategorySettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	catID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CATEGORY_ID", "invalid category id"))
		return
	}
	if err := h.svc.SetCategorySettings(r.Context(), orgID, eventID, catID, req); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetEventSettings(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, err := parseOrgAndEvent(r)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	resp, err := h.svc.GetEventSettings(r.Context(), orgID, eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, resp)
}
