package publiccatalog

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	orgSlug := chi.URLParam(r, "orgSlug")
	out, err := h.svc.ListEvents(r.Context(), orgSlug)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) GetEvent(w http.ResponseWriter, r *http.Request) {
	orgSlug := chi.URLParam(r, "orgSlug")
	eventSlug := chi.URLParam(r, "eventSlug")
	ev, err := h.svc.GetEvent(r.Context(), orgSlug, eventSlug)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, ev)
}
