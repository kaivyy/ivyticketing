package queue

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

type Handler struct {
	svc     *Service
	limiter RateLimiter
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) WithRateLimiter(l RateLimiter) { h.limiter = l }

func caller(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return uuid.Nil, false
	}
	return id.UserID, true
}

func parseQueueOrgAndEvent(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	var orgID uuid.UUID
	if oStr := chi.URLParam(r, "orgId"); oStr != "" {
		parsed, err := uuid.Parse(oStr)
		if err != nil {
			return uuid.Nil, uuid.Nil, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id")
		}
		orgID = parsed
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		return uuid.Nil, uuid.Nil, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id")
	}
	return orgID, eventID, nil
}

func (h *Handler) Join(w http.ResponseWriter, r *http.Request) {
	uid, ok := caller(w, r)
	if !ok {
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	resp, err := h.svc.JoinByEvent(r.Context(), eventID, uid)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	uid, ok := caller(w, r)
	if !ok {
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	if h.limiter != nil {
		allowed, _ := h.limiter.Allow(r.Context(), "status:"+eventID.String()+":"+uid.String(), 5, time.Second)
		if !allowed {
			w.Header().Set("Retry-After", "1")
			apperr.WriteError(w, r, apperr.New(http.StatusTooManyRequests, "TOO_MANY_REQUESTS", "too many status requests, please retry shortly"))
			return
		}
	}
	resp, err := h.svc.Status(r.Context(), eventID, uid)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) Pause(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, err := parseQueueOrgAndEvent(r)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	if err := h.svc.Pause(r.Context(), eventID, orgID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Resume(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, err := parseQueueOrgAndEvent(r)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	if err := h.svc.Resume(r.Context(), eventID, orgID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) SetRate(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, err := parseQueueOrgAndEvent(r)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	var req struct {
		Rate int32 `json:"rate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	if err := h.svc.SetRate(r.Context(), eventID, req.Rate, orgID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) QueueStats(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, err := parseQueueOrgAndEvent(r)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	stats, err := h.svc.Stats(r.Context(), eventID, orgID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, stats)
}

func (h *Handler) SetSchedule(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, err := parseQueueOrgAndEvent(r)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	var req struct {
		Seed          string  `json:"seed"`
		SaleStartAt   *string `json:"saleStartAt"`
		PresaleOpenAt *string `json:"presaleOpenAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	var saleStart, presaleOpen *time.Time
	if req.SaleStartAt != nil {
		t, err := time.Parse(time.RFC3339, *req.SaleStartAt)
		if err != nil {
			apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_SALE_START", "invalid saleStartAt"))
			return
		}
		saleStart = &t
	}
	if req.PresaleOpenAt != nil {
		t, err := time.Parse(time.RFC3339, *req.PresaleOpenAt)
		if err != nil {
			apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_PRESALE_OPEN", "invalid presaleOpenAt"))
			return
		}
		presaleOpen = &t
	}
	if err := h.svc.SetSchedule(r.Context(), eventID, req.Seed, saleStart, presaleOpen, orgID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
