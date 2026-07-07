package enterprise

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// --- API keys ---

// CreateAPIKeyRequest is the organizer payload for minting a new key.
type CreateAPIKeyRequest struct {
	Name            string   `json:"name"`
	Scopes          []string `json:"scopes"`
	RateLimitPerMin int      `json:"rateLimitPerMin"`
}

// APIKeyView is the safe representation of a key: never includes key_hash and
// only the raw secret on the create response (RawKey), never on list.
type APIKeyView struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	KeyPrefix       string     `json:"keyPrefix"`
	Scopes          []string   `json:"scopes"`
	RateLimitPerMin int        `json:"rateLimitPerMin"`
	LastUsedAt      *time.Time `json:"lastUsedAt,omitempty"`
	RevokedAt       *time.Time `json:"revokedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	// RawKey is set only on the create response — the one and only time the
	// plaintext key is ever returned. Omitted (empty) everywhere else.
	RawKey string `json:"rawKey,omitempty"`
}

func toAPIKeyView(k db.ApiKey) APIKeyView {
	v := APIKeyView{
		ID:              k.ID,
		Name:            k.Name,
		KeyPrefix:       k.KeyPrefix,
		Scopes:          decodeScopes(k.Scopes),
		RateLimitPerMin: int(k.RateLimitPerMin),
		CreatedAt:       k.CreatedAt.Time,
	}
	if k.LastUsedAt.Valid {
		t := k.LastUsedAt.Time
		v.LastUsedAt = &t
	}
	if k.RevokedAt.Valid {
		t := k.RevokedAt.Time
		v.RevokedAt = &t
	}
	return v
}

// --- webhook endpoints ---

// CreateWebhookRequest is the organizer payload for registering an outbound
// webhook subscription.
type CreateWebhookRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

// WebhookView is the safe representation of an endpoint. The signing secret is
// returned only on create (so the organizer can configure verification) and
// never on list.
type WebhookView struct {
	ID        uuid.UUID `json:"id"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	IsActive  bool      `json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
	Secret    string    `json:"secret,omitempty"`
}

func toWebhookView(e db.WebhookEndpoint) WebhookView {
	return WebhookView{
		ID:        e.ID,
		URL:       e.Url,
		Events:    decodeScopes(e.Events),
		IsActive:  e.IsActive,
		CreatedAt: e.CreatedAt.Time,
	}
}

// WebhookDeliveryView surfaces the delivery ledger for observability.
type WebhookDeliveryView struct {
	ID            uuid.UUID  `json:"id"`
	EndpointID    uuid.UUID  `json:"endpointId"`
	EventType     string     `json:"eventType"`
	EventKey      string     `json:"eventKey"`
	Status        string     `json:"status"`
	Attempts      int        `json:"attempts"`
	LastError     string     `json:"lastError,omitempty"`
	NextAttemptAt time.Time  `json:"nextAttemptAt"`
	DeliveredAt   *time.Time `json:"deliveredAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

func toDeliveryView(d db.WebhookDelivery) WebhookDeliveryView {
	v := WebhookDeliveryView{
		ID:            d.ID,
		EndpointID:    d.EndpointID,
		EventType:     d.EventType,
		EventKey:      d.EventKey,
		Status:        d.Status,
		Attempts:      int(d.Attempts),
		NextAttemptAt: d.NextAttemptAt.Time,
		CreatedAt:     d.CreatedAt.Time,
	}
	if d.LastError.Valid {
		v.LastError = d.LastError.String
	}
	if d.DeliveredAt.Valid {
		t := d.DeliveredAt.Time
		v.DeliveredAt = &t
	}
	return v
}

// decodeScopes unmarshals a jsonb string array, tolerating NULL/empty as [].
func decodeScopes(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return []string{}
	}
	return out
}

// encodeScopes marshals a string slice to jsonb bytes, never nil.
func encodeScopes(scopes []string) []byte {
	if scopes == nil {
		scopes = []string{}
	}
	b, _ := json.Marshal(scopes)
	return b
}
