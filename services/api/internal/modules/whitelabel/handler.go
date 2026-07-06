package whitelabel

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// Handler is the HTTP entry point for the white-label module.
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

func parseDomainID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "domainId"))
	if err != nil {
		apperr.WriteError(w, r, ErrDomainNotFound)
		return uuid.Nil, false
	}
	return id, true
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

// --- branding ---

// GetBranding returns the org's branding overrides.
// GET /organizations/{orgId}/branding
func (h *Handler) GetBranding(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	b, err := h.svc.GetBranding(r.Context(), orgID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, b)
}

// UpsertBranding sets the org's branding overrides.
// PUT /organizations/{orgId}/branding
func (h *Handler) UpsertBranding(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r)
	if !ok {
		return
	}
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	var body UpsertBrandingRequest
	if !decode(w, r, &body) {
		return
	}
	b, err := h.svc.UpsertBranding(r.Context(), actor, orgID, body)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, b)
}

// --- custom domains ---

// ListDomains returns the org's custom domains.
// GET /organizations/{orgId}/branding/domains
func (h *Handler) ListDomains(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	domains, err := h.svc.ListDomains(r.Context(), orgID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, domains)
}

// AddDomain registers a custom domain.
// POST /organizations/{orgId}/branding/domains
func (h *Handler) AddDomain(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r)
	if !ok {
		return
	}
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	var body AddDomainRequest
	if !decode(w, r, &body) {
		return
	}
	d, err := h.svc.AddDomain(r.Context(), actor, orgID, body)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, d)
}

// VerifyDomain checks DNS TXT and transitions the domain state.
// POST /organizations/{orgId}/branding/domains/{domainId}/verify
func (h *Handler) VerifyDomain(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r)
	if !ok {
		return
	}
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	domainID, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	d, err := h.svc.VerifyDomain(r.Context(), actor, orgID, domainID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, d)
}

// DeleteDomain removes a custom domain.
// DELETE /organizations/{orgId}/branding/domains/{domainId}
func (h *Handler) DeleteDomain(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r)
	if !ok {
		return
	}
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	domainID, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteDomain(r.Context(), actor, orgID, domainID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
