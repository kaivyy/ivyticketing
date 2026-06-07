//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestFormFlow_AutoCreateAddFieldsReorderPreview(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	token, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner@x.com", "Form Org")
	eventID := createEvent(t, client, srv.URL, token, orgID, "Marathon")

	base := srv.URL + "/api/v1/organizations/" + orgID + "/events/" + eventID + "/form"

	// GET form auto-creates empty.
	req, _ := http.NewRequest(http.MethodGet, base, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := client.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get form = %d, want 200", resp.StatusCode)
	}
	var form struct {
		ID     string `json:"id"`
		Fields []any  `json:"fields"`
	}
	json.NewDecoder(resp.Body).Decode(&form)
	resp.Body.Close()
	if form.ID == "" || len(form.Fields) != 0 {
		t.Fatalf("expected empty auto-created form, got %+v", form)
	}

	// Add text field.
	resp = postJSON(t, client, base+"/fields",
		map[string]any{"fieldType": "text", "label": "Nama", "fieldKey": "nama", "isRequired": true}, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add text field = %d, want 201", resp.StatusCode)
	}
	var f1 struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&f1)
	resp.Body.Close()

	// Add dropdown field with options.
	resp = postJSON(t, client, base+"/fields",
		map[string]any{"fieldType": "dropdown", "label": "Gender", "fieldKey": "gender", "options": []string{"Pria", "Wanita"}}, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add dropdown = %d, want 201", resp.StatusCode)
	}
	var f2 struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&f2)
	resp.Body.Close()

	// Reject dropdown without options.
	resp = postJSON(t, client, base+"/fields",
		map[string]any{"fieldType": "radio", "label": "Bad", "fieldKey": "bad"}, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("dropdown-no-options = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()

	// Reject duplicate key.
	resp = postJSON(t, client, base+"/fields",
		map[string]any{"fieldType": "email", "label": "Dup", "fieldKey": "nama"}, token)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("dup-key = %d, want 409", resp.StatusCode)
	}
	resp.Body.Close()

	// Reorder: gender first, then nama.
	body, _ := json.Marshal(map[string]any{"fieldIds": []string{f2.ID, f1.ID}})
	req, _ = http.NewRequest(http.MethodPut, base+"/fields/reorder", bytesReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reorder = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Preview returns visible fields (no category filter → both).
	req, _ = http.NewRequest(http.MethodGet, base+"/preview", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ = client.Do(req)
	var preview []struct{ FieldKey string `json:"fieldKey"` }
	json.NewDecoder(resp.Body).Decode(&preview)
	resp.Body.Close()
	if len(preview) != 2 {
		t.Fatalf("preview fields = %d, want 2", len(preview))
	}
}
