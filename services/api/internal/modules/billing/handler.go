package billing

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// Handler is the HTTP entry point for the billing module.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func parseOrg(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	orgID, err := uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, ErrInvalidOrgID)
		return uuid.Nil, false
	}
	return orgID, true
}

func actorID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "not authenticated"))
		return uuid.Nil, false
	}
	return id.UserID, true
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(dst); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body"))
		return false
	}
	return true
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrPackageNotFound),
		errors.Is(err, ErrSubscriptionNotFound),
		errors.Is(err, ErrInvoiceNotFound),
		errors.Is(err, ErrInvalidPackage),
		errors.Is(err, ErrInvalidOrgID),
		errors.Is(err, ErrEventLimitReached):
		apperr.WriteError(w, r, err)
	default:
		apperr.WriteError(w, r, err)
	}
}

func pagination(r *http.Request) (int32, int32) {
	limit, offset := int32(50), int32(0)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
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

// --- super-admin: packages ---

// ListPackages returns the full package catalog. GET /admin/billing/packages
func (h *Handler) ListPackages(w http.ResponseWriter, r *http.Request) {
	pkgs, err := h.svc.ListPackages(r.Context(), false)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, pkgs)
}

// CreatePackage adds a package. POST /admin/billing/packages
func (h *Handler) CreatePackage(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r)
	if !ok {
		return
	}
	var body UpsertPackageRequest
	if !decode(w, r, &body) {
		return
	}
	pkg, err := h.svc.CreatePackage(r.Context(), actor, body)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, pkg)
}

// UpdatePackage edits a package. PUT /admin/billing/packages/{packageId}
func (h *Handler) UpdatePackage(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "packageId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_PACKAGE_ID", "invalid package id"))
		return
	}
	var body UpsertPackageRequest
	if !decode(w, r, &body) {
		return
	}
	pkg, err := h.svc.UpdatePackage(r.Context(), actor, id, body)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, pkg)
}

// --- super-admin: subscriptions + revenue + invoices ---

// AssignSubscription assigns/upgrades an org. PUT /admin/billing/organizations/{orgId}/subscription
func (h *Handler) AssignSubscription(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r)
	if !ok {
		return
	}
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	var body AssignSubscriptionRequest
	if !decode(w, r, &body) {
		return
	}
	sub, err := h.svc.AssignSubscription(r.Context(), actor, orgID, body)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, sub)
}

// PlatformRevenue returns cross-org fee aggregate. GET /admin/billing/revenue
func (h *Handler) PlatformRevenue(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.PlatformRevenue(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, rows)
}

// GenerateInvoice issues an invoice. POST /admin/billing/organizations/{orgId}/invoices
func (h *Handler) GenerateInvoice(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r)
	if !ok {
		return
	}
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	var body GenerateInvoiceRequest
	if !decode(w, r, &body) {
		return
	}
	inv, err := h.svc.GenerateInvoice(r.Context(), actor, orgID, body)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, inv)
}

// MarkInvoicePaid transitions an invoice to PAID. POST /admin/billing/invoices/{invoiceId}/paid
func (h *Handler) MarkInvoicePaid(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "invoiceId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_INVOICE_ID", "invalid invoice id"))
		return
	}
	inv, err := h.svc.MarkInvoicePaid(r.Context(), actor, id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, inv)
}

// --- organizer: own subscription, packages, invoices, fees ---

// GetSubscription returns the org's current subscription.
// GET /organizations/{orgId}/billing/subscription
func (h *Handler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	sub, err := h.svc.GetSubscription(r.Context(), orgID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, sub)
}

// ListActivePackages returns the upgrade catalog.
// GET /organizations/{orgId}/billing/packages
func (h *Handler) ListActivePackages(w http.ResponseWriter, r *http.Request) {
	if _, ok := parseOrg(w, r); !ok {
		return
	}
	pkgs, err := h.svc.ListPackages(r.Context(), true)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, pkgs)
}

// FeeSummary returns the org's platform-fee aggregate.
// GET /organizations/{orgId}/billing/fees/summary
func (h *Handler) FeeSummary(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	summary, err := h.svc.FeeSummary(r.Context(), orgID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, summary)
}

// ListInvoices returns the org's invoices.
// GET /organizations/{orgId}/billing/invoices
func (h *Handler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	limit, offset := pagination(r)
	invs, err := h.svc.ListInvoices(r.Context(), orgID, limit, offset)
	if err != nil {
		writeError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, invs)
}
