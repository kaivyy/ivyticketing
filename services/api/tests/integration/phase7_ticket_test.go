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

// TestPhase7_OwnershipNotFound: ticket belonging to user A returns 404 when accessed by user B.
func TestPhase7_OwnershipNotFound(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}
	ctx := context.Background()

	ownerToken, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner7b@x.com", "Phase7B Org")
	eventID, categoryID := publishEventWithCategory(t, client, srv.URL, ownerToken, orgID, 10, 5)

	// User A checks out and gets a ticket.
	partAToken := registerAndLogin(t, client, srv.URL, "partA7b@x.com")
	resp := postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/categories/"+categoryID+"/checkout", map[string]any{}, partAToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("checkout = %d, want 201", resp.StatusCode)
	}
	var order struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&order)
	resp.Body.Close()

	// Seed payment and apply PAID to issue the ticket.
	var dbOrgID, dbEventID, dbParticipantID string
	var dbAmount int64
	err := pool.QueryRow(ctx,
		`SELECT organization_id::text, event_id::text, participant_id::text, total FROM orders WHERE id = $1`,
		order.ID,
	).Scan(&dbOrgID, &dbEventID, &dbParticipantID, &dbAmount)
	if err != nil {
		t.Fatalf("query order: %v", err)
	}
	merchantRef := "TEST-P7B-" + order.ID[:8]
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
	paymentsRepo := paymentsmod.NewRepository(pool)
	ticketIssuer := ticketsmod.NewIssuer(nil)
	processor := paymentsmod.NewProcessor(paymentsRepo, nil, ticketIssuer)
	paidAt := time.Now()
	if err := processor.Apply(ctx, "duitku", gw.CallbackResult{
		MerchantReference: merchantRef,
		GatewayReference:  "GW-P7B-001",
		Status:            gw.StatusPaid,
		Amount:            dbAmount,
		PaidAt:            &paidAt,
	}); err != nil {
		t.Fatalf("processor.Apply: %v", err)
	}

	// Fetch ticketID issued to user A.
	var ticketID string
	err = pool.QueryRow(ctx, `SELECT id::text FROM tickets WHERE order_id = $1`, order.ID).Scan(&ticketID)
	if err != nil {
		t.Fatalf("query ticket: %v", err)
	}

	// User B tries to access user A's ticket → 404.
	partBToken := registerAndLogin(t, client, srv.URL, "partB7b@x.com")
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/tickets/"+ticketID, nil)
	req.Header.Set("Authorization", "Bearer "+partBToken)
	got, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET ticket as user B: %v", err)
	}
	got.Body.Close()
	if got.StatusCode != http.StatusNotFound {
		t.Errorf("GET /tickets/%s as user B = %d, want 404", ticketID, got.StatusCode)
	}
}

// TestPhase7_InvoiceGating: invoice endpoint returns non-200 before order is PAID, 200 after.
func TestPhase7_InvoiceGating(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}
	ctx := context.Background()

	ownerToken, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner7c@x.com", "Phase7C Org")
	eventID, categoryID := publishEventWithCategory(t, client, srv.URL, ownerToken, orgID, 10, 5)
	partToken := registerAndLogin(t, client, srv.URL, "part7c@x.com")

	// Checkout → PENDING_PAYMENT order.
	resp := postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/categories/"+categoryID+"/checkout", map[string]any{}, partToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("checkout = %d, want 201", resp.StatusCode)
	}
	var order struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&order)
	resp.Body.Close()

	// Invoice before PAID → non-200.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/orders/"+order.ID+"/invoice", nil)
	req.Header.Set("Authorization", "Bearer "+partToken)
	preResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET invoice before PAID: %v", err)
	}
	preResp.Body.Close()
	if preResp.StatusCode == http.StatusOK {
		t.Errorf("GET invoice before PAID = 200, want non-200")
	}

	// Seed payment and apply PAID.
	var dbOrgID, dbEventID, dbParticipantID string
	var dbAmount int64
	err = pool.QueryRow(ctx,
		`SELECT organization_id::text, event_id::text, participant_id::text, total FROM orders WHERE id = $1`,
		order.ID,
	).Scan(&dbOrgID, &dbEventID, &dbParticipantID, &dbAmount)
	if err != nil {
		t.Fatalf("query order: %v", err)
	}
	merchantRef := "TEST-P7C-" + order.ID[:8]
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
	paymentsRepo := paymentsmod.NewRepository(pool)
	ticketIssuer := ticketsmod.NewIssuer(nil)
	processor := paymentsmod.NewProcessor(paymentsRepo, nil, ticketIssuer)
	paidAt := time.Now()
	if err := processor.Apply(ctx, "duitku", gw.CallbackResult{
		MerchantReference: merchantRef,
		GatewayReference:  "GW-P7C-001",
		Status:            gw.StatusPaid,
		Amount:            dbAmount,
		PaidAt:            &paidAt,
	}); err != nil {
		t.Fatalf("processor.Apply: %v", err)
	}

	// Invoice after PAID → 200.
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/v1/orders/"+order.ID+"/invoice", nil)
	req.Header.Set("Authorization", "Bearer "+partToken)
	postResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET invoice after PAID: %v", err)
	}
	postResp.Body.Close()
	if postResp.StatusCode != http.StatusOK {
		t.Errorf("GET invoice after PAID = %d, want 200", postResp.StatusCode)
	}
}

// TestPhase7_OrganizerTicketViewPermission: member without ticket.view gets 403;
// org owner (who has ticket.view from seed) gets 200.
func TestPhase7_OrganizerTicketViewPermission(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	ownerToken, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner7d@x.com", "Phase7D Org")
	eventID, _ := publishEventWithCategory(t, client, srv.URL, ownerToken, orgID, 10, 5)

	// Register a fresh staff user and add them as a member with no roles.
	staffToken := registerAndLogin(t, client, srv.URL, "staff7d@x.com")
	addResp := postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/members",
		map[string]any{"email": "staff7d@x.com", "roleIds": []string{}}, ownerToken)
	if addResp.StatusCode != http.StatusCreated {
		t.Fatalf("add member = %d, want 201", addResp.StatusCode)
	}
	addResp.Body.Close()

	ticketsURL := srv.URL + "/api/v1/organizations/" + orgID + "/events/" + eventID + "/tickets"

	// Staff without ticket.view → 403.
	req, _ := http.NewRequest(http.MethodGet, ticketsURL, nil)
	req.Header.Set("Authorization", "Bearer "+staffToken)
	staffResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET tickets as staff: %v", err)
	}
	staffResp.Body.Close()
	if staffResp.StatusCode != http.StatusForbidden {
		t.Errorf("GET tickets without ticket.view = %d, want 403", staffResp.StatusCode)
	}

	// Owner (has ticket.view via Owner role from seed) → 200.
	req, _ = http.NewRequest(http.MethodGet, ticketsURL, nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	ownerResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET tickets as owner: %v", err)
	}
	ownerResp.Body.Close()
	if ownerResp.StatusCode != http.StatusOK {
		t.Errorf("GET tickets with ticket.view (owner) = %d, want 200", ownerResp.StatusCode)
	}
}
