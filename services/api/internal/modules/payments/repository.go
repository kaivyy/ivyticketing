package payments

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// ErrDuplicateDedupe is returned when a dedupe key already exists on another webhook row.
var ErrDuplicateDedupe = errors.New("duplicate dedupe key")

// Repository defines all data-access operations needed by the payments module.
type Repository interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error
	// Querier returns the underlying sqlc querier (tx-bound inside ExecTx).
	// Used to run the ticket issuer in the same transaction as the PAID transition.
	Querier() *db.Queries

	// Payment CRUD
	CreatePayment(ctx context.Context, arg db.CreatePaymentParams) (db.Payment, error)
	GetPaymentByID(ctx context.Context, id uuid.UUID) (db.Payment, error)
	GetPaymentByMerchantRefForUpdate(ctx context.Context, merchantReference string) (db.Payment, error)
	ListPaymentsByOrder(ctx context.Context, orderID uuid.UUID) ([]db.Payment, error)
	ListPaymentsByOrgEvent(ctx context.Context, arg db.ListPaymentsByOrgEventParams) ([]db.Payment, error)
	GetActivePaymentByOrder(ctx context.Context, orderID uuid.UUID) (db.Payment, error)
	MarkPaymentPaid(ctx context.Context, arg db.MarkPaymentPaidParams) (db.Payment, error)
	UpdatePaymentStatus(ctx context.Context, arg db.UpdatePaymentStatusParams) (db.Payment, error)

	// Webhook CRUD
	CreatePaymentWebhook(ctx context.Context, arg db.CreatePaymentWebhookParams) (db.PaymentWebhook, error)
	// ClaimWebhookDedupe sets the dedupe_key on a webhook row.
	// Returns ErrDuplicateDedupe if the key is already claimed by another row.
	ClaimWebhookDedupe(ctx context.Context, id uuid.UUID, dedupeKey string) error
	MarkWebhookProcessed(ctx context.Context, arg db.MarkWebhookProcessedParams) error

	// Order / reservation (for atomic transitions in the same tx)
	GetOrderByIDForUpdate(ctx context.Context, id uuid.UUID) (db.Order, error)
	UpdateOrderStatus(ctx context.Context, arg db.UpdateOrderStatusParams) (db.Order, error)
	CompleteReservationsForOrder(ctx context.Context, orderID uuid.UUID) error
}

type sqlcRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

// NewRepository returns a pool-backed Repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &sqlcRepo{pool: pool, q: db.New(pool)}
}

func (r *sqlcRepo) Querier() *db.Queries { return r.q }

func (r *sqlcRepo) ExecTx(ctx context.Context, fn func(Repository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(&sqlcRepo{pool: r.pool, q: db.New(tx)}); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

// Payment CRUD

func (r *sqlcRepo) CreatePayment(ctx context.Context, arg db.CreatePaymentParams) (db.Payment, error) {
	return r.q.CreatePayment(ctx, arg)
}

func (r *sqlcRepo) GetPaymentByID(ctx context.Context, id uuid.UUID) (db.Payment, error) {
	return r.q.GetPaymentByID(ctx, id)
}

func (r *sqlcRepo) GetPaymentByMerchantRefForUpdate(ctx context.Context, merchantReference string) (db.Payment, error) {
	return r.q.GetPaymentByMerchantRefForUpdate(ctx, merchantReference)
}

func (r *sqlcRepo) ListPaymentsByOrder(ctx context.Context, orderID uuid.UUID) ([]db.Payment, error) {
	return r.q.ListPaymentsByOrder(ctx, orderID)
}

func (r *sqlcRepo) ListPaymentsByOrgEvent(ctx context.Context, arg db.ListPaymentsByOrgEventParams) ([]db.Payment, error) {
	return r.q.ListPaymentsByOrgEvent(ctx, arg)
}

func (r *sqlcRepo) GetActivePaymentByOrder(ctx context.Context, orderID uuid.UUID) (db.Payment, error) {
	return r.q.GetActivePaymentByOrder(ctx, orderID)
}

func (r *sqlcRepo) MarkPaymentPaid(ctx context.Context, arg db.MarkPaymentPaidParams) (db.Payment, error) {
	return r.q.MarkPaymentPaid(ctx, arg)
}

func (r *sqlcRepo) UpdatePaymentStatus(ctx context.Context, arg db.UpdatePaymentStatusParams) (db.Payment, error) {
	return r.q.UpdatePaymentStatus(ctx, arg)
}

// Webhook CRUD

func (r *sqlcRepo) CreatePaymentWebhook(ctx context.Context, arg db.CreatePaymentWebhookParams) (db.PaymentWebhook, error) {
	return r.q.CreatePaymentWebhook(ctx, arg)
}

func (r *sqlcRepo) ClaimWebhookDedupe(ctx context.Context, id uuid.UUID, dedupeKey string) error {
	_, err := r.q.ClaimWebhookDedupe(ctx, db.ClaimWebhookDedupeParams{
		ID:        id,
		DedupeKey: pgtype.Text{String: dedupeKey, Valid: true},
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateDedupe
		}
		return err
	}
	return nil
}

func (r *sqlcRepo) MarkWebhookProcessed(ctx context.Context, arg db.MarkWebhookProcessedParams) error {
	return r.q.MarkWebhookProcessed(ctx, arg)
}

// Order / reservation access

func (r *sqlcRepo) GetOrderByIDForUpdate(ctx context.Context, id uuid.UUID) (db.Order, error) {
	return r.q.GetOrderByIDForUpdate(ctx, id)
}

func (r *sqlcRepo) UpdateOrderStatus(ctx context.Context, arg db.UpdateOrderStatusParams) (db.Order, error) {
	return r.q.UpdateOrderStatus(ctx, arg)
}

func (r *sqlcRepo) CompleteReservationsForOrder(ctx context.Context, orderID uuid.UUID) error {
	return r.q.UpdateReservationStatusByOrder(ctx, db.UpdateReservationStatusByOrderParams{
		OrderID: orderID,
		Status:  ReservationCompleted,
	})
}

// nullText returns a pgtype.Text that is NULL when s is empty.
func nullText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}
