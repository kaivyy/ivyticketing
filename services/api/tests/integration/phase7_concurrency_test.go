//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
	paymentsmod "github.com/varin/ivyticketing/services/api/internal/modules/payments"
	ticketsmod "github.com/varin/ivyticketing/services/api/internal/modules/tickets"
)

// TestPhase7_ConcurrentPaid_OneTicket fires N concurrent Processor.Apply(PAID)
// calls for the same order; exactly one ticket must exist afterwards.
func TestPhase7_ConcurrentPaid_OneTicket(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}
	ctx := context.Background()

	ownerToken, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner7c@x.com", "Phase7C Org")
	eventID, categoryID := publishEventWithCategory(t, client, srv.URL, ownerToken, orgID, 10, 5)
	partToken := registerAndLogin(t, client, srv.URL, "participant7c@x.com")

	// Checkout → get order.
	resp := postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/categories/"+categoryID+"/checkout", map[string]any{}, partToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("checkout = %d, want 201", resp.StatusCode)
	}
	var order struct {
		ID    string `json:"id"`
		Total int64  `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&order)
	resp.Body.Close()

	// Fetch DB columns needed to seed the payment row.
	var dbOrgID, dbEventID, dbParticipantID string
	var dbAmount int64
	err := pool.QueryRow(ctx,
		`SELECT organization_id::text, event_id::text, participant_id::text, total FROM orders WHERE id = $1`,
		order.ID,
	).Scan(&dbOrgID, &dbEventID, &dbParticipantID, &dbAmount)
	if err != nil {
		t.Fatalf("query order: %v", err)
	}

	// Seed a single PENDING payment row.
	merchantRef := "TEST-CONC7-" + order.ID[:8]
	queries := db.New(pool)
	_, err = queries.CreatePayment(ctx, db.CreatePaymentParams{
		OrganizationID:    mustUUID(t, dbOrgID),
		EventID:           mustUUID(t, dbEventID),
		OrderID:           mustUUID(t, order.ID),
		ParticipantID:     mustUUID(t, dbParticipantID),
		Gateway:           "duitku",
		Method:            "qris",
		Status:            paymentsmod.StatusPending,
		Amount:            dbAmount,
		Currency:          "IDR",
		MerchantReference: merchantRef,
		ExpiresAt:         pgtype.Timestamptz{Time: time.Now().Add(15 * time.Minute), Valid: true},
	})
	if err != nil {
		t.Fatalf("seed payment: %v", err)
	}

	// Build a Processor per goroutine to avoid shared state (each has its own repo).
	const N = 20
	paidAt := time.Now()
	res := gw.CallbackResult{
		MerchantReference: merchantRef,
		GatewayReference:  "GW-CONC7-001",
		Status:            gw.StatusPaid,
		Amount:            dbAmount,
		PaidAt:            &paidAt,
	}

	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			proc := paymentsmod.NewProcessor(
				paymentsmod.NewRepository(pool),
				nil,
				ticketsmod.NewIssuer(nil),
			)
			// Tolerate idempotent no-ops and duplicate-key errors — only real faults matter.
			_ = proc.Apply(context.Background(), "duitku", res)
		}()
	}
	wg.Wait()

	// Exactly one ticket must exist for this order.
	var count int
	pool.QueryRow(ctx, `SELECT count(*) FROM tickets WHERE order_id = $1`, order.ID).Scan(&count)
	if count != 1 {
		t.Fatalf("ticket count = %d, want 1 (idempotency failure)", count)
	}

	// The ticket must be VALID.
	var ticketStatus string
	pool.QueryRow(ctx, `SELECT status FROM tickets WHERE order_id = $1`, order.ID).Scan(&ticketStatus)
	if ticketStatus != ticketsmod.StatusValid {
		t.Fatalf("ticket status = %q, want %q", ticketStatus, ticketsmod.StatusValid)
	}

	// The order must be PAID.
	var orderStatus string
	pool.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, order.ID).Scan(&orderStatus)
	if orderStatus != "PAID" {
		t.Fatalf("order status = %q, want PAID", orderStatus)
	}
}
