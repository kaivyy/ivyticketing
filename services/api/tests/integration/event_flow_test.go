//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// loginCreateOrg registers, logs in, creates an org, and returns (token, orgID, orgSlug).
// Distinct from loginAndCreateOrg in tenant_isolation_test.go which returns only (token, orgID).
func loginCreateOrg(t *testing.T, client *http.Client, baseURL, email, orgName string) (token, orgID, orgSlug string) {
	t.Helper()
	postJSON(t, client, baseURL+"/api/v1/auth/register",
		map[string]string{"email": email, "password": "pw123456", "fullName": email}, "").Body.Close()
	resp := postJSON(t, client, baseURL+"/api/v1/auth/login",
		map[string]string{"email": email, "password": "pw123456"}, "")
	var login struct{ AccessToken string `json:"accessToken"` }
	json.NewDecoder(resp.Body).Decode(&login)
	resp.Body.Close()

	resp = postJSON(t, client, baseURL+"/api/v1/organizations",
		map[string]string{"name": orgName}, login.AccessToken)
	var org struct{ ID, Slug string }
	json.NewDecoder(resp.Body).Decode(&org)
	resp.Body.Close()
	return login.AccessToken, org.ID, org.Slug
}

func TestEventFlow_CreateCategoryPublishPublic(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	token, orgID, orgSlug := loginCreateOrg(t, client, srv.URL, "owner@x.com", "Jakarta Marathon Org")

	// Create event.
	resp := postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events",
		map[string]any{"name": "Jakarta Marathon 2026", "eventType": "marathon"}, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create event = %d, want 201", resp.StatusCode)
	}
	var ev struct{ ID, Slug, Status string }
	json.NewDecoder(resp.Body).Decode(&ev)
	resp.Body.Close()
	if ev.Status != "draft" {
		t.Errorf("status = %q, want draft", ev.Status)
	}

	// Publish without categories → 409.
	resp = postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+ev.ID+"/publish", map[string]any{}, token)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("publish-no-cats = %d, want 409", resp.StatusCode)
	}
	resp.Body.Close()

	// Add a category.
	resp = postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+ev.ID+"/categories",
		map[string]any{
			"name":                 "42K",
			"price":                350000,
			"capacity":             2000,
			"registrationOpensAt":  time.Now().Format(time.RFC3339),
			"registrationClosesAt": time.Now().Add(720 * time.Hour).Format(time.RFC3339),
			"maxOrderPerUser":      1,
		}, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create category = %d, want 201", resp.StatusCode)
	}
	resp.Body.Close()

	// Publish now succeeds.
	resp = postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+ev.ID+"/publish", map[string]any{}, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("publish = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Public list shows it.
	resp, _ = client.Get(srv.URL + "/api/v1/public/organizations/" + orgSlug + "/events")
	var list []struct{ Slug string }
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list) != 1 || list[0].Slug != ev.Slug {
		t.Fatalf("public list = %+v, want 1 event %q", list, ev.Slug)
	}

	// Public detail includes the category.
	resp, _ = client.Get(srv.URL + "/api/v1/public/organizations/" + orgSlug + "/events/" + ev.Slug)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("public detail = %d, want 200", resp.StatusCode)
	}
	var detail struct {
		Categories []struct{ Name string } `json:"categories"`
	}
	json.NewDecoder(resp.Body).Decode(&detail)
	resp.Body.Close()
	if len(detail.Categories) != 1 || detail.Categories[0].Name != "42K" {
		t.Errorf("public detail categories = %+v", detail.Categories)
	}

	// Unpublish → public detail 404.
	resp = postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+ev.ID+"/unpublish", map[string]any{}, token)
	resp.Body.Close()
	resp, _ = client.Get(srv.URL + "/api/v1/public/organizations/" + orgSlug + "/events/" + ev.Slug)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("public after unpublish = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
}
