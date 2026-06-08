package tickets_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/tickets"
)

// fakeQ implements tickets.IssuerQuerier.
type fakeQ struct {
	user     db.User
	event    db.Event
	category db.EventCategory
	created  []db.CreateTicketParams
	conflict bool // when true, CreateTicket returns pgx.ErrNoRows (duplicate)
}

func (f *fakeQ) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	return f.user, nil
}
func (f *fakeQ) GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error) {
	return f.event, nil
}
func (f *fakeQ) GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error) {
	return f.category, nil
}
func (f *fakeQ) CreateTicket(ctx context.Context, arg db.CreateTicketParams) (db.Ticket, error) {
	f.created = append(f.created, arg)
	if f.conflict {
		return db.Ticket{}, pgx.ErrNoRows
	}
	return db.Ticket{ID: uuid.New(), OrderID: arg.OrderID}, nil
}

func sampleOrder() db.Order {
	return db.Order{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		EventID:        uuid.New(),
		CategoryID:     uuid.New(),
		ParticipantID:  uuid.New(),
		OrderNumber:    "ORD-20260608-ABCDEF",
		Status:         "PAID",
	}
}

func TestIssueForOrder_Snapshots(t *testing.T) {
	fq := &fakeQ{
		user:     db.User{Email: "p@example.com", FullName: "Pelari Satu"},
		event:    db.Event{Name: "Jakarta Run 2026"},
		category: db.EventCategory{Name: "10K"},
	}
	iss := tickets.NewIssuer(nil) // nil audit recorder tolerated
	order := sampleOrder()

	if err := iss.IssueWith(context.Background(), fq, order); err != nil {
		t.Fatalf("issue: %v", err)
	}
	if len(fq.created) != 1 {
		t.Fatalf("expected 1 create, got %d", len(fq.created))
	}
	got := fq.created[0]
	if got.HolderName != "Pelari Satu" || got.HolderEmail != "p@example.com" {
		t.Errorf("holder snapshot wrong: %+v", got)
	}
	if got.EventTitle != "Jakarta Run 2026" || got.CategoryName != "10K" {
		t.Errorf("event/category snapshot wrong: %+v", got)
	}
	if got.OrderID != order.ID {
		t.Errorf("order id mismatch")
	}
}

func TestIssueForOrder_DuplicateNoError(t *testing.T) {
	fq := &fakeQ{
		user:     db.User{Email: "p@example.com", FullName: "Pelari Satu"},
		event:    db.Event{Name: "Jakarta Run 2026"},
		category: db.EventCategory{Name: "10K"},
		conflict: true,
	}
	iss := tickets.NewIssuer(nil)
	if err := iss.IssueWith(context.Background(), fq, sampleOrder()); err != nil {
		t.Fatalf("duplicate issue should be no-op, got: %v", err)
	}
}
