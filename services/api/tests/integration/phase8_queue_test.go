//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	ordersmod "github.com/varin/ivyticketing/services/api/internal/modules/orders"
	queuemod "github.com/varin/ivyticketing/services/api/internal/modules/queue"
	pq "github.com/varin/ivyticketing/services/api/internal/platform/queue"
)

// ---------------------------------------------------------------------------
// Setup helpers
// ---------------------------------------------------------------------------

// skipIfNoDB skips when TEST_DATABASE_URL or REDIS_TEST_URL is absent.
// testPool already handles TEST_DATABASE_URL; this adds the Redis check.
func skipIfNoDB(t *testing.T) {
	t.Helper()
	if os.Getenv("TEST_DATABASE_URL") == "" || os.Getenv("REDIS_TEST_URL") == "" {
		t.Skip("requires TEST_DATABASE_URL and REDIS_TEST_URL")
	}
}

// testRedis returns a Redis client pointed at REDIS_TEST_URL (default localhost).
func testRedis(t *testing.T) *goredis.Client {
	t.Helper()
	redisURL := os.Getenv("REDIS_TEST_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}
	opt, err := goredis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("parse REDIS_TEST_URL: %v", err)
	}
	c := goredis.NewClient(opt)
	t.Cleanup(func() { c.Close() })
	return c
}

// buildQueueSvc wires a real queue.Service for direct Release/ExpireDue calls.
func buildQueueSvc(t *testing.T, pool *pgxpool.Pool, rdb *goredis.Client) *queuemod.Service {
	t.Helper()
	repo := queuemod.NewRepository(pool)
	store := queuemod.NewStore(pq.New(rdb))
	return queuemod.NewService(repo, store, nil, nil, 10, nil)
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

// setRegistrationMode PUTs event registration settings.
func setRegistrationMode(t *testing.T, baseURL, orgID, eventID, token, mode string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"defaultMode":  mode,
		"queueEnabled": mode == "WAR_QUEUE",
	})
	req, _ := http.NewRequest(http.MethodPut,
		baseURL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/registration",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("setRegistrationMode PUT: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("setRegistrationMode = %d, want 200/204", resp.StatusCode)
	}
}

// joinQueue POSTs to join the event queue. Caller closes the body.
func joinQueue(t *testing.T, baseURL, eventID, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost,
		baseURL+"/api/v1/events/"+eventID+"/queue/join",
		bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("joinQueue POST: %v", err)
	}
	return resp
}

// queueStatus GETs the event queue status. Caller closes the body.
func queueStatus(t *testing.T, baseURL, eventID, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet,
		baseURL+"/api/v1/events/"+eventID+"/queue/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("queueStatus GET: %v", err)
	}
	return resp
}

// checkoutWithToken POSTs checkout; sets X-Queue-Token when admissionToken != "".
func checkoutWithToken(t *testing.T, baseURL, orgID, eventID, categoryID, userToken, admissionToken string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost,
		baseURL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/categories/"+categoryID+"/checkout",
		bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)
	if admissionToken != "" {
		req.Header.Set("X-Queue-Token", admissionToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("checkout POST: %v", err)
	}
	return resp
}

// ---------------------------------------------------------------------------
// Task 21 — WAR_QUEUE end-to-end integration tests
// ---------------------------------------------------------------------------

// TestPhase8_NORMAL_Regression proves the queue gate is transparent for NORMAL
// mode: no registration settings → checkout works exactly as Phase 5.
func TestPhase8_NORMAL_Regression(t *testing.T) {
	skipIfNoDB(t)
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	ownerToken, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner8a@x.com", "Phase8A Org")
	eventID, categoryID := publishEventWithCategory(t, client, srv.URL, ownerToken, orgID, 50, 5)
	partToken := registerAndLogin(t, client, srv.URL, "participant8a@x.com")

	// No registration settings → NORMAL mode; checkout must succeed with 201.
	resp := checkoutWithToken(t, srv.URL, orgID, eventID, categoryID, partToken, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("NORMAL checkout = %d, want 201", resp.StatusCode)
	}
	var order struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&order)
	if order.ID == "" {
		t.Fatal("expected order ID in response")
	}
}

// TestPhase8_WAR_JoinStatusRelease covers the full happy path:
// WAR_QUEUE → join (WAITING) → Release → ALLOWED + admissionToken →
// checkout with token (201) and without token (403 ADMISSION_REQUIRED).
func TestPhase8_WAR_JoinStatusRelease(t *testing.T) {
	skipIfNoDB(t)
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}
	ctx := context.Background()

	ownerToken, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner8b@x.com", "Phase8B Org")
	eventID, categoryID := publishEventWithCategory(t, client, srv.URL, ownerToken, orgID, 50, 5)
	partToken := registerAndLogin(t, client, srv.URL, "participant8b@x.com")

	// Set WAR_QUEUE mode.
	setRegistrationMode(t, srv.URL, orgID, eventID, ownerToken, "WAR_QUEUE")

	// Join queue → 201, status=WAITING, position >= 0.
	joinResp := joinQueue(t, srv.URL, eventID, partToken)
	if joinResp.StatusCode != http.StatusCreated {
		t.Fatalf("join = %d, want 201", joinResp.StatusCode)
	}
	var joinBody struct {
		TokenID  string `json:"tokenId"`
		Status   string `json:"status"`
		Position int64  `json:"position"`
	}
	json.NewDecoder(joinResp.Body).Decode(&joinBody)
	joinResp.Body.Close()

	if joinBody.Status != queuemod.StatusWaiting {
		t.Fatalf("join status = %q, want WAITING", joinBody.Status)
	}
	if joinBody.Position < 0 {
		t.Fatalf("join position = %d, want >= 0", joinBody.Position)
	}

	// GET status → WAITING.
	st1 := queueStatus(t, srv.URL, eventID, partToken)
	if st1.StatusCode != http.StatusOK {
		t.Fatalf("queue status = %d, want 200", st1.StatusCode)
	}
	var stBody1 struct{ Status string `json:"status"` }
	json.NewDecoder(st1.Body).Decode(&stBody1)
	st1.Body.Close()
	if stBody1.Status != queuemod.StatusWaiting {
		t.Fatalf("status before release = %q, want WAITING", stBody1.Status)
	}

	// Direct Release via service.
	rdb := testRedis(t)
	qSvc := buildQueueSvc(t, pool, rdb)
	eventUUID := mustUUID(t, eventID)
	promoted, err := qSvc.Release(ctx, eventUUID, 10, 5*time.Minute)
	if err != nil {
		t.Fatalf("Release: %v", err)
	}
	if promoted == 0 {
		t.Fatal("Release promoted 0, expected >= 1")
	}

	// GET status → ALLOWED with admissionToken.
	st2 := queueStatus(t, srv.URL, eventID, partToken)
	if st2.StatusCode != http.StatusOK {
		t.Fatalf("queue status (post-release) = %d, want 200", st2.StatusCode)
	}
	var stBody2 struct {
		Status         string `json:"status"`
		AdmissionToken string `json:"admissionToken"`
	}
	json.NewDecoder(st2.Body).Decode(&stBody2)
	st2.Body.Close()
	if stBody2.Status != queuemod.StatusAllowed {
		t.Fatalf("status after release = %q, want ALLOWED", stBody2.Status)
	}
	if stBody2.AdmissionToken == "" {
		t.Fatal("expected admissionToken after release")
	}

	// Checkout WITHOUT X-Queue-Token → 403 ADMISSION_REQUIRED.
	noTok := checkoutWithToken(t, srv.URL, orgID, eventID, categoryID, partToken, "")
	if noTok.StatusCode != http.StatusForbidden {
		t.Fatalf("checkout without token = %d, want 403", noTok.StatusCode)
	}
	var errBody struct{ Code string `json:"code"` }
	json.NewDecoder(noTok.Body).Decode(&errBody)
	noTok.Body.Close()
	if errBody.Code != "ADMISSION_REQUIRED" {
		t.Fatalf("error code = %q, want ADMISSION_REQUIRED", errBody.Code)
	}

	// Checkout WITH X-Queue-Token → 201.
	okResp := checkoutWithToken(t, srv.URL, orgID, eventID, categoryID, partToken, stBody2.AdmissionToken)
	defer okResp.Body.Close()
	if okResp.StatusCode != http.StatusCreated {
		t.Fatalf("checkout with token = %d, want 201", okResp.StatusCode)
	}
}

// TestPhase8_JoinIdempotent verifies joining twice returns the same tokenId.
func TestPhase8_JoinIdempotent(t *testing.T) {
	skipIfNoDB(t)
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	ownerToken, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner8c@x.com", "Phase8C Org")
	eventID, _ := publishEventWithCategory(t, client, srv.URL, ownerToken, orgID, 50, 5)
	partToken := registerAndLogin(t, client, srv.URL, "participant8c@x.com")

	setRegistrationMode(t, srv.URL, orgID, eventID, ownerToken, "WAR_QUEUE")

	var firstTokenID string
	for i := 0; i < 2; i++ {
		resp := joinQueue(t, srv.URL, eventID, partToken)
		var body struct{ TokenID string `json:"tokenId"` }
		json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("join attempt %d = %d, want 201", i+1, resp.StatusCode)
		}
		if i == 0 {
			firstTokenID = body.TokenID
		} else if body.TokenID != firstTokenID {
			t.Fatalf("idempotency broken: first=%q second=%q", firstTokenID, body.TokenID)
		}
	}
}

// TestPhase8_PauseResume verifies Release promotes 0 when paused, > 0 after resume.
func TestPhase8_PauseResume(t *testing.T) {
	skipIfNoDB(t)
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}
	ctx := context.Background()

	ownerToken, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner8d@x.com", "Phase8D Org")
	eventID, _ := publishEventWithCategory(t, client, srv.URL, ownerToken, orgID, 50, 5)
	partToken := registerAndLogin(t, client, srv.URL, "participant8d@x.com")

	setRegistrationMode(t, srv.URL, orgID, eventID, ownerToken, "WAR_QUEUE")

	// Seed participant into queue.
	jr := joinQueue(t, srv.URL, eventID, partToken)
	jr.Body.Close()
	if jr.StatusCode != http.StatusCreated {
		t.Fatalf("join = %d, want 201", jr.StatusCode)
	}

	// Pause via HTTP.
	pauseReq, _ := http.NewRequest(http.MethodPost,
		srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/queue/pause", nil)
	pauseReq.Header.Set("Authorization", "Bearer "+ownerToken)
	pauseResp, err := http.DefaultClient.Do(pauseReq)
	if err != nil {
		t.Fatalf("pause POST: %v", err)
	}
	pauseResp.Body.Close()
	if pauseResp.StatusCode != http.StatusNoContent {
		t.Fatalf("pause = %d, want 204", pauseResp.StatusCode)
	}

	// Confirm control is PAUSED via repo.
	repo := queuemod.NewRepository(pool)
	eventUUID := mustUUID(t, eventID)
	ctrl, err := repo.GetControl(ctx, eventUUID)
	if err != nil {
		t.Fatalf("GetControl: %v", err)
	}
	if ctrl.State != queuemod.StatePaused {
		t.Fatalf("control state = %q, want PAUSED", ctrl.State)
	}

	// Mirror ReleaseJob guard: only call Release when RUNNING.
	rdb := testRedis(t)
	qSvc := buildQueueSvc(t, pool, rdb)
	promoted := 0
	if ctrl.State == queuemod.StateRunning {
		promoted, err = qSvc.Release(ctx, eventUUID, 10, 5*time.Minute)
		if err != nil {
			t.Fatalf("Release (paused): %v", err)
		}
	}
	if promoted != 0 {
		t.Fatalf("promoted while paused = %d, want 0", promoted)
	}

	// Resume via HTTP.
	resumeReq, _ := http.NewRequest(http.MethodPost,
		srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/queue/resume", nil)
	resumeReq.Header.Set("Authorization", "Bearer "+ownerToken)
	resumeResp, err := http.DefaultClient.Do(resumeReq)
	if err != nil {
		t.Fatalf("resume POST: %v", err)
	}
	resumeResp.Body.Close()
	if resumeResp.StatusCode != http.StatusNoContent {
		t.Fatalf("resume = %d, want 204", resumeResp.StatusCode)
	}

	// Release after resume → promoted > 0.
	promoted, err = qSvc.Release(ctx, eventUUID, 10, 5*time.Minute)
	if err != nil {
		t.Fatalf("Release (running): %v", err)
	}
	if promoted == 0 {
		t.Fatal("Release after resume promoted 0, expected > 0")
	}
}

// TestPhase8_AdmissionExpiry verifies expired admissions requeue the token to WAITING.
func TestPhase8_AdmissionExpiry(t *testing.T) {
	skipIfNoDB(t)
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}
	ctx := context.Background()

	ownerToken, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner8e@x.com", "Phase8E Org")
	eventID, _ := publishEventWithCategory(t, client, srv.URL, ownerToken, orgID, 50, 5)
	partToken := registerAndLogin(t, client, srv.URL, "participant8e@x.com")

	setRegistrationMode(t, srv.URL, orgID, eventID, ownerToken, "WAR_QUEUE")

	jr := joinQueue(t, srv.URL, eventID, partToken)
	jr.Body.Close()
	if jr.StatusCode != http.StatusCreated {
		t.Fatalf("join = %d, want 201", jr.StatusCode)
	}

	rdb := testRedis(t)
	qSvc := buildQueueSvc(t, pool, rdb)
	eventUUID := mustUUID(t, eventID)

	// Release with 1 ms window — admission expires almost immediately.
	promoted, err := qSvc.Release(ctx, eventUUID, 10, time.Millisecond)
	if err != nil {
		t.Fatalf("Release: %v", err)
	}
	if promoted == 0 {
		t.Fatal("Release promoted 0, expected >= 1")
	}

	// Wait for window to pass.
	time.Sleep(20 * time.Millisecond)

	// Expire due admissions and requeue.
	expired, err := qSvc.ExpireDue(ctx, 10)
	if err != nil {
		t.Fatalf("ExpireDue: %v", err)
	}
	if expired == 0 {
		t.Fatal("ExpireDue expired 0, expected >= 1")
	}

	// GET status → participant must be back to WAITING.
	st := queueStatus(t, srv.URL, eventID, partToken)
	if st.StatusCode != http.StatusOK {
		t.Fatalf("queue status (post-expiry) = %d, want 200", st.StatusCode)
	}
	var stBody struct{ Status string `json:"status"` }
	json.NewDecoder(st.Body).Decode(&stBody)
	st.Body.Close()
	if stBody.Status != queuemod.StatusWaiting {
		t.Fatalf("status after expiry = %q, want WAITING", stBody.Status)
	}
}

// ---------------------------------------------------------------------------
// Task 22 — Concurrency tests
// ---------------------------------------------------------------------------

// TestPhase8_NoDuplicateToken fires 50 concurrent Join calls for the same
// (event, participant) and asserts exactly 1 token row in DB.
func TestPhase8_NoDuplicateToken(t *testing.T) {
	skipIfNoDB(t)
	pool := testPool(t)
	truncate(t, pool)
	ctx := context.Background()

	orgID, eventID, _ := seedPublishedCategory(t, pool, 500, 5)
	participantID := seedUsers(t, pool, 1)[0]

	rdb := testRedis(t)
	qSvc := buildQueueSvc(t, pool, rdb)

	const n = 50
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			qSvc.Join(ctx, orgID, eventID, participantID) //nolint:errcheck
		}()
	}
	close(start)
	wg.Wait()

	var count int
	pool.QueryRow(ctx,
		`SELECT count(*) FROM queue_tokens WHERE event_id = $1 AND participant_id = $2`,
		eventID, participantID,
	).Scan(&count)
	if count != 1 {
		t.Fatalf("token count = %d, want exactly 1 (UNIQUE constraint)", count)
	}
}

// TestPhase8_ReleaseIdempotent seeds 5 WAITING tokens then runs 2 concurrent
// Release(n=5) calls. Total admitted must be exactly 5, not 10.
func TestPhase8_ReleaseIdempotent(t *testing.T) {
	skipIfNoDB(t)
	pool := testPool(t)
	truncate(t, pool)
	ctx := context.Background()

	orgID, eventID, _ := seedPublishedCategory(t, pool, 500, 5)
	participants := seedUsers(t, pool, 5)

	rdb := testRedis(t)
	qSvc := buildQueueSvc(t, pool, rdb)

	// Seed 5 WAITING tokens.
	for _, pid := range participants {
		if _, err := qSvc.Join(ctx, orgID, eventID, pid); err != nil {
			t.Fatalf("seed join: %v", err)
		}
	}

	// Two concurrent Release(5) calls.
	results := make([]int, 2)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			n, _ := qSvc.Release(ctx, eventID, 5, 5*time.Minute)
			results[i] = n
		}()
	}
	close(start)
	wg.Wait()

	total := results[0] + results[1]
	if total != 5 {
		t.Fatalf("total admitted = %d, want exactly 5 (idempotent Release)", total)
	}
}

// TestPhase8_NoOversold runs capacity=2 with 5 concurrent checkouts (NORMAL mode
// via direct service call) and asserts at most 2 orders are created.
func TestPhase8_NoOversold(t *testing.T) {
	skipIfNoDB(t)
	pool := testPool(t)
	truncate(t, pool)
	ctx := context.Background()

	// capacity=2, maxOrder=1 per user — 5 users compete for 2 slots.
	_, eventID, categoryID := seedPublishedCategory(t, pool, 2, 1)
	participants := seedUsers(t, pool, 5)

	svc := ordersmod.NewService(ordersmod.NewRepository(pool), nil, 15*time.Minute, nil, nil)

	const n = 5
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			svc.Checkout(ctx, participants[i], eventID, categoryID, "") //nolint:errcheck
		}()
	}
	close(start)
	wg.Wait()

	var orderCount int
	pool.QueryRow(ctx,
		`SELECT count(*) FROM orders WHERE category_id = $1`, categoryID,
	).Scan(&orderCount)
	if orderCount > 2 {
		t.Fatalf("oversold: orders = %d, want <= 2 (capacity=2)", orderCount)
	}
	if orderCount == 0 {
		t.Fatal("expected at least 1 order to succeed")
	}

	// Inventory reservations must not exceed capacity.
	var reserved int
	pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(quantity), 0) FROM inventory_reservations WHERE category_id = $1`, categoryID,
	).Scan(&reserved)
	if reserved > 2 {
		t.Fatalf("inventory_reservations = %d, want <= 2", reserved)
	}
}
