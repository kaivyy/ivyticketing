package enterprise

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// Handler is the HTTP entry point for organizer-facing enterprise management:
// API-key lifecycle and webhook subscriptions, gated on apikey.manage.
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
		apperr.WriteError(w, r, ErrInvalidPayload)
		return false
	}
	return true
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

// --- API keys ---

// ListAPIKeys returns the org's keys (never the hash or raw secret).
// GET /organizations/{orgId}/enterprise/api-keys
func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	keys, err := h.svc.ListAPIKeys(r.Context(), orgID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, keys)
}

// CreateAPIKey mints a new key. The raw secret is returned exactly once.
// POST /organizations/{orgId}/enterprise/api-keys
func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r)
	if !ok {
		return
	}
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	var body CreateAPIKeyRequest
	if !decode(w, r, &body) {
		return
	}
	key, err := h.svc.CreateAPIKey(r.Context(), actor, orgID, body)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, key)
}

// RevokeAPIKey soft-revokes a key.
// DELETE /organizations/{orgId}/enterprise/api-keys/{keyId}
func (h *Handler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r)
	if !ok {
		return
	}
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	keyID, err := uuid.Parse(chi.URLParam(r, "keyId"))
	if err != nil {
		apperr.WriteError(w, r, ErrAPIKeyNotFound)
		return
	}
	if err := h.svc.RevokeAPIKey(r.Context(), actor, orgID, keyID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- webhooks ---

// ListWebhooks returns the org's webhook endpoints (never the secret).
// GET /organizations/{orgId}/enterprise/webhooks
func (h *Handler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	hooks, err := h.svc.ListWebhooks(r.Context(), orgID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, hooks)
}

// CreateWebhook registers an endpoint. The signing secret is returned once.
// POST /organizations/{orgId}/enterprise/webhooks
func (h *Handler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r)
	if !ok {
		return
	}
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	var body CreateWebhookRequest
	if !decode(w, r, &body) {
		return
	}
	hook, err := h.svc.CreateWebhook(r.Context(), actor, orgID, body)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, hook)
}

// DeleteWebhook removes an endpoint (cascading its deliveries).
// DELETE /organizations/{orgId}/enterprise/webhooks/{webhookId}
func (h *Handler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorID(w, r)
	if !ok {
		return
	}
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "webhookId"))
	if err != nil {
		apperr.WriteError(w, r, ErrWebhookNotFound)
		return
	}
	if err := h.svc.DeleteWebhook(r.Context(), actor, orgID, id); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListDeliveries returns the delivery ledger for observability.
// GET /organizations/{orgId}/enterprise/webhooks/deliveries
func (h *Handler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	orgID, ok := parseOrg(w, r)
	if !ok {
		return
	}
	limit, offset := pagination(r)
	rows, err := h.svc.ListDeliveries(r.Context(), orgID, limit, offset)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, rows)
}
