//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
	paymentsmod "github.com/varin/ivyticketing/services/api/internal/modules/payments"
	ticketsmod "github.com/varin/ivyticketing/services/api/internal/modules/tickets"
	"github.com/varin/ivyticketing/services/api/internal/modules/tickets/qr"
)

// TestPhase7_PaidIssuesTicket: checkout → seed payment row → Processor.Apply(PAID)
// → order becomes PAID, exactly one VALID ticket exists, QR token verifies.
func TestPhase7_PaidIssuesTicket(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}
	ctx := context.Background()

	ownerToken, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner7@x.com", "Phase7 Org")
	eventID, categoryID := publishEventWithCategory(t, client, srv.URL, ownerToken, orgID, 10, 5)
	partToken := registerAndLogin(t, client, srv.URL, "participant7@x.com")

	// Checkout → get order.
	resp := postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/categories/"+categoryID+"/checkout", map[string]any{}, partToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("checkout = %d, want 201", resp.StatusCode)
	}
	var order struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Total  int64  `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&order)
	resp.Body.Close()
	if order.Status != "PENDING_PAYMENT" {
		t.Fatalf("order status = %q, want PENDING_PAYMENT", order.Status)
	}

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

	// Seed a PENDING payment row directly (gateway HTTP layer is not wired in tests).
	merchantRef := "TEST-PHASE7-" + order.ID[:8]
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

	// Apply PAID callback via Processor — same code path as the webhook binary.
	paymentsRepo := paymentsmod.NewRepository(pool)
	ticketIssuer := ticketsmod.NewIssuer(nil)
	processor := paymentsmod.NewProcessor(paymentsRepo, nil, ticketIssuer)

	paidAt := time.Now()
	if err := processor.Apply(ctx, "duitku", gw.CallbackResult{
		MerchantReference: merchantRef,
		GatewayReference:  "GW-PHASE7-001",
		Status:            gw.StatusPaid,
		Amount:            dbAmount,
		PaidAt:            &paidAt,
	}); err != nil {
		t.Fatalf("processor.Apply: %v", err)
	}

	// Assert order is PAID.
	var orderStatus string
	pool.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, order.ID).Scan(&orderStatus)
	if orderStatus != "PAID" {
		t.Fatalf("order status after PAID callback = %q, want PAID", orderStatus)
	}

	// Assert exactly one VALID ticket exists for this order.
	var ticketID, ticketStatus string
	err = pool.QueryRow(ctx,
		`SELECT id::text, status FROM tickets WHERE order_id = $1`,
		order.ID,
	).Scan(&ticketID, &ticketStatus)
	if err != nil {
		t.Fatalf("query ticket: %v", err)
	}
	if ticketStatus != ticketsmod.StatusValid {
		t.Fatalf("ticket status = %q, want %q", ticketStatus, ticketsmod.StatusValid)
	}

	// Fetch the QR token via the participant API.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/tickets/"+ticketID+"/qr", nil)
	req.Header.Set("Authorization", "Bearer "+partToken)
	qrResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET qr: %v", err)
	}
	defer qrResp.Body.Close()
	if qrResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /tickets/%s/qr = %d, want 200", ticketID, qrResp.StatusCode)
	}
	var qrBody struct {
		QRToken string `json:"qrToken"`
	}
	json.NewDecoder(qrResp.Body).Decode(&qrBody)
	if qrBody.QRToken == "" {
		t.Fatal("QR token is empty")
	}

	// Verify QR token with the same secret used in newTestServer.
	signer := qr.NewSigner("test-qr-secret")
	ref, err := signer.Verify(qrBody.QRToken)
	if err != nil {
		t.Fatalf("QR verify: %v", err)
	}
	if ref.TicketID.String() != ticketID {
		t.Errorf("QR ticketID = %s, want %s", ref.TicketID, ticketID)
	}
	if ref.EventID.String() != eventID {
		t.Errorf("QR eventID = %s, want %s", ref.EventID, eventID)
	}
}
