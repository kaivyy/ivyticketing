package payments

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

// AuditRecorder is satisfied by *audit.Logger.
type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry)
}

// TicketIssuer issues a ticket for a just-PAID order, using the SAME tx querier.
// Implemented by *tickets.Issuer. Must be idempotent.
// Declared here so payments does not import tickets.
type TicketIssuer interface {
	IssueForOrder(ctx context.Context, q *db.Queries, order db.Order) error
}

// Processor is the idempotent callback handler used by both the HTTP webhook
// receiver and the manual reconcile path.
type Processor struct {
	repo   Repository
	audit  AuditRecorder
	issuer TicketIssuer
}

func NewProcessor(repo Repository, recorder AuditRecorder, issuer TicketIssuer) *Processor {
	return &Processor{repo: repo, audit: recorder, issuer: issuer}
}

// ProcessRaw stores the raw webhook first, then processes it.
// Used by the webhook receiver binary.
func (p *Processor) ProcessRaw(ctx context.Context, g gw.Gateway, headers map[string][]string, rawBody []byte) error {
	signatureValid := g.VerifySignature(http.Header(headers), rawBody)

	parsed, parseErr := g.ParseCallback(rawBody)

	wh, err := p.repo.CreatePaymentWebhook(ctx, db.CreatePaymentWebhookParams{
		Gateway:           g.Name(),
		EventType:         nullText(parsed.EventType),
		MerchantReference: nullText(parsed.MerchantReference),
		GatewayReference:  nullText(parsed.GatewayReference),
		Signature:         nullText(extractSignature(g.Name(), headers)),
		SignatureValid:    signatureValid,
		Payload:           rawBody,
		ProcessingStatus:  WebhookReceived,
	})
	if err != nil {
		return fmt.Errorf("store webhook: %w", err)
	}

	if !signatureValid {
		_ = p.repo.MarkWebhookProcessed(ctx, db.MarkWebhookProcessedParams{
			ID:               wh.ID,
			ProcessingStatus: WebhookRejected,
			ErrorDetail:      nullText("INVALID_SIGNATURE"),
		})
		p.recordRejected(ctx, g.Name(), "INVALID_SIGNATURE")
		return ErrInvalidSignature
	}
	if parseErr != nil {
		_ = p.repo.MarkWebhookProcessed(ctx, db.MarkWebhookProcessedParams{
			ID:               wh.ID,
			ProcessingStatus: WebhookFailed,
			ErrorDetail:      nullText(parseErr.Error()),
		})
		return fmt.Errorf("parse callback: %w", parseErr)
	}

	return p.applyWithWebhook(ctx, wh.ID, g.Name(), parsed)
}

// Apply runs the idempotent state transition without an associated webhook row.
// Used directly by the reconcile path.
func (p *Processor) Apply(ctx context.Context, gatewayName string, res gw.CallbackResult) error {
	return p.apply(ctx, uuid.Nil, gatewayName, res)
}

func (p *Processor) applyWithWebhook(ctx context.Context, webhookID uuid.UUID, gatewayName string, res gw.CallbackResult) error {
	return p.apply(ctx, webhookID, gatewayName, res)
}

func (p *Processor) apply(ctx context.Context, webhookID uuid.UUID, gatewayName string, res gw.CallbackResult) error {
	dedupeK := dedupeKey(gatewayName, res)

	// Try to claim dedupe slot (only for real webhooks)
	if webhookID != uuid.Nil && dedupeK != "" {
		if err := p.repo.ClaimWebhookDedupe(ctx, webhookID, dedupeK); err != nil {
			if errors.Is(err, ErrDuplicateDedupe) {
				_ = p.repo.MarkWebhookProcessed(ctx, db.MarkWebhookProcessedParams{
					ID:               webhookID,
					ProcessingStatus: WebhookDuplicate,
					ErrorDetail:      nullText("duplicate dedupe key"),
				})
				return nil // idempotent no-op
			}
			return err
		}
	}

	return p.repo.ExecTx(ctx, func(tx Repository) error {
		payment, err := tx.GetPaymentByMerchantRefForUpdate(ctx, res.MerchantReference)
		if errors.Is(err, pgx.ErrNoRows) {
			if webhookID != uuid.Nil {
				_ = p.repo.MarkWebhookProcessed(ctx, db.MarkWebhookProcessedParams{
					ID:               webhookID,
					ProcessingStatus: WebhookRejected,
					ErrorDetail:      nullText("PAYMENT_NOT_FOUND"),
				})
			}
			return ErrPaymentNotFound
		} else if err != nil {
			return err
		}

		if res.Amount != 0 && res.Amount != payment.Amount {
			if webhookID != uuid.Nil {
				_ = p.repo.MarkWebhookProcessed(ctx, db.MarkWebhookProcessedParams{
					ID:               webhookID,
					ProcessingStatus: WebhookRejected,
					ErrorDetail:      nullText("PAYMENT_AMOUNT_MISMATCH"),
				})
			}
			return ErrAmountMismatch
		}

		// Payment already final → idempotent no-op
		if payment.Status != StatusPending {
			if webhookID != uuid.Nil {
				_ = p.repo.MarkWebhookProcessed(ctx, db.MarkWebhookProcessedParams{
					ID:               webhookID,
					ProcessingStatus: WebhookProcessed,
				})
			}
			return nil
		}

		switch res.Status {
		case gw.StatusPaid:
			return p.applyPaid(ctx, tx, webhookID, payment, res)
		case gw.StatusExpired, gw.StatusFailed:
			_, err := tx.UpdatePaymentStatus(ctx, db.UpdatePaymentStatusParams{
				ID:     payment.ID,
				Status: dbStatusFromGateway(res.Status),
			})
			if err != nil {
				return err
			}
			if webhookID != uuid.Nil {
				id := payment.ID
				_ = p.repo.MarkWebhookProcessed(ctx, db.MarkWebhookProcessedParams{
					ID:                 webhookID,
					ProcessingStatus:   WebhookProcessed,
					ProcessedPaymentID: &id,
				})
			}
			return nil
		default:
			if webhookID != uuid.Nil {
				_ = p.repo.MarkWebhookProcessed(ctx, db.MarkWebhookProcessedParams{
					ID:               webhookID,
					ProcessingStatus: WebhookProcessed,
				})
			}
			return nil
		}
	})
}

func (p *Processor) applyPaid(ctx context.Context, tx Repository, webhookID uuid.UUID, payment db.Payment, res gw.CallbackResult) error {
	var paidAt pgtype.Timestamptz
	if res.PaidAt != nil {
		paidAt = pgtype.Timestamptz{Time: *res.PaidAt, Valid: true}
	} else {
		paidAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}

	updated, err := tx.MarkPaymentPaid(ctx, db.MarkPaymentPaidParams{
		ID:               payment.ID,
		PaidAt:           paidAt,
		GatewayReference: nullText(res.GatewayReference),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Already PAID by a concurrent process → no-op
		if webhookID != uuid.Nil {
			_ = p.repo.MarkWebhookProcessed(ctx, db.MarkWebhookProcessedParams{
				ID:               webhookID,
				ProcessingStatus: WebhookProcessed,
			})
		}
		return nil
	} else if err != nil {
		return err
	}

	order, err := tx.GetOrderByIDForUpdate(ctx, updated.OrderID)
	if err != nil {
		return err
	}

	var note string
	if order.Status == OrderPendingPayment {
		if _, err := tx.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID:       order.ID,
			Status:   OrderPaid,
			Status_2: OrderPendingPayment,
		}); err != nil {
			return err
		}
		if err := tx.CompleteReservationsForOrder(ctx, order.ID); err != nil {
			return err
		}
		if p.issuer != nil {
			if err := p.issuer.IssueForOrder(ctx, tx.Querier(), order); err != nil {
				return err
			}
		}
	} else {
		note = "ORDER_ALREADY_" + order.Status
	}

	if webhookID != uuid.Nil {
		id := updated.ID
		params := db.MarkWebhookProcessedParams{
			ID:                 webhookID,
			ProcessingStatus:   WebhookProcessed,
			ProcessedPaymentID: &id,
		}
		if note != "" {
			params.ErrorDetail = nullText(note)
		}
		_ = p.repo.MarkWebhookProcessed(ctx, params)
	}

	p.recordPaid(ctx, updated, note)
	return nil
}

func dedupeKey(gateway string, res gw.CallbackResult) string {
	ref := res.GatewayReference
	if ref == "" {
		ref = res.MerchantReference
	}
	return gateway + ":" + ref + ":" + string(res.Status)
}

func extractSignature(gateway string, headers map[string][]string) string {
	switch gateway {
	case "xendit":
		if v, ok := headers["X-Callback-Token"]; ok && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

// uuidNullable converts a uuid.UUID to a *uuid.UUID for use in MarkWebhookProcessedParams.
func uuidPtr(id uuid.UUID) *uuid.UUID {
	return &id
}

func (p *Processor) recordPaid(ctx context.Context, payment db.Payment, note string) {
	if p.audit == nil {
		return
	}
	oid := payment.OrganizationID
	uid := payment.ParticipantID
	meta := map[string]any{"merchantReference": payment.MerchantReference}
	if note != "" {
		meta["note"] = note
	}
	p.audit.Record(ctx, audit.Entry{
		OrganizationID: &oid,
		ActorUserID:    &uid,
		Action:         "PAYMENT_PAID",
		TargetType:     "payment",
		TargetID:       payment.ID.String(),
		Metadata:       meta,
	})
}

func (p *Processor) recordRejected(ctx context.Context, gateway, reason string) {
	if p.audit == nil {
		return
	}
	p.audit.Record(ctx, audit.Entry{
		Action:     "PAYMENT_CALLBACK_REJECTED",
		TargetType: "webhook",
		Metadata:   map[string]any{"gateway": gateway, "reason": reason},
	})
}
