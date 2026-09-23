package tickets

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/tickets/qr"
)

// QRSigner is the signing surface the service needs (qr.Signer satisfies it).
type QRSigner interface {
	Sign(ticketID, eventID uuid.UUID) (string, error)
}

// NewQRSigner is a thin constructor re-export for callers/tests.
func NewQRSigner(secret string) *qr.Signer { return qr.NewSigner(secret) }

// Service handles ticket reads and invoice generation.
type Service struct {
	repo   Repository
	signer QRSigner
	audit  AuditRecorder
}

// NewService constructs a Service. recorder may be nil (audit is skipped).
func NewService(repo Repository, signer QRSigner, recorder AuditRecorder) *Service {
	return &Service{repo: repo, signer: signer, audit: recorder}
}

func toResponse(t db.Ticket) TicketResponse {
	r := TicketResponse{
		ID:           t.ID.String(),
		TicketNumber: t.TicketNumber,
		Status:       t.Status,
		OrderID:      t.OrderID.String(),
		EventID:      t.EventID.String(),
		CategoryID:   t.CategoryID.String(),
		HolderName:   t.HolderName,
		HolderEmail:  t.HolderEmail,
		EventTitle:   t.EventTitle,
		CategoryName: t.CategoryName,
		IssuedAt:     t.IssuedAt.Time,
	}
	if t.UsedAt.Valid {
		u := t.UsedAt.Time
		r.UsedAt = &u
	}
	if t.BibNumber.Valid {
		v := t.BibNumber.String
		r.BibNumber = &v
	}
	if t.BibAssignedAt.Valid {
		u := t.BibAssignedAt.Time
		r.BibAssignedAt = &u
	}
	if t.BibAssignmentMethod.Valid {
		v := t.BibAssignmentMethod.String
		r.BibAssignmentMethod = &v
	}
	if len(t.FormAnswers) > 0 {
		var fa map[string]any
		if err := json.Unmarshal(t.FormAnswers, &fa); err == nil {
			r.FormAnswers = fa
		}
	}
	return r
}

// GetTicketForUser returns a ticket owned by userID (else ErrTicketNotFound).
// repo param allows passing a tx-bound repo; pass s.repo for plain reads.
func (s *Service) GetTicketForUser(ctx context.Context, repo Repository, userID, ticketID uuid.UUID) (TicketWithQR, error) {
	t, err := repo.GetTicketByID(ctx, ticketID)
	if errors.Is(err, pgx.ErrNoRows) {
		return TicketWithQR{}, ErrTicketNotFound
	}
	if err != nil {
		return TicketWithQR{}, err
	}
	if t.ParticipantID == nil || *t.ParticipantID != userID {
		return TicketWithQR{}, ErrTicketNotFound
	}
	token, err := s.signer.Sign(t.ID, t.EventID)
	if err != nil {
		return TicketWithQR{}, err
	}
	return TicketWithQR{TicketResponse: toResponse(t), QRToken: token}, nil
}

// GetTicketByOrderForUser returns the ticket for a given order owned by userID.
func (s *Service) GetTicketByOrderForUser(ctx context.Context, userID, orderID uuid.UUID) (TicketWithQR, error) {
	t, err := s.repo.GetTicketByOrderID(ctx, orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return TicketWithQR{}, ErrTicketNotFound
	}
	if err != nil {
		return TicketWithQR{}, err
	}
	if t.ParticipantID == nil || *t.ParticipantID != userID {
		return TicketWithQR{}, ErrTicketNotFound
	}
	token, err := s.signer.Sign(t.ID, t.EventID)
	if err != nil {
		return TicketWithQR{}, err
	}
	return TicketWithQR{TicketResponse: toResponse(t), QRToken: token}, nil
}

// ListMyTickets returns all tickets belonging to userID.
func (s *Service) ListMyTickets(ctx context.Context, userID uuid.UUID) ([]TicketResponse, error) {
	rows, err := s.repo.ListTicketsByParticipant(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]TicketResponse, 0, len(rows))
	for _, t := range rows {
		out = append(out, toResponse(t))
	}
	return out, nil
}

// GetQRForUser returns only the QR token for a ticket owned by userID.
func (s *Service) GetQRForUser(ctx context.Context, userID, ticketID uuid.UUID) (string, error) {
	tw, err := s.GetTicketForUser(ctx, s.repo, userID, ticketID)
	if err != nil {
		return "", err
	}
	return tw.QRToken, nil
}

// ListEventTickets returns all tickets for an event scoped to an organisation.
func (s *Service) ListEventTickets(ctx context.Context, orgID, eventID uuid.UUID) ([]TicketResponse, error) {
	rows, err := s.repo.ListTicketsByEvent(ctx, db.ListTicketsByEventParams{
		OrganizationID: orgID,
		EventID:        eventID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]TicketResponse, 0, len(rows))
	for _, t := range rows {
		out = append(out, toResponse(t))
	}
	return out, nil
}

// GetInvoiceForUser returns a JSON invoice for a PAID order owned by userID.
func (s *Service) GetInvoiceForUser(ctx context.Context, userID, orderID uuid.UUID) (InvoiceResponse, error) {
	order, err := s.repo.GetOrderByID(ctx, orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return InvoiceResponse{}, ErrTicketNotFound
	}
	if err != nil {
		return InvoiceResponse{}, err
	}
	if order.ParticipantID == nil || *order.ParticipantID != userID {
		return InvoiceResponse{}, ErrTicketNotFound
	}
	if order.Status != orderStatusPaid {
		return InvoiceResponse{}, ErrInvoiceNotAvailable
	}
	t, err := s.repo.GetTicketByOrderID(ctx, orderID)
	if err != nil {
		return InvoiceResponse{}, err
	}
	return InvoiceResponse{
		OrderID:      order.ID.String(),
		OrderNumber:  order.OrderNumber,
		Status:       order.Status,
		EventTitle:   t.EventTitle,
		CategoryName: t.CategoryName,
		HolderName:   t.HolderName,
		HolderEmail:  t.HolderEmail,
		Subtotal:     order.Subtotal,
		Fee:          order.Fee,
		Discount:     order.Discount,
		Total:        order.Total,
		Currency:     "IDR",
		IssuedAt:     t.IssuedAt.Time,
	}, nil
}

// UpdateParticipant updates the holder details and form answers of a ticket.
func (s *Service) UpdateParticipant(ctx context.Context, orgID, eventID, ticketID uuid.UUID, req UpdateParticipantRequest) (TicketResponse, error) {
	var answersJSON []byte
	if len(req.FormAnswers) > 0 {
		if b, err := json.Marshal(req.FormAnswers); err == nil {
			answersJSON = b
		}
	}
	updated, err := s.repo.UpdateTicketParticipant(ctx, db.UpdateTicketParticipantParams{
		ID:             ticketID,
		HolderName:     req.HolderName,
		HolderEmail:    req.HolderEmail,
		FormAnswers:    answersJSON,
		OrganizationID: orgID,
		EventID:        eventID,
	})
	if err != nil {
		return TicketResponse{}, err
	}
	return toResponse(updated), nil
}
