package enterprise

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/db"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

// PublicAPI serves the versioned, API-key-authenticated read endpoints for
// enterprise integrators. Every handler scopes its reads to the API key's org
// (from APIContext) so a key can never read another organization's data.
type PublicAPI struct {
	repo Repository
}

// NewPublicAPI constructs the public read API handler.
func NewPublicAPI(repo Repository) *PublicAPI { return &PublicAPI{repo: repo} }

// --- public-safe DTOs ---
// These deliberately omit internal columns (object keys, gateway refs, merchant
// refs, instructions blobs) so the public contract stays stable and leak-free.

type publicEvent struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description string     `json:"description,omitempty"`
	EventType   string     `json:"eventType"`
	Status      string     `json:"status"`
	VenueName   string     `json:"venueName,omitempty"`
	StartsAt    *time.Time `json:"startsAt,omitempty"`
	EndsAt      *time.Time `json:"endsAt,omitempty"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
}

type publicOrder struct {
	ID          uuid.UUID `json:"id"`
	OrderNumber string    `json:"orderNumber"`
	EventID     uuid.UUID `json:"eventId"`
	Status      string    `json:"status"`
	Subtotal    int64     `json:"subtotal"`
	Fee         int64     `json:"fee"`
	Discount    int64     `json:"discount"`
	Total       int64     `json:"total"`
	CreatedAt   time.Time `json:"createdAt"`
}

type publicPayment struct {
	ID        uuid.UUID  `json:"id"`
	OrderID   uuid.UUID  `json:"orderId"`
	Status    string     `json:"status"`
	Method    string     `json:"method"`
	Amount    int64      `json:"amount"`
	Currency  string     `json:"currency"`
	PaidAt    *time.Time `json:"paidAt,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

func toPublicEvent(e db.Event) publicEvent {
	v := publicEvent{
		ID:        e.ID,
		Name:      e.Name,
		Slug:      e.Slug,
		EventType: e.EventType,
		Status:    e.Status,
	}
	if e.Description.Valid {
		v.Description = e.Description.String
	}
	if e.VenueName.Valid {
		v.VenueName = e.VenueName.String
	}
	if e.StartsAt.Valid {
		t := e.StartsAt.Time
		v.StartsAt = &t
	}
	if e.EndsAt.Valid {
		t := e.EndsAt.Time
		v.EndsAt = &t
	}
	if e.PublishedAt.Valid {
		t := e.PublishedAt.Time
		v.PublishedAt = &t
	}
	return v
}

func toPublicOrder(o db.Order) publicOrder {
	return publicOrder{
		ID:          o.ID,
		OrderNumber: o.OrderNumber,
		EventID:     o.EventID,
		Status:      o.Status,
		Subtotal:    o.Subtotal,
		Fee:         o.Fee,
		Discount:    o.Discount,
		Total:       o.Total,
		CreatedAt:   o.CreatedAt.Time,
	}
}

func toPublicPayment(p db.Payment) publicPayment {
	v := publicPayment{
		ID:        p.ID,
		OrderID:   p.OrderID,
		Status:    p.Status,
		Method:    p.Method,
		Amount:    p.Amount,
		Currency:  p.Currency,
		CreatedAt: p.CreatedAt.Time,
	}
	if p.PaidAt.Valid {
		t := p.PaidAt.Time
		v.PaidAt = &t
	}
	if p.ExpiresAt.Valid {
		t := p.ExpiresAt.Time
		v.ExpiresAt = &t
	}
	return v
}

// --- handlers ---

// ListEvents returns the calling org's events. GET /public/v1/events
func (a *PublicAPI) ListEvents(w http.ResponseWriter, r *http.Request) {
	ac, ok := APIContextFrom(r.Context())
	if !ok {
		apperr.WriteError(w, r, ErrInvalidAPIKey)
		return
	}
	events, err := a.repo.ListEventsByOrg(r.Context(), ac.OrgID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	out := make([]publicEvent, 0, len(events))
	for _, e := range events {
		out = append(out, toPublicEvent(e))
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

// GetEvent returns one event by ID, scoped to the calling org.
// GET /public/v1/events/{eventId}
func (a *PublicAPI) GetEvent(w http.ResponseWriter, r *http.Request) {
	ac, ok := APIContextFrom(r.Context())
	if !ok {
		apperr.WriteError(w, r, ErrInvalidAPIKey)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, ErrInvalidPayload)
		return
	}
	e, err := a.repo.GetEvent(r.Context(), id)
	if err != nil || e.OrganizationID != ac.OrgID {
		apperr.WriteError(w, r, ErrResourceNotFound)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, toPublicEvent(e))
}

// ListOrders returns an event's orders, scoped to the calling org.
// GET /public/v1/events/{eventId}/orders
func (a *PublicAPI) ListOrders(w http.ResponseWriter, r *http.Request) {
	ac, ok := APIContextFrom(r.Context())
	if !ok {
		apperr.WriteError(w, r, ErrInvalidAPIKey)
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, ErrInvalidPayload)
		return
	}
	orders, err := a.repo.ListOrdersByOrgEvent(r.Context(), ac.OrgID, eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	out := make([]publicOrder, 0, len(orders))
	for _, o := range orders {
		out = append(out, toPublicOrder(o))
	}
	apperr.WriteJSON(w, http.StatusOK, out)
}

// GetOrder returns one order by ID, scoped to the calling org.
// GET /public/v1/orders/{orderId}
func (a *PublicAPI) GetOrder(w http.ResponseWriter, r *http.Request) {
	ac, ok := APIContextFrom(r.Context())
	if !ok {
		apperr.WriteError(w, r, ErrInvalidAPIKey)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "orderId"))
	if err != nil {
		apperr.WriteError(w, r, ErrInvalidPayload)
		return
	}
	o, err := a.repo.GetOrder(r.Context(), id)
	if err != nil || o.OrganizationID != ac.OrgID {
		apperr.WriteError(w, r, ErrResourceNotFound)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, toPublicOrder(o))
}

// GetPayment returns one payment's status by ID, scoped to the calling org.
// GET /public/v1/payments/{paymentId}
func (a *PublicAPI) GetPayment(w http.ResponseWriter, r *http.Request) {
	ac, ok := APIContextFrom(r.Context())
	if !ok {
		apperr.WriteError(w, r, ErrInvalidAPIKey)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "paymentId"))
	if err != nil {
		apperr.WriteError(w, r, ErrInvalidPayload)
		return
	}
	p, err := a.repo.GetPayment(r.Context(), id)
	if err != nil || p.OrganizationID != ac.OrgID {
		apperr.WriteError(w, r, ErrResourceNotFound)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, toPublicPayment(p))
}
