//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func postJSON(t *testing.T, client *http.Client, url string, body any, bearer string) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func TestFullFlow_RegisterLoginCreateOrgAddMember(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	// Register the owner.
	resp := postJSON(t, client, srv.URL+"/api/v1/auth/register",
		map[string]string{"email": "owner@x.com", "password": "pw123456", "fullName": "Owner"}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register status = %d, want 201", resp.StatusCode)
	}
	resp.Body.Close()

	// Register a second user (future staff).
	resp = postJSON(t, client, srv.URL+"/api/v1/auth/register",
		map[string]string{"email": "staff@x.com", "password": "pw123456", "fullName": "Staff"}, "")
	resp.Body.Close()

	// Login owner.
	resp = postJSON(t, client, srv.URL+"/api/v1/auth/login",
		map[string]string{"email": "owner@x.com", "password": "pw123456"}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want 200", resp.StatusCode)
	}
	var login struct {
		AccessToken string `json:"accessToken"`
	}
	json.NewDecoder(resp.Body).Decode(&login)
	resp.Body.Close()
	if login.AccessToken == "" {
		t.Fatal("expected access token")
	}

	// Create org.
	resp = postJSON(t, client, srv.URL+"/api/v1/organizations",
		map[string]string{"name": "Jakarta Marathon"}, login.AccessToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create org status = %d, want 201", resp.StatusCode)
	}
	var org struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&org)
	resp.Body.Close()

	// Owner can list members (has member.manage via Owner role).
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+org.ID+"/members", nil)
	req.Header.Set("Authorization", "Bearer "+login.AccessToken)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list members status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Add staff member without roles.
	resp = postJSON(t, client, srv.URL+"/api/v1/organizations/"+org.ID+"/members",
		map[string]any{"email": "staff@x.com", "roleIds": []string{}}, login.AccessToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add member status = %d, want 201", resp.StatusCode)
	}
	resp.Body.Close()

	// /me returns memberships.
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+login.AccessToken)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/me status = %d, want 200", resp.StatusCode)
	}
	var me struct {
		Memberships []struct {
			OrganizationID string   `json:"organizationId"`
			RoleSlugs      []string `json:"roleSlugs"`
		} `json:"memberships"`
	}
	json.NewDecoder(resp.Body).Decode(&me)
	resp.Body.Close()
	if len(me.Memberships) != 1 {
		t.Fatalf("/me memberships = %d, want 1", len(me.Memberships))
	}
	if me.Memberships[0].OrganizationID != org.ID {
		t.Errorf("/me membership org = %q, want %q", me.Memberships[0].OrganizationID, org.ID)
	}
}
