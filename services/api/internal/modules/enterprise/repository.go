package enterprise

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// Repository is the data-access surface for the enterprise module: API keys,
// webhook endpoints, and the idempotent outbound-delivery ledger.
type Repository interface {
	// API keys.
	CreateAPIKey(ctx context.Context, arg db.CreateAPIKeyParams) (db.ApiKey, error)
	ListAPIKeysByOrg(ctx context.Context, orgID uuid.UUID) ([]db.ApiKey, error)
	GetAPIKeyByHash(ctx context.Context, keyHash string) (db.ApiKey, error)
	RevokeAPIKey(ctx context.Context, id, orgID uuid.UUID) (db.ApiKey, error)
	TouchAPIKey(ctx context.Context, id uuid.UUID) error

	// Webhook endpoints.
	CreateWebhookEndpoint(ctx context.Context, arg db.CreateWebhookEndpointParams) (db.WebhookEndpoint, error)
	ListWebhookEndpointsByOrg(ctx context.Context, orgID uuid.UUID) ([]db.WebhookEndpoint, error)
	ListActiveWebhookEndpointsForEvent(ctx context.Context, orgID uuid.UUID, eventType string) ([]db.WebhookEndpoint, error)
	GetWebhookEndpointByID(ctx context.Context, id uuid.UUID) (db.WebhookEndpoint, error)
	DeleteWebhookEndpoint(ctx context.Context, id, orgID uuid.UUID) error

	// Delivery ledger.
	EnqueueWebhookDelivery(ctx context.Context, arg db.EnqueueWebhookDeliveryParams) (db.WebhookDelivery, error)
	ListDueWebhookDeliveries(ctx context.Context, limit int32) ([]db.WebhookDelivery, error)
	MarkWebhookDelivered(ctx context.Context, id uuid.UUID) error
	MarkWebhookRetry(ctx context.Context, arg db.MarkWebhookRetryParams) error
	ListWebhookDeliveriesByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]db.WebhookDelivery, error)

	// Public read APIs (org-scoped; callers must enforce org ownership on
	// single-resource lookups since the GetByID queries are not org-filtered).
	GetEvent(ctx context.Context, id uuid.UUID) (db.Event, error)
	ListEventsByOrg(ctx context.Context, orgID uuid.UUID) ([]db.Event, error)
	GetOrder(ctx context.Context, id uuid.UUID) (db.Order, error)
	ListOrdersByOrgEvent(ctx context.Context, orgID, eventID uuid.UUID) ([]db.Order, error)
	GetPayment(ctx context.Context, id uuid.UUID) (db.Payment, error)
}

type sqlcRepo struct{ q *db.Queries }

// NewRepository constructs a Repository backed by the sqlc-generated Queries.
func NewRepository(pool *pgxpool.Pool) Repository { return &sqlcRepo{q: db.New(pool)} }

// --- API keys ---

func (r *sqlcRepo) CreateAPIKey(ctx context.Context, arg db.CreateAPIKeyParams) (db.ApiKey, error) {
	return r.q.CreateAPIKey(ctx, arg)
}
func (r *sqlcRepo) ListAPIKeysByOrg(ctx context.Context, orgID uuid.UUID) ([]db.ApiKey, error) {
	return r.q.ListAPIKeysByOrg(ctx, orgID)
}
func (r *sqlcRepo) GetAPIKeyByHash(ctx context.Context, keyHash string) (db.ApiKey, error) {
	return r.q.GetAPIKeyByHash(ctx, keyHash)
}
func (r *sqlcRepo) RevokeAPIKey(ctx context.Context, id, orgID uuid.UUID) (db.ApiKey, error) {
	return r.q.RevokeAPIKey(ctx, db.RevokeAPIKeyParams{ID: id, OrganizationID: orgID})
}
func (r *sqlcRepo) TouchAPIKey(ctx context.Context, id uuid.UUID) error {
	return r.q.TouchAPIKey(ctx, id)
}

// --- webhook endpoints ---

func (r *sqlcRepo) CreateWebhookEndpoint(ctx context.Context, arg db.CreateWebhookEndpointParams) (db.WebhookEndpoint, error) {
	return r.q.CreateWebhookEndpoint(ctx, arg)
}
func (r *sqlcRepo) ListWebhookEndpointsByOrg(ctx context.Context, orgID uuid.UUID) ([]db.WebhookEndpoint, error) {
	return r.q.ListWebhookEndpointsByOrg(ctx, orgID)
}
func (r *sqlcRepo) ListActiveWebhookEndpointsForEvent(ctx context.Context, orgID uuid.UUID, eventType string) ([]db.WebhookEndpoint, error) {
	return r.q.ListActiveWebhookEndpointsForEvent(ctx, db.ListActiveWebhookEndpointsForEventParams{
		OrganizationID: orgID,
		EventType:      eventType,
	})
}
func (r *sqlcRepo) GetWebhookEndpointByID(ctx context.Context, id uuid.UUID) (db.WebhookEndpoint, error) {
	return r.q.GetWebhookEndpointByID(ctx, id)
}
func (r *sqlcRepo) DeleteWebhookEndpoint(ctx context.Context, id, orgID uuid.UUID) error {
	return r.q.DeleteWebhookEndpoint(ctx, db.DeleteWebhookEndpointParams{ID: id, OrganizationID: orgID})
}

// --- delivery ledger ---

func (r *sqlcRepo) EnqueueWebhookDelivery(ctx context.Context, arg db.EnqueueWebhookDeliveryParams) (db.WebhookDelivery, error) {
	return r.q.EnqueueWebhookDelivery(ctx, arg)
}
func (r *sqlcRepo) ListDueWebhookDeliveries(ctx context.Context, limit int32) ([]db.WebhookDelivery, error) {
	return r.q.ListDueWebhookDeliveries(ctx, limit)
}
func (r *sqlcRepo) MarkWebhookDelivered(ctx context.Context, id uuid.UUID) error {
	return r.q.MarkWebhookDelivered(ctx, id)
}
func (r *sqlcRepo) MarkWebhookRetry(ctx context.Context, arg db.MarkWebhookRetryParams) error {
	return r.q.MarkWebhookRetry(ctx, arg)
}
func (r *sqlcRepo) ListWebhookDeliveriesByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int32) ([]db.WebhookDelivery, error) {
	return r.q.ListWebhookDeliveriesByOrg(ctx, db.ListWebhookDeliveriesByOrgParams{
		OrganizationID: orgID,
		Limit:          limit,
		Offset:         offset,
	})
}

// --- public read APIs ---

func (r *sqlcRepo) GetEvent(ctx context.Context, id uuid.UUID) (db.Event, error) {
	return r.q.GetEventByID(ctx, id)
}
func (r *sqlcRepo) ListEventsByOrg(ctx context.Context, orgID uuid.UUID) ([]db.Event, error) {
	return r.q.ListEventsByOrg(ctx, orgID)
}
func (r *sqlcRepo) GetOrder(ctx context.Context, id uuid.UUID) (db.Order, error) {
	return r.q.GetOrderByID(ctx, id)
}
func (r *sqlcRepo) ListOrdersByOrgEvent(ctx context.Context, orgID, eventID uuid.UUID) ([]db.Order, error) {
	return r.q.ListOrdersByOrgEvent(ctx, db.ListOrdersByOrgEventParams{
		OrganizationID: orgID,
		EventID:        eventID,
	})
}
func (r *sqlcRepo) GetPayment(ctx context.Context, id uuid.UUID) (db.Payment, error) {
	return r.q.GetPaymentByID(ctx, id)
}

// pgText wraps a string into a valid pgtype.Text.
func pgText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }
