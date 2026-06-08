package tickets

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

// IssuerQuerier is the minimal tx-bound query surface the issuer needs.
// *db.Queries satisfies it.
type IssuerQuerier interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
	GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error)
	GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error)
	CreateTicket(ctx context.Context, arg db.CreateTicketParams) (db.Ticket, error)
}

// AuditRecorder is satisfied by *audit.Logger.
type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry)
}

// Issuer creates a ticket for a just-PAID order, on the SAME tx as the caller.
type Issuer struct {
	audit AuditRecorder
}

// NewIssuer constructs an Issuer. recorder may be nil (audit is skipped).
func NewIssuer(recorder AuditRecorder) *Issuer {
	return &Issuer{audit: recorder}
}

// IssueForOrder satisfies payments.TicketIssuer. q MUST be the tx-bound querier
// from the payments transaction so the INSERT commits/rolls back atomically.
func (i *Issuer) IssueForOrder(ctx context.Context, q *db.Queries, order db.Order) error {
	return i.IssueWith(ctx, q, order)
}

// IssueWith is the testable core; q is any IssuerQuerier.
func (i *Issuer) IssueWith(ctx context.Context, q IssuerQuerier, order db.Order) error {
	user, err := q.GetUserByID(ctx, order.ParticipantID)
	if err != nil {
		return err
	}
	event, err := q.GetEventByID(ctx, order.EventID)
	if err != nil {
		return err
	}
	category, err := q.GetCategoryByID(ctx, order.CategoryID)
	if err != nil {
		return err
	}

	num, err := generateTicketNumber(time.Now())
	if err != nil {
		return err
	}

	created, err := q.CreateTicket(ctx, db.CreateTicketParams{
		OrganizationID: order.OrganizationID,
		EventID:        order.EventID,
		CategoryID:     order.CategoryID,
		OrderID:        order.ID,
		ParticipantID:  order.ParticipantID,
		TicketNumber:   num,
		HolderName:     user.FullName,
		HolderEmail:    user.Email,
		EventTitle:     event.Name,
		CategoryName:   category.Name,
		QrVersion:      1,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Ticket already issued for this order (ON CONFLICT DO NOTHING) — idempotent no-op.
		return nil
	}
	if err != nil {
		return err
	}

	if i.audit != nil {
		orgID := order.OrganizationID
		actor := order.ParticipantID
		i.audit.Record(ctx, audit.Entry{
			OrganizationID: &orgID,
			ActorUserID:    &actor,
			Action:         "TICKET_ISSUED",
			TargetType:     "ticket",
			TargetID:       created.ID.String(),
			Metadata:       map[string]any{"orderId": order.ID.String(), "ticketNumber": num},
		})
	}
	return nil
}
