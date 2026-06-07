package events

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct {
	svc            *Service
	maxUploadBytes int64
}

func NewHandler(svc *Service, maxUploadBytes int64) *Handler {
	return &Handler{svc: svc, maxUploadBytes: maxUploadBytes}
}

func (h *Handler) orgID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id"))
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) eventID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.EventType == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "name and eventType are required"))
		return
	}
	ev, err := h.svc.Create(r.Context(), orgID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, ev)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	out, err := h.svc.List(r.Context(), orgID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	eventID, ok := h.eventID(w, r)
	if !ok {
		return
	}
	ev, err := h.svc.Get(r.Context(), orgID, eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, ev)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	eventID, ok := h.eventID(w, r)
	if !ok {
		return
	}
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.EventType == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "name and eventType are required"))
		return
	}
	ev, err := h.svc.Update(r.Context(), orgID, eventID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, ev)
}

func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	h.runTransition(w, r, h.svc.Publish)
}
func (h *Handler) Unpublish(w http.ResponseWriter, r *http.Request) {
	h.runTransition(w, r, h.svc.Unpublish)
}
func (h *Handler) Archive(w http.ResponseWriter, r *http.Request) {
	h.runTransition(w, r, h.svc.Archive)
}

func (h *Handler) runTransition(w http.ResponseWriter, r *http.Request, fn func(context.Context, uuid.UUID, uuid.UUID) (Response, error)) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	eventID, ok := h.eventID(w, r)
	if !ok {
		return
	}
	ev, err := fn(r.Context(), orgID, eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, ev)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgID(w, r)
	if !ok {
		return
	}
	eventID, ok := h.eventID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), orgID, eventID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
