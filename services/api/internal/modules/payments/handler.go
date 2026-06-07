package payments

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
	rec *Reconciler
}

func NewHandler(svc *Service, rec *Reconciler) *Handler { return &Handler{svc: svc, rec: rec} }

func participantID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return uuid.Nil, false
	}
	return id.UserID, true
}

func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	uid, ok := participantID(w, r)
	if !ok {
		return
	}
	orderID, err := uuid.Parse(chi.URLParam(r, "orderId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORDER_ID", "invalid order id"))
		return
	}
	var req CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	resp, err := h.svc.CreatePayment(r.Context(), uid, orderID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) ListByOrder(w http.ResponseWriter, r *http.Request) {
	uid, ok := participantID(w, r)
	if !ok {
		return
	}
	orderID, err := uuid.Parse(chi.URLParam(r, "orderId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORDER_ID", "invalid order id"))
		return
	}
	list, err := h.svc.ListByOrder(r.Context(), uid, orderID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, list)
}

func (h *Handler) GetMine(w http.ResponseWriter, r *http.Request) {
	uid, ok := participantID(w, r)
	if !ok {
		return
	}
	paymentID, err := uuid.Parse(chi.URLParam(r, "paymentId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_PAYMENT_ID", "invalid payment id"))
		return
	}
	resp, err := h.svc.GetForParticipant(r.Context(), uid, paymentID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) ListByOrgEvent(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid org id"))
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	list, err := h.svc.ListForOrgEvent(r.Context(), orgID, eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, list)
}

func (h *Handler) Reconcile(w http.ResponseWriter, r *http.Request) {
	paymentID, err := uuid.Parse(chi.URLParam(r, "paymentId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_PAYMENT_ID", "invalid payment id"))
		return
	}
	if err := h.rec.Reconcile(r.Context(), paymentID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
