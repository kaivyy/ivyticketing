package abuse

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func actor(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return uuid.Nil, false
	}
	return id.UserID, true
}

func pageParams(r *http.Request) (int32, int32) {
	limit := int32(50)
	offset := int32(0)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = int32(n)
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = int32(n)
		}
	}
	return limit, offset
}

func (h *Handler) ListSettings(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListSettings(r.Context())
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) SetSetting(w http.ResponseWriter, r *http.Request) {
	uid, ok := actor(w, r)
	if !ok {
		return
	}
	var req SettingDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	if err := h.svc.SetSetting(r.Context(), uid, req.Key, req.Value); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Block(w http.ResponseWriter, r *http.Request) {
	uid, ok := actor(w, r)
	if !ok {
		return
	}
	var req BlockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	if err := h.svc.Block(r.Context(), uid, req); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Unblock(w http.ResponseWriter, r *http.Request) {
	uid, ok := actor(w, r)
	if !ok {
		return
	}
	var req UnblockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	if err := h.svc.Unblock(r.Context(), uid, req); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListBlocked(w http.ResponseWriter, r *http.Request) {
	limit, offset := pageParams(r)
	out, err := h.svc.ListBlocked(r.Context(), limit, offset)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) ListLog(w http.ResponseWriter, r *http.Request) {
	limit, offset := pageParams(r)
	out, err := h.svc.ListAbuseLog(r.Context(), limit, offset)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) ListIPRules(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListIPRules(r.Context())
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) AddIPRule(w http.ResponseWriter, r *http.Request) {
	uid, ok := actor(w, r)
	if !ok {
		return
	}
	var req IPRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	if err := h.svc.AddIPRule(r.Context(), uid, req); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeleteIPRule(w http.ResponseWriter, r *http.Request) {
	uid, ok := actor(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "ruleId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_RULE_ID", "invalid rule id"))
		return
	}
	if err := h.svc.DeleteIPRule(r.Context(), uid, id); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
