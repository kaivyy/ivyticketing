//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestEventTenantIsolation(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	tokenA, orgA, _ := loginCreateOrg(t, client, srv.URL, "a@x.com", "Org A")
	tokenB, orgB, _ := loginCreateOrg(t, client, srv.URL, "b@x.com", "Org B")

	// A creates an event.
	resp := postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgA+"/events",
		map[string]any{"name": "A Event", "eventType": "marathon"}, tokenA)
	var ev struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&ev)
	resp.Body.Close()

	// B tries to read A's event via B's org path → 404 (event not in org B).
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgB+"/events/"+ev.ID, nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-tenant event read = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()

	// B tries via A's org path → 403 (not a member of org A).
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgA+"/events/"+ev.ID, nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-org access = %d, want 403", resp.StatusCode)
	}
	resp.Body.Close()
}
