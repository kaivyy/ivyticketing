package payments

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

// Reconciler queries the gateway for the current payment status and applies
// the idempotent state transition via the Processor.
type Reconciler struct {
	repo      Repository
	registry  *gw.Registry
	processor *Processor
}

func NewReconciler(repo Repository, registry *gw.Registry, processor *Processor) *Reconciler {
	return &Reconciler{repo: repo, registry: registry, processor: processor}
}

// Reconcile looks up the payment, queries the gateway for its current status,
// and delegates processing to the Processor.
func (r *Reconciler) Reconcile(ctx context.Context, paymentID uuid.UUID) error {
	pay, err := r.repo.GetPaymentByID(ctx, paymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPaymentNotFound
	} else if err != nil {
		return err
	}

	g, ok := r.registry.Get(pay.Gateway)
	if !ok {
		return ErrGatewayNotAvail
	}

	gwRef := ""
	if pay.GatewayReference.Valid {
		gwRef = pay.GatewayReference.String
	}

	res, err := g.QueryStatus(ctx, gwRef)
	if err != nil {
		return ErrReconcileFailed
	}

	// Ensure the processor can look up the payment by merchant reference.
	res.MerchantReference = pay.MerchantReference
	// Fill in amount if the gateway didn't return one.
	if res.Amount == 0 {
		res.Amount = pay.Amount
	}

	return r.processor.Apply(ctx, pay.Gateway, res)
}
