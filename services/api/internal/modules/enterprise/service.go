package enterprise

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

// Service holds the enterprise business logic: API-key lifecycle, webhook
// subscription management, and idempotent outbound delivery enqueue.
type Service struct {
	repo  Repository
	audit *audit.Logger
	log   *slog.Logger
}

// NewService constructs a Service.
func NewService(repo Repository, auditLog *audit.Logger, log *slog.Logger) *Service {
	return &Service{repo: repo, audit: auditLog, log: log}
}

// --- API keys ---

// CreateAPIKey mints a new key for the org. The raw key is returned exactly
// once (on the view's RawKey field); only its SHA-256 hash is persisted.
func (s *Service) CreateAPIKey(ctx context.Context, actor, orgID uuid.UUID, req CreateAPIKeyRequest) (APIKeyView, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return APIKeyView{}, ErrInvalidPayload
	}
	scopes := sanitizeScopes(req.Scopes)
	rate := req.RateLimitPerMin
	if rate <= 0 {
		rate = 120
	}
	if rate > 10000 {
		rate = 10000
	}

	raw, err := generateKey()
	if err != nil {
		return APIKeyView{}, err
	}
	k, err := s.repo.CreateAPIKey(ctx, db.CreateAPIKeyParams{
		OrganizationID:  orgID,
		Name:            name,
		KeyPrefix:       prefixOf(raw),
		KeyHash:         hashKey(raw),
		Scopes:          encodeScopes(scopes),
		RateLimitPerMin: int32(rate),
	})
	if err != nil {
		return APIKeyView{}, err
	}
	s.record(ctx, actor, orgID, "APIKEY_CREATED", "api_key", k.ID.String(), map[string]any{"name": name, "scopes": scopes})

	view := toAPIKeyView(k)
	view.RawKey = raw // shown once
	return view, nil
}

// ListAPIKeys returns the org's keys (never the hash or raw secret).
func (s *Service) ListAPIKeys(ctx context.Context, orgID uuid.UUID) ([]APIKeyView, error) {
	keys, err := s.repo.ListAPIKeysByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]APIKeyView, 0, len(keys))
	for _, k := range keys {
		out = append(out, toAPIKeyView(k))
	}
	return out, nil
}

// RevokeAPIKey soft-revokes a key (sets revoked_at). Idempotent: revoking an
// already-revoked or unknown key returns ErrAPIKeyNotFound.
func (s *Service) RevokeAPIKey(ctx context.Context, actor, orgID, keyID uuid.UUID) error {
	_, err := s.repo.RevokeAPIKey(ctx, keyID, orgID)
	if err != nil {
		return ErrAPIKeyNotFound
	}
	s.record(ctx, actor, orgID, "APIKEY_REVOKED", "api_key", keyID.String(), nil)
	return nil
}

// Authenticate resolves a raw API key to its stored record. Returns
// ErrInvalidAPIKey when the key is malformed, unknown, or revoked. The
// constant-time compare guards against a timing side-channel on the hash.
func (s *Service) Authenticate(ctx context.Context, raw string) (db.ApiKey, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "ivyk_") {
		return db.ApiKey{}, ErrInvalidAPIKey
	}
	hash := hashKey(raw)
	k, err := s.repo.GetAPIKeyByHash(ctx, hash)
	if err != nil {
		return db.ApiKey{}, ErrInvalidAPIKey
	}
	if !constantTimeEqual(hash, k.KeyHash) {
		return db.ApiKey{}, ErrInvalidAPIKey
	}
	// Best-effort last-used stamp; never block the request on it.
	go func() { _ = s.repo.TouchAPIKey(context.WithoutCancel(ctx), k.ID) }()
	return k, nil
}

// --- webhook endpoints ---

// CreateWebhook registers an outbound webhook subscription and returns the view
// with the signing secret (shown once so the organizer can configure HMAC
// verification on their receiver).
func (s *Service) CreateWebhook(ctx context.Context, actor, orgID uuid.UUID, req CreateWebhookRequest) (WebhookView, error) {
	u := strings.TrimSpace(req.URL)
	if !isValidHTTPSURL(u) {
		return WebhookView{}, ErrInvalidWebhookURL
	}
	events := sanitizeScopes(req.Events)
	if len(events) == 0 {
		return WebhookView{}, ErrInvalidPayload
	}
	secret, err := generateKey()
	if err != nil {
		return WebhookView{}, err
	}
	e, err := s.repo.CreateWebhookEndpoint(ctx, db.CreateWebhookEndpointParams{
		OrganizationID: orgID,
		Url:            u,
		Secret:         secret,
		Events:         encodeScopes(events),
		IsActive:       true,
	})
	if err != nil {
		return WebhookView{}, err
	}
	s.record(ctx, actor, orgID, "WEBHOOK_CREATED", "webhook_endpoint", e.ID.String(), map[string]any{"url": u, "events": events})

	view := toWebhookView(e)
	view.Secret = secret // shown once
	return view, nil
}

// ListWebhooks returns the org's webhook endpoints (never the secret).
func (s *Service) ListWebhooks(ctx context.Context, orgID uuid.UUID) ([]WebhookView, error) {
	eps, err := s.repo.ListWebhookEndpointsByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]WebhookView, 0, len(eps))
	for _, e := range eps {
		out = append(out, toWebhookView(e))
	}
	return out, nil
}

// DeleteWebhook removes an endpoint (and cascades its deliveries).
func (s *Service) DeleteWebhook(ctx context.Context, actor, orgID, id uuid.UUID) error {
	if err := s.repo.DeleteWebhookEndpoint(ctx, id, orgID); err != nil {
		return ErrWebhookNotFound
	}
	s.record(ctx, actor, orgID, "WEBHOOK_DELETED", "webhook_endpoint", id.String(), nil)
	return nil
}

// ListDeliveries returns the org's delivery ledger for observability.
func (s *Service) ListDeliveries(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]WebhookDeliveryView, error) {
	rows, err := s.repo.ListWebhookDeliveriesByOrg(ctx, orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]WebhookDeliveryView, 0, len(rows))
	for _, d := range rows {
		out = append(out, toDeliveryView(d))
	}
	return out, nil
}

// Emit is the fan-out entry point called by other modules when a business event
// occurs (e.g. order.paid). It enqueues one delivery row per active endpoint
// subscribed to eventType. Idempotency: eventKey should be stable for the
// business event (e.g. "order.paid:<orderID>"); the UNIQUE(endpoint_id,
// event_key) constraint makes a duplicate Emit a no-op per endpoint.
//
// Never returns an error to the caller's critical path — outbound integration
// must not break core flows. Failures are logged and left for the dispatcher.
func (s *Service) Emit(ctx context.Context, orgID uuid.UUID, eventType, eventKey string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		s.logWarn("webhook emit marshal failed", "event", eventType, "err", err)
		return
	}
	eps, err := s.repo.ListActiveWebhookEndpointsForEvent(ctx, orgID, eventType)
	if err != nil {
		s.logWarn("webhook emit list endpoints failed", "event", eventType, "err", err)
		return
	}
	for _, e := range eps {
		_, err := s.repo.EnqueueWebhookDelivery(ctx, db.EnqueueWebhookDeliveryParams{
			EndpointID:     e.ID,
			OrganizationID: orgID,
			EventType:      eventType,
			EventKey:       eventKey,
			Payload:        body,
		})
		if err != nil {
			s.logWarn("webhook enqueue failed", "endpoint", e.ID, "event", eventType, "err", err)
		}
	}
}

// --- helpers ---

func (s *Service) record(ctx context.Context, actor, orgID uuid.UUID, action, targetType, targetID string, meta map[string]any) {
	if s.audit == nil {
		return
	}
	a, o := actor, orgID
	s.audit.Record(ctx, audit.Entry{
		OrganizationID: &o,
		ActorUserID:    &a,
		Action:         action,
		TargetType:     targetType,
		TargetID:       targetID,
		Metadata:       meta,
	})
}

func (s *Service) logWarn(msg string, args ...any) {
	if s.log != nil {
		s.log.Warn(msg, args...)
	}
}

// sanitizeScopes trims, lowercases, drops blanks, and de-dupes a scope/event list.
func sanitizeScopes(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		v := strings.ToLower(strings.TrimSpace(s))
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// isValidHTTPSURL requires an absolute https URL with a host. Enforcing https
// keeps webhook payloads (which may carry PII) encrypted in transit.
func isValidHTTPSURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return u.Scheme == "https" && u.Host != ""
}
