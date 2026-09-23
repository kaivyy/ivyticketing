package orders

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

func caller(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return uuid.Nil, false
	}
	return id.UserID, true
}

func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	userID, ok := caller(w, r)
	if !ok {
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	categoryID, err := uuid.Parse(chi.URLParam(r, "categoryId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CATEGORY_ID", "invalid category id"))
		return
	}
	admissionToken := r.Header.Get("X-Queue-Token")
	var body struct {
		FormAnswers map[string]any `json:"formAnswers"`
		Answers     map[string]any `json:"answers"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	formAnswers := body.FormAnswers
	if formAnswers == nil {
		formAnswers = body.Answers
	}
	order, err := h.svc.Checkout(r.Context(), userID, eventID, categoryID, admissionToken, formAnswers)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, order)
}

func (h *Handler) GuestCheckout(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return
	}
	categoryID, err := uuid.Parse(chi.URLParam(r, "categoryId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CATEGORY_ID", "invalid category id"))
		return
	}

	var req GuestCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_PAYLOAD", "invalid json payload"))
		return
	}
	if req.AdmissionToken == "" {
		req.AdmissionToken = r.Header.Get("X-Queue-Token")
	}

	order, err := h.svc.GuestCheckout(r.Context(), req, eventID, categoryID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, order)
}

func (h *Handler) ClaimOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := caller(w, r)
	if !ok {
		return
	}

	var req ClaimOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_PAYLOAD", "invalid json payload"))
		return
	}
	if req.OrderID == uuid.Nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORDER_ID", "order id is required"))
		return
	}

	order, err := h.svc.ClaimOrder(r.Context(), userID, req.OrderID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, order)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := caller(w, r)
	if !ok {
		return
	}
	orders, err := h.svc.ListForParticipant(r.Context(), userID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, orders)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := caller(w, r)
	if !ok {
		return
	}
	orderID, err := uuid.Parse(chi.URLParam(r, "orderId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORDER_ID", "invalid order id"))
		return
	}
	order, err := h.svc.GetForParticipant(r.Context(), userID, orderID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, order)
}

func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	userID, ok := caller(w, r)
	if !ok {
		return
	}
	orderID, err := uuid.Parse(chi.URLParam(r, "orderId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORDER_ID", "invalid order id"))
		return
	}
	if err := h.svc.Cancel(r.Context(), userID, orderID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	orders, err := h.svc.ListForOrgEvent(r.Context(), orgID, eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, orders)
}

func (h *Handler) Refund(w http.ResponseWriter, r *http.Request) {
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
	orderID, err := uuid.Parse(chi.URLParam(r, "orderId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORDER_ID", "invalid order id"))
		return
	}
	var req RefundOrderRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	refunded, err := h.svc.RefundOrder(r.Context(), orgID, eventID, orderID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, refunded)
}
