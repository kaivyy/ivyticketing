//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestFormConditional_ShowHideAndCategoryScope(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	token, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner@x.com", "Cond Org")
	eventID := createEvent(t, client, srv.URL, token, orgID, "Marathon")

	// Add a category so category_scope can reference it.
	resp := postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/categories",
		map[string]any{
			"name": "42K", "price": 100000, "capacity": 100,
			"registrationOpensAt":  time.Now().Format(time.RFC3339),
			"registrationClosesAt": time.Now().Add(240 * time.Hour).Format(time.RFC3339),
			"maxOrderPerUser":      1,
		}, token)
	var cat struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&cat)
	resp.Body.Close()

	base := srv.URL + "/api/v1/organizations/" + orgID + "/events/" + eventID + "/form"

	// Auto-create form.
	req, _ := http.NewRequest(http.MethodGet, base, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	client.Do(req)

	// Field A: dropdown "wna".
	resp = postJSON(t, client, base+"/fields",
		map[string]any{"fieldType": "dropdown", "label": "WNA", "fieldKey": "wna", "options": []string{"Ya", "Tidak"}}, token)
	resp.Body.Close()

	// Field B: passport, showIf wna == Ya, required.
	resp = postJSON(t, client, base+"/fields", map[string]any{
		"fieldType": "text", "label": "Passport", "fieldKey": "passport", "isRequired": true,
		"conditional": map[string]any{"field": "wna", "op": "equals", "value": "Ya"},
	}, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add conditional field = %d, want 201", resp.StatusCode)
	}
	resp.Body.Close()

	// Validate with wna=Ya → passport visible & required & missing → invalid.
	res := previewValidate(t, client, base, token, map[string]any{"wna": "Ya"})
	if res.Valid {
		t.Fatalf("expected invalid (passport required), got valid")
	}
	if !containsKey(res.VisibleFields, "passport") {
		t.Errorf("passport should be visible when wna=Ya")
	}

	// Validate with wna=Tidak → passport hidden → valid.
	res = previewValidate(t, client, base, token, map[string]any{"wna": "Tidak"})
	if !res.Valid {
		t.Fatalf("expected valid (passport hidden), got errors %+v", res.Errors)
	}
	if containsKey(res.VisibleFields, "passport") {
		t.Errorf("passport should be hidden when wna=Tidak")
	}

	// Add a category-scoped field (only for 42K).
	resp = postJSON(t, client, base+"/fields", map[string]any{
		"fieldType": "text", "label": "Jersey", "fieldKey": "jersey",
		"categoryScope": []string{cat.ID},
	}, token)
	resp.Body.Close()

	// Preview for 42K includes jersey; preview without category excludes it.
	if !containsKey(previewKeys(t, client, base, token, "?categoryId="+cat.ID), "jersey") {
		t.Error("jersey should appear in 42K preview")
	}
	if containsKey(previewKeys(t, client, base, token, ""), "jersey") {
		t.Error("jersey should NOT appear in no-category preview")
	}
}

type pvResp struct {
	Valid  bool `json:"valid"`
	Errors []struct {
		FieldKey string `json:"fieldKey"`
	} `json:"errors"`
	VisibleFields []string `json:"visibleFields"`
}

func previewValidate(t *testing.T, client *http.Client, base, token string, answers map[string]any) pvResp {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"answers": answers})
	req, _ := http.NewRequest(http.MethodPost, base+"/preview/validate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := client.Do(req)
	defer resp.Body.Close()
	var r pvResp
	json.NewDecoder(resp.Body).Decode(&r)
	return r
}

func previewKeys(t *testing.T, client *http.Client, base, token, query string) []string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, base+"/preview"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := client.Do(req)
	defer resp.Body.Close()
	var fields []struct {
		FieldKey string `json:"fieldKey"`
	}
	json.NewDecoder(resp.Body).Decode(&fields)
	keys := make([]string, 0, len(fields))
	for _, f := range fields {
		keys = append(keys, f.FieldKey)
	}
	return keys
}

func containsKey(keys []string, want string) bool {
	for _, k := range keys {
		if k == want {
			return true
		}
	}
	return false
}
