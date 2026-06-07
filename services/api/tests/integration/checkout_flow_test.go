//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCheckoutFlow_CreateGetListCancel(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	ownerToken, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner@x.com", "Checkout Org")
	eventID, categoryID := publishEventWithCategory(t, client, srv.URL, ownerToken, orgID, 100, 2)

	// Participant (a different logged-in user).
	partToken := registerAndLogin(t, client, srv.URL, "participant@x.com")

	// Checkout.
	resp := postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/categories/"+categoryID+"/checkout", map[string]any{}, partToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("checkout = %d, want 201", resp.StatusCode)
	}
	var order struct {
		ID          string `json:"id"`
		OrderNumber string `json:"orderNumber"`
		Status      string `json:"status"`
		Total       int64  `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&order)
	resp.Body.Close()
	if order.Status != "PENDING_PAYMENT" || order.Total != 100000 {
		t.Fatalf("unexpected order: %+v", order)
	}

	// Get own order.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/orders/"+order.ID, nil)
	req.Header.Set("Authorization", "Bearer "+partToken)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get order = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// List own orders.
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/v1/orders", nil)
	req.Header.Set("Authorization", "Bearer "+partToken)
	resp, _ = client.Do(req)
	var list []map[string]any
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list) != 1 {
		t.Fatalf("list = %d orders, want 1", len(list))
	}

	// Organizer sees it via order.view.
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/orders", nil)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("org list = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Cancel.
	req, _ = http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/orders/"+order.ID, nil)
	req.Header.Set("Authorization", "Bearer "+partToken)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("cancel = %d, want 204", resp.StatusCode)
	}
	resp.Body.Close()

	// Reservation released → slot back.
	var active int
	pool.QueryRow(context.Background(), `SELECT count(*) FROM inventory_reservations WHERE category_id=$1 AND status='ACTIVE'`, categoryID).Scan(&active)
	if active != 0 {
		t.Errorf("active reservations after cancel = %d, want 0", active)
	}
}

func TestCheckout_OwnershipIsolation(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	ownerToken, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner@x.com", "Iso Org")
	eventID, categoryID := publishEventWithCategory(t, client, srv.URL, ownerToken, orgID, 100, 5)

	aToken := registerAndLogin(t, client, srv.URL, "a@x.com")
	bToken := registerAndLogin(t, client, srv.URL, "b@x.com")

	resp := postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/categories/"+categoryID+"/checkout", map[string]any{}, aToken)
	var order struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&order)
	resp.Body.Close()

	// B tries to read A's order → 404.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/orders/"+order.ID, nil)
	req.Header.Set("Authorization", "Bearer "+bToken)
	resp, _ = client.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-participant get = %d, want 404", resp.StatusCode)
	}
}
