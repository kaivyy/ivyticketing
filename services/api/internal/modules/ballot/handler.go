package ballot

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

func (h *Handler) CreateDraw(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return
	}
	eventID, _ := uuid.Parse(chi.URLParam(r, "eventId"))
	categoryID, _ := uuid.Parse(chi.URLParam(r, "categoryId"))
	orgID, _ := uuid.Parse(chi.URLParam(r, "orgId"))
	var req CreateDrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	draw, err := h.svc.CreateDraw(r.Context(), orgID, eventID, categoryID, actor.UserID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, draw)
}

func (h *Handler) UpdateDraw(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return
	}
	_ = actor
	apperr.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) OpenDraw(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return
	}
	drawID, _ := uuid.Parse(chi.URLParam(r, "drawId"))
	if err := h.svc.OpenDraw(r.Context(), drawID, actor.UserID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CloseDraw(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return
	}
	drawID, _ := uuid.Parse(chi.URLParam(r, "drawId"))
	if err := h.svc.CloseDraw(r.Context(), drawID, actor.UserID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RunDraw(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return
	}
	drawID, _ := uuid.Parse(chi.URLParam(r, "drawId"))
	if err := h.svc.RunDraw(r.Context(), drawID, actor.UserID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AnnounceDraw(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return
	}
	drawID, _ := uuid.Parse(chi.URLParam(r, "drawId"))
	if err := h.svc.AnnounceDraw(r.Context(), drawID, actor.UserID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListResults(w http.ResponseWriter, r *http.Request) {
	drawID, _ := uuid.Parse(chi.URLParam(r, "drawId"))
	limit, offset := int32(50), int32(0)
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
	results, err := h.svc.ListResults(r.Context(), drawID, limit, offset)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, results)
}

func (h *Handler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	drawID, _ := uuid.Parse(chi.URLParam(r, "drawId"))
	data, err := h.svc.ExportResultsCSV(r.Context(), drawID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="ballot-results.csv"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) PromoteWaitlist(w http.ResponseWriter, r *http.Request) {
	drawID, _ := uuid.Parse(chi.URLParam(r, "drawId"))
	if err := h.svc.PromoteWaitlist(r.Context(), drawID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
