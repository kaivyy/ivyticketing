//go:build integration

package integration

import (
	"net/http"
	"testing"
)

func TestFormTenantIsolation(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	tokenA, orgA, _ := loginCreateOrg(t, client, srv.URL, "a@x.com", "Org A")
	tokenB, orgB, _ := loginCreateOrg(t, client, srv.URL, "b@x.com", "Org B")
	eventA := createEvent(t, client, srv.URL, tokenA, orgA, "A Event")

	// B reads A's form via B's org path → 404 (event not in org B).
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgB+"/events/"+eventA+"/form", nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	resp, _ := client.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-tenant form via own org = %d, want 404", resp.StatusCode)
	}

	// B reads A's form via A's org path → 403 (not a member of org A).
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgA+"/events/"+eventA+"/form", nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	resp, _ = client.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-org form = %d, want 403", resp.StatusCode)
	}
}

func TestFormConditional_CyclicRejected(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	token, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner@x.com", "Cycle Org")
	eventID := createEvent(t, client, srv.URL, token, orgID, "Marathon")
	base := srv.URL + "/api/v1/organizations/" + orgID + "/events/" + eventID + "/form"

	req, _ := http.NewRequest(http.MethodGet, base, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	client.Do(req)

	// Field referencing a field that doesn't exist yet (forward/unknown ref) → rejected.
	resp := postJSON(t, client, base+"/fields", map[string]any{
		"fieldType": "text", "label": "P", "fieldKey": "passport",
		"conditional": map[string]any{"field": "wna", "op": "equals", "value": "Ya"},
	}, token)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown-ref conditional = %d, want 400", resp.StatusCode)
	}
}
