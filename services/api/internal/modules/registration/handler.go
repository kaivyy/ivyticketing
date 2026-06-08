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

func (h *Handler) SetEventSettings(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	var req EventSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	if err := h.svc.SetEventSettings(r.Context(), eventID, req); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) SetCategorySettings(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
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
	if err := h.svc.SetCategorySettings(r.Context(), eventID, catID, req); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetEventSettings(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}

	evSettings, evErr := h.svc.repo.GetEventSettings(r.Context(), eventID)

	resp := SettingsResponse{EventID: eventID.String()}
	if evErr == nil {
		resp.DefaultMode = evSettings.DefaultMode
		resp.QueueEnabled = evSettings.QueueEnabled
		resp.BallotEnabled = evSettings.BallotEnabled
		resp.PriorityEnabled = evSettings.PriorityEnabled
		resp.WaitlistEnabled = evSettings.WaitlistEnabled
	} else {
		resp.DefaultMode = string(ModeNormal)
	}

	catSettings, err := h.svc.repo.ListCategorySettingsByEvent(r.Context(), eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	for _, cs := range catSettings {
		c := CategorySettingsResponse{
			CategoryID:      cs.CategoryID.String(),
			OverrideEnabled: cs.OverrideEnabled,
		}
		if cs.RegistrationMode.Valid {
			m := cs.RegistrationMode.String
			c.RegistrationMode = &m
		}
		resp.Categories = append(resp.Categories, c)
	}
	apperr.WriteJSON(w, http.StatusOK, resp)
}
