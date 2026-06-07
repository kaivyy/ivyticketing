//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

func loginAndCreateOrg(t *testing.T, client *http.Client, baseURL, email, orgName string) (token, orgID string) {
	t.Helper()
	postJSON(t, client, baseURL+"/api/v1/auth/register",
		map[string]string{"email": email, "password": "pw123456", "fullName": email}, "").Body.Close()
	resp := postJSON(t, client, baseURL+"/api/v1/auth/login",
		map[string]string{"email": email, "password": "pw123456"}, "")
	var login struct {
		AccessToken string `json:"accessToken"`
	}
	json.NewDecoder(resp.Body).Decode(&login)
	resp.Body.Close()

	resp = postJSON(t, client, baseURL+"/api/v1/organizations",
		map[string]string{"name": orgName}, login.AccessToken)
	var org struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&org)
	resp.Body.Close()
	return login.AccessToken, org.ID
}

func TestTenantIsolation_MemberOfACannotAccessB(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	_, orgA := loginAndCreateOrg(t, client, srv.URL, "owner-a@x.com", "Org A")
	tokenB, _ := loginAndCreateOrg(t, client, srv.URL, "owner-b@x.com", "Org B")

	// Owner B tries to list members of Org A → 403.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgA+"/members", nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	resp, _ := client.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-tenant access status = %d, want 403", resp.StatusCode)
	}
}
