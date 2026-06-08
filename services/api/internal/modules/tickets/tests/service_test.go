package tickets_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/tickets"
)

type fakeRepo struct {
	ticket db.Ticket
	order  db.Order
	getErr error
}

func (f *fakeRepo) ExecTx(ctx context.Context, fn func(tickets.Repository) error) error { return fn(f) }
func (f *fakeRepo) CreateTicket(ctx context.Context, arg db.CreateTicketParams) (db.Ticket, error) {
	return f.ticket, nil
}
func (f *fakeRepo) GetTicketByID(ctx context.Context, id uuid.UUID) (db.Ticket, error) {
	if f.getErr != nil {
		return db.Ticket{}, f.getErr
	}
	return f.ticket, nil
}
func (f *fakeRepo) GetTicketByOrderID(ctx context.Context, orderID uuid.UUID) (db.Ticket, error) {
	if f.getErr != nil {
		return db.Ticket{}, f.getErr
	}
	return f.ticket, nil
}
func (f *fakeRepo) ListTicketsByParticipant(ctx context.Context, pid uuid.UUID) ([]db.Ticket, error) {
	return []db.Ticket{f.ticket}, nil
}
func (f *fakeRepo) ListTicketsByEvent(ctx context.Context, arg db.ListTicketsByEventParams) ([]db.Ticket, error) {
	return []db.Ticket{f.ticket}, nil
}
func (f *fakeRepo) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	return db.User{}, nil
}
func (f *fakeRepo) GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error) {
	return db.Event{}, nil
}
func (f *fakeRepo) GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error) {
	return db.EventCategory{}, nil
}
func (f *fakeRepo) GetOrderByID(ctx context.Context, id uuid.UUID) (db.Order, error) {
	return f.order, nil
}

func TestGetTicketForUser_OwnershipMismatch_404(t *testing.T) {
	owner := uuid.New()
	other := uuid.New()
	repo := &fakeRepo{ticket: db.Ticket{ID: uuid.New(), ParticipantID: owner}}
	svc := tickets.NewService(repo, tickets.NewQRSigner("secret"), nil)

	_, err := svc.GetTicketForUser(context.Background(), repo, other, repo.ticket.ID)
	if !errors.Is(err, tickets.ErrTicketNotFound) {
		t.Fatalf("want ErrTicketNotFound, got %v", err)
	}
}

func TestGetTicketForUser_NotFound_404(t *testing.T) {
	repo := &fakeRepo{getErr: pgx.ErrNoRows}
	svc := tickets.NewService(repo, tickets.NewQRSigner("secret"), nil)
	_, err := svc.GetTicketForUser(context.Background(), repo, uuid.New(), uuid.New())
	if !errors.Is(err, tickets.ErrTicketNotFound) {
		t.Fatalf("want ErrTicketNotFound, got %v", err)
	}
}

func TestGetInvoice_OrderNotPaid_Conflict(t *testing.T) {
	uid := uuid.New()
	repo := &fakeRepo{order: db.Order{ID: uuid.New(), ParticipantID: uid, Status: "PENDING_PAYMENT"}}
	svc := tickets.NewService(repo, tickets.NewQRSigner("secret"), nil)
	_, err := svc.GetInvoiceForUser(context.Background(), uid, repo.order.ID)
	if !errors.Is(err, tickets.ErrInvoiceNotAvailable) {
		t.Fatalf("want ErrInvoiceNotAvailable, got %v", err)
	}
}
