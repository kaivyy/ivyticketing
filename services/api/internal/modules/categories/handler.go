package categories

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) ids(w http.ResponseWriter, r *http.Request) (orgID, eventID uuid.UUID, ok bool) {
	var err error
	orgID, err = uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id"))
		return uuid.Nil, uuid.Nil, false
	}
	eventID, err = uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, eventID, true
}

func (h *Handler) categoryID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "categoryId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CATEGORY_ID", "invalid category id"))
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	out, err := h.svc.List(r.Context(), orgID, eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	var req WriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "name is required"))
		return
	}
	c, err := h.svc.Create(r.Context(), orgID, eventID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, c)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	categoryID, ok := h.categoryID(w, r)
	if !ok {
		return
	}
	c, err := h.svc.Get(r.Context(), orgID, eventID, categoryID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, c)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	categoryID, ok := h.categoryID(w, r)
	if !ok {
		return
	}
	var req WriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "name is required"))
		return
	}
	c, err := h.svc.Update(r.Context(), orgID, eventID, categoryID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, c)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	categoryID, ok := h.categoryID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), orgID, eventID, categoryID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
