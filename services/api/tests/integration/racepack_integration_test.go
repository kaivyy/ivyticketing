//go:build integration

// Integration tests for Phase 14 Racepack endpoints.
//
// Run with: go test -tags=integration ./tests/integration/...
// Requires TEST_DATABASE_URL (see make test-db-setup).
package integration

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestRacepack_CounterLifecycle exercises Fix 1+2 (JSON contract) + Fix 4
// (counter ownership scoping) at the HTTP layer.
func TestRacepack_CounterLifecycle(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	token, orgID, _ := loginCreateOrg(t, client, srv.URL, "race-owner@x.com", "Race Org")

	// Set up event + category so counters have somewhere to belong.
	eventID := createPublishedEvent(t, client, srv.URL, token, orgID, "Race Event")

	// Counter creation — camelCase JSON (current contract).
	resp := postJSON(t, client,
		srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/racepack/counters",
		map[string]any{"name": "Counter A", "location": "Lobby", "active": true},
		token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create counter = %d, want 201", resp.StatusCode)
	}
	var counter struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Active bool   `json:"active"`
	}
	mustDecode(t, resp, &counter)
	if counter.Name != "Counter A" || !counter.Active {
		t.Errorf("counter = %+v, want name=Counter A active=true", counter)
	}

	// Counter list returns at least our entry.
	resp = postJSONGet(t, client,
		srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/racepack/counters", token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list counters = %d, want 200", resp.StatusCode)
	}
	var counters []map[string]any
	mustDecode(t, resp, &counters)
	if len(counters) < 1 {
		t.Fatalf("expected >=1 counter, got %d", len(counters))
	}

	// Set active=false.
	resp = postJSON(t, client,
		srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/racepack/counters/"+counter.ID+"/activate",
		map[string]any{"active": false}, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("activate counter = %d, want 200", resp.StatusCode)
	}
	var updated struct{ Active bool `json:"active"` }
	mustDecode(t, resp, &updated)
	if updated.Active {
		t.Error("expected active=false after deactivate")
	}
}

// TestRacepack_SlotCapacityEnforcedAtAPI exercises Fix 6+7: the participant
// reservation endpoint must reject over-capacity with 409 SLOT_FULL.
func TestRacepack_SlotCapacityEnforcedAtAPI(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	token, orgID, _ := loginCreateOrg(t, client, srv.URL, "race-owner2@x.com", "Race Org 2")
	eventID := createPublishedEvent(t, client, srv.URL, token, orgID, "Race Event 2")

	// Create a slot with capacity 1.
	now := time.Now().UTC()
	resp := postJSON(t, client,
		srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/racepack/slots",
		map[string]any{
			"name":       "Slot 1",
			"pickupDate": now.Format("2006-01-02"),
			"startTime":  now.Add(1 * time.Hour).Format(time.RFC3339),
			"endTime":    now.Add(3 * time.Hour).Format(time.RFC3339),
			"capacity":   1,
		},
		token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create slot = %d, want 201", resp.StatusCode)
	}
	var slot struct {
		ID            string `json:"id"`
		Capacity      int    `json:"capacity"`
		ReservedCount int    `json:"reservedCount"`
	}
	mustDecode(t, resp, &slot)

	// First reserve: 200.
	resp = postJSON(t, client,
		srv.URL+"/api/v1/events/"+eventID+"/racepack/slots/"+slot.ID+"/reserve",
		nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first reserve = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Second reserve: 409 SLOT_FULL.
	resp = postJSON(t, client,
		srv.URL+"/api/v1/events/"+eventID+"/racepack/slots/"+slot.ID+"/reserve",
		nil, token)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("second reserve = %d, want 409 SLOT_FULL", resp.StatusCode)
	}
	var errResp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	mustDecode(t, resp, &errResp)
	if errResp.Error.Code != "SLOT_FULL" {
		t.Errorf("error code = %q, want SLOT_FULL", errResp.Error.Code)
	}
}

// TestRacepack_DashboardShape verifies Fix 2: dashboard returns the
// unified `byCounter` + `openCases` shape the frontend expects.
func TestRacepack_DashboardShape(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	token, orgID, _ := loginCreateOrg(t, client, srv.URL, "race-owner3@x.com", "Race Org 3")
	eventID := createPublishedEvent(t, client, srv.URL, token, orgID, "Race Event 3")

	// Create one counter so byCounter has at least one entry.
	resp := postJSON(t, client,
		srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/racepack/counters",
		map[string]any{"name": "C1", "active": true}, token)
	resp.Body.Close()

	resp = postJSONGet(t, client,
		srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/racepack/dashboard", token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dashboard = %d, want 200", resp.StatusCode)
	}
	// Must contain the fields the frontend dashboard reads.
	var dash map[string]json.RawMessage
	mustDecode(t, resp, &dash)
	for _, k := range []string{"totalPickups", "byCounter", "openCases", "totalCounters", "activeCounters"} {
		if _, ok := dash[k]; !ok {
			t.Errorf("dashboard missing key %q", k)
		}
	}
	// byCounter rows must use snake_case (frontend expects counter_id, count).
	var rows []map[string]any
	if err := json.Unmarshal(dash["byCounter"], &rows); err != nil {
		t.Fatalf("byCounter unmarshal: %v", err)
	}
	if len(rows) >= 1 {
		first := rows[0]
		if _, ok := first["counter_id"]; !ok {
			t.Errorf("byCounter[0] missing counter_id")
		}
		if _, ok := first["count"]; !ok {
			t.Errorf("byCounter[0] missing count")
		}
	}
}

// TestRacepack_ProblemCaseRequiresTarget verifies Fix 11: at least one of
// ticket_id or participant_id is required.
func TestRacepack_ProblemCaseRequiresTarget(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	token, orgID, _ := loginCreateOrg(t, client, srv.URL, "race-owner4@x.com", "Race Org 4")
	eventID := createPublishedEvent(t, client, srv.URL, token, orgID, "Race Event 4")

	resp := postJSON(t, client,
		srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/racepack/problem-cases",
		map[string]any{"reason": "missing BIB"}, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("no-target problem-case = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()

	// Open a valid problem case with participant_id only.
	resp = postJSON(t, client,
		srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/racepack/problem-cases",
		map[string]any{
			"participantId": uuid.NewString(),
			"reason":        "missing BIB",
		}, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("valid problem-case = %d, want 201", resp.StatusCode)
	}
	resp.Body.Close()

	// Dashboard openCases should now be 1.
	resp = postJSONGet(t, client,
		srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/racepack/dashboard", token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dashboard = %d, want 200", resp.StatusCode)
	}
	var dash struct {
		OpenCases int64 `json:"openCases"`
	}
	mustDecode(t, resp, &dash)
	if dash.OpenCases != 1 {
		t.Errorf("openCases = %d, want 1", dash.OpenCases)
	}
}

// --- helpers ---

func mustDecode(t *testing.T, resp *http.Response, out any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

// postJSONGet is a GET-with-bearer helper. Mirrors postJSON but for reads.
func postJSONGet(t *testing.T, client *http.Client, url, bearer string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

// createPublishedEvent creates an event + category and publishes it.
func createPublishedEvent(t *testing.T, client *http.Client, baseURL, token, orgID, name string) string {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/organizations/"+orgID+"/events",
		map[string]any{"name": name, "eventType": "marathon"}, token)
	var ev struct{ ID string }
	mustDecode(t, resp, &ev)
	postJSON(t, client, baseURL+"/api/v1/organizations/"+orgID+"/events/"+ev.ID+"/categories",
		map[string]any{
			"name":                 "10K",
			"price":                100000,
			"capacity":             100,
			"registrationOpensAt":  time.Now().Format(time.RFC3339),
			"registrationClosesAt": time.Now().Add(720 * time.Hour).Format(time.RFC3339),
			"maxOrderPerUser":      1,
		}, token).Body.Close()
	postJSON(t, client, baseURL+"/api/v1/organizations/"+orgID+"/events/"+ev.ID+"/publish",
		map[string]any{}, token).Body.Close()
	return ev.ID
}
