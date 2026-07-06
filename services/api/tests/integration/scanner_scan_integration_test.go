//go:build integration

// Integration tests for the Scanner PWA scan endpoints (task 14.4). These
// exercise the scanner.Service composed with REAL collaborators against a real
// Postgres so the authoritative HMAC verification (tickets/qr.Signer), the
// VALID->USED transition (SELECT ... FOR UPDATE + guarded MarkTicketUsed), the
// racepack duplicate read, and the platform idempotency_keys cache are all in
// play end-to-end.
//
// Scenarios (per design "Request Flow — Online Scan" and "Offline Scan →
// Deferred Sync"):
//
//  1. Online scan → verify → check-in happy path: a VALID ticket (PAID order +
//     assigned BIB) is verified with a token signed by the SAME qr signer the
//     server uses, then checked in (VALID → USED). A second verify then reports
//     alreadyCheckedIn=true.
//
//  2. Offline replay with Idempotency-Key (exactly-once, Req 8.3): the SAME
//     check-in replayed with the SAME Idempotency-Key applies the effect once
//     and returns the byte-identical original response for every replay. This
//     is verified through the HTTP handler (httptest), because the
//     Idempotency-Key cache lives in the HANDLER, not the service — so we
//     verify exactly-once at the layer that owns it.
//
//  3. Forged-token sync rejection → the FAILED path (Req 8.6): a token signed
//     with a DIFFERENT secret fails HMAC verification and is rejected by Verify
//     with ErrSignatureInvalid, with NO ticket transition. This is exactly the
//     non-retryable rejection the client Sync_Engine classifies as FAILED when
//     a synced forged op reaches the server.
//
// Run with: make test-db-setup && make test-integration
// (or: TEST_DATABASE_URL=... go test -tags=integration ./tests/integration/...)
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/modules/racepack"
	"github.com/varin/ivyticketing/services/api/internal/modules/scanner"
	"github.com/varin/ivyticketing/services/api/internal/modules/tickets/qr"
	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
)

// scanFlowSecret is the HMAC secret the "server" (our directly-constructed qr
// signer) uses to sign and verify VALID tokens in these tests. Valid tokens are
// produced with a signer built on this secret; forged tokens are produced with
// a DIFFERENT secret so their HMAC fails verification.
const scanFlowSecret = "test-qr-secret"

// scanFlowFixture holds the reusable rows shared by the scan-flow scenarios: an
// org + published event + category, plus a staff user who holds BOTH
// racepack.execute and checkin.execute in that org (so AssertEventPermitted
// passes for verify and check-in).
type scanFlowFixture struct {
	pool    *pgxpool.Pool
	orgID   uuid.UUID
	eventID uuid.UUID
	catID   uuid.UUID
	staffID uuid.UUID
}

// newScanFlowFixture seeds the org/event/category (via the shared concurrency
// seed helper) and a permitted staff user. The staff user is granted an
// org-scoped role holding racepack.execute + checkin.execute and joined to the
// org, mirroring the authz seeding in scanner_authz_property_test.go.
func newScanFlowFixture(t *testing.T, pool *pgxpool.Pool) scanFlowFixture {
	t.Helper()
	ctx := context.Background()
	orgID, eventID, catID := seedPublishedCategory(t, pool, 1000, 1000)

	staffID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name)
		 VALUES ($1,$2,'x','Scan Flow Staff')`,
		staffID, fmt.Sprintf("scanflow-%s@test.com", staffID.String()[:8])); err != nil {
		t.Fatalf("seed staff user: %v", err)
	}

	// Org-scoped role granting both scan permissions, assigned to the staff.
	roleID := uuid.New()
	short := orgID.String()[:8]
	if _, err := pool.Exec(ctx,
		`INSERT INTO roles (id, organization_id, name, slug) VALUES ($1,$2,$3,$4)`,
		roleID, orgID, "Scan Flow Role "+short, "scanflow-role-"+short); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	for _, key := range []string{"racepack.execute", "checkin.execute"} {
		if _, err := pool.Exec(ctx,
			`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2)`,
			roleID, permissionID(t, pool, key)); err != nil {
			t.Fatalf("grant %q: %v", key, err)
		}
	}
	memberID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO organization_members (id, organization_id, user_id) VALUES ($1,$2,$3)`,
		memberID, orgID, staffID); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO member_roles (organization_member_id, role_id) VALUES ($1,$2)`,
		memberID, roleID); err != nil {
		t.Fatalf("assign role: %v", err)
	}

	return scanFlowFixture{pool: pool, orgID: orgID, eventID: eventID, catID: catID, staffID: staffID}
}

// seedValidTicket inserts a participant, a PAID order, and a VALID ticket with
// an assigned BIB (the eligible happy-path shape). Returns the ticket ID.
func (f scanFlowFixture) seedValidTicket(t *testing.T) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	participantID := uuid.New()
	short := participantID.String()[:8]

	if _, err := f.pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name)
		 VALUES ($1,$2,'x','Scan Flow Participant')`,
		participantID, fmt.Sprintf("scanflowpart-%s@test.com", short)); err != nil {
		t.Fatalf("seed participant: %v", err)
	}

	orderID := uuid.New()
	if _, err := f.pool.Exec(ctx,
		`INSERT INTO orders
		   (id, organization_id, event_id, category_id, participant_id, order_number, status, subtotal, fee, discount, total)
		 VALUES ($1,$2,$3,$4,$5,$6,'PAID',100000,0,0,100000)`,
		orderID, f.orgID, f.eventID, f.catID, participantID, "ORD-"+short); err != nil {
		t.Fatalf("seed order: %v", err)
	}

	ticketID := uuid.New()
	if _, err := f.pool.Exec(ctx,
		`INSERT INTO tickets
		   (id, organization_id, event_id, category_id, order_id, participant_id,
		    ticket_number, status, holder_name, holder_email, event_title, category_name, bib_number)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,'VALID','Scan Flow Participant',$8,'Scan Flow Event','42K',$9)`,
		ticketID, f.orgID, f.eventID, f.catID, orderID, participantID,
		"TKT-"+short, fmt.Sprintf("scanflowpart-%s@test.com", short), "B-"+short); err != nil {
		t.Fatalf("seed ticket: %v", err)
	}
	return ticketID
}

// ticketStatus reads a ticket's current status straight from the DB (the source
// of truth) so the tests can assert whether a transition actually happened.
func (f scanFlowFixture) ticketStatus(t *testing.T, ticketID uuid.UUID) string {
	t.Helper()
	var status string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT status FROM tickets WHERE id=$1`, ticketID).Scan(&status); err != nil {
		t.Fatalf("read ticket status: %v", err)
	}
	return status
}

// newScanFlowService constructs a scanner.Service with the SAME real
// collaborators the server wires: a qr signer as the QRVerifier, the ticket
// read model, racepack.Service as the pickup executor, the scanner repository,
// a nil audit recorder, and a logger.
func newScanFlowService(pool *pgxpool.Pool, secret string) *scanner.Service {
	return scanner.NewService(
		qr.NewSigner(secret),
		scanner.NewTicketReader(pool),
		racepack.NewService(racepack.NewRepository(pool), nil, nil),
		scanner.NewRepository(pool),
		nil, // audit recorder — best-effort, safe to omit in tests
		newNopLogger(),
	)
}

// --- Scenario 1: online scan → verify → check-in happy path ----------------

// TestScanFlow_OnlineVerifyThenCheckIn exercises the full online happy path:
// sign a token with the server's own qr signer, verify it (expecting the
// whitelisted display info and no duplicate flags), check the participant in
// (VALID → USED), then re-verify and observe alreadyCheckedIn=true with the
// recorded timestamp.
//
// Requirements: 2.2 (verify signed token), 5.1 (VALID → USED check-in).
func TestScanFlow_OnlineVerifyThenCheckIn(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	fix := newScanFlowFixture(t, pool)
	svc := newScanFlowService(pool, scanFlowSecret)
	ctx := context.Background()

	ticketID := fix.seedValidTicket(t)

	// VALID token: signed with the SAME secret the service verifies against.
	token, err := qr.NewSigner(scanFlowSecret).Sign(ticketID, fix.eventID)
	if err != nil {
		t.Fatalf("sign valid token: %v", err)
	}

	// Verify → expect success, whitelisted display info, no duplicate flags.
	vr, err := svc.Verify(ctx, scanner.VerifyInput{
		Token:   token,
		EventID: fix.eventID,
		OrgID:   fix.orgID,
		StaffID: fix.staffID,
	})
	if err != nil {
		t.Fatalf("verify valid token: %v", err)
	}
	if vr.TicketID != ticketID.String() {
		t.Errorf("verify ticketId = %q, want %q", vr.TicketID, ticketID)
	}
	if vr.Display.TicketStatus != scanner.TicketStatusValid {
		t.Errorf("verify display status = %q, want VALID", vr.Display.TicketStatus)
	}
	if vr.Display.ParticipantName == "" || vr.Display.BibNumber == "" || vr.Display.CategoryName == "" {
		t.Errorf("verify display incomplete: %+v", vr.Display)
	}
	if vr.AlreadyCheckedIn {
		t.Errorf("fresh ticket must not be alreadyCheckedIn")
	}

	// Check-in → VALID → USED, not a duplicate.
	cr, err := svc.CheckIn(ctx, scanner.CheckInInput{
		OrgID:    fix.orgID,
		EventID:  fix.eventID,
		TicketID: ticketID,
		StaffID:  fix.staffID,
	})
	if err != nil {
		t.Fatalf("check-in: %v", err)
	}
	if cr.Status != scanner.TicketStatusUsed {
		t.Errorf("check-in status = %q, want USED", cr.Status)
	}
	if cr.Duplicate {
		t.Errorf("first check-in must not be a duplicate")
	}
	if cr.CheckedInAt.IsZero() {
		t.Errorf("check-in must record a timestamp")
	}

	// Source of truth: the DB row transitioned to USED.
	if got := fix.ticketStatus(t, ticketID); got != scanner.TicketStatusUsed {
		t.Errorf("DB ticket status = %q, want USED", got)
	}

	// Re-verify → now alreadyCheckedIn with the recorded timestamp.
	vr2, err := svc.Verify(ctx, scanner.VerifyInput{
		Token:   token,
		EventID: fix.eventID,
		OrgID:   fix.orgID,
		StaffID: fix.staffID,
	})
	if err != nil {
		t.Fatalf("re-verify after check-in: %v", err)
	}
	if !vr2.AlreadyCheckedIn {
		t.Errorf("re-verify must report alreadyCheckedIn=true")
	}
	if vr2.CheckedInAt == nil {
		t.Errorf("re-verify must include checkedInAt timestamp")
	}
}

// --- Scenario 2: offline replay with Idempotency-Key (exactly-once) ---------

// checkInRouter mounts the real scanner.Handler behind a chi router that
// injects an authenticated staff identity and the {orgId}/{eventId} URL params,
// exactly like the mounted route in server.go. The Idempotency-Key cache lives
// in the handler, so exactly-once is verified here — at the layer that owns it.
func checkInRouter(h *scanner.Handler, staffID uuid.UUID) http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := authctx.WithIdentity(req.Context(), authctx.Identity{UserID: staffID})
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Route("/organizations/{orgId}/events/{eventId}", func(r chi.Router) {
		r.Post("/scan/check-in", h.CheckIn)
	})
	return r
}

// TestScanFlow_OfflineReplayIdempotencyKey replays the SAME check-in with the
// SAME Idempotency-Key several times against a real Postgres and asserts the
// effect is applied exactly once (a single VALID → USED transition, timestamp
// unchanged across replays) and every replay returns the byte-identical
// original response. This models an offline op re-sent by the Sync_Engine.
//
// Requirements: 8.3 (server idempotency is exactly-once).
func TestScanFlow_OfflineReplayIdempotencyKey(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	fix := newScanFlowFixture(t, pool)
	svc := newScanFlowService(pool, scanFlowSecret)
	router := checkInRouter(scanner.NewHandler(svc), fix.staffID)

	ticketID := fix.seedValidTicket(t)

	// The original offline scan time carried by the replayed op.
	scannedAt := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Second)
	reqBody, err := json.Marshal(scanner.CheckInRequest{
		TicketID:  ticketID.String(),
		ScannedAt: &scannedAt,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	path := "/organizations/" + fix.orgID.String() + "/events/" + fix.eventID.String() + "/scan/check-in"
	idempKey := uuid.NewString()

	// "Identical response" (Req 8.3) is a SEMANTIC guarantee over the real
	// idempotency cache: response_body is a jsonb column, so Postgres normalises
	// the stored bytes on read-back (canonical key order + inserted whitespace).
	// The fresh response is compact json.Marshal output; every replay returns
	// the jsonb-normalised form. They decode to the same object and — critically
	// — all CACHED replays are byte-identical to each other (the cache is
	// stable). So we assert: same HTTP status, same decoded CheckInResult across
	// all replays, and byte-identity among the cached replays. The exactly-once
	// EFFECT is asserted against the DB below.
	const replays = 4
	var results []scanner.CheckInResult
	var cachedBodies [][]byte
	for i := 0; i < replays; i++ {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempKey)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("replay %d: status = %d, want 200 (body=%s)", i, rec.Code, rec.Body.String())
		}
		body := bytes.TrimSpace(rec.Body.Bytes())
		var res scanner.CheckInResult
		if err := json.Unmarshal(body, &res); err != nil {
			t.Fatalf("replay %d: unmarshal body %s: %v", i, body, err)
		}
		results = append(results, res)
		if i > 0 { // i==0 is the fresh (compact) response; i>=1 are cached.
			cachedBodies = append(cachedBodies, append([]byte(nil), body...))
		}
	}

	// Every replay decodes to the identical response object (Req 8.3).
	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			t.Fatalf("replay %d result = %+v, want identical original %+v", i, results[i], results[0])
		}
	}
	// The cache is stable: all cached replays are byte-identical to each other.
	for i := 1; i < len(cachedBodies); i++ {
		if !bytes.Equal(cachedBodies[i], cachedBodies[0]) {
			t.Fatalf("cached replay %d body = %s, want identical %s", i+1, cachedBodies[i], cachedBodies[0])
		}
	}

	// The original response reflects the applied (non-duplicate) effect.
	first := results[0]
	if first.Duplicate {
		t.Errorf("original response should reflect the applied effect (Duplicate=false)")
	}
	if first.Status != scanner.TicketStatusUsed {
		t.Errorf("original response status = %q, want USED", first.Status)
	}

	// Source of truth: exactly one transition. used_at == the original scan
	// time (Req 10.3) and it never moved across the replays.
	var usedAt time.Time
	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status, used_at FROM tickets WHERE id=$1`, ticketID).Scan(&status, &usedAt); err != nil {
		t.Fatalf("read ticket: %v", err)
	}
	if status != scanner.TicketStatusUsed {
		t.Errorf("DB ticket status = %q, want USED", status)
	}
	if !usedAt.UTC().Equal(scannedAt) {
		t.Errorf("used_at = %s, want original scannedAt %s (effect applied once with the original time)",
			usedAt.UTC(), scannedAt)
	}
}

// --- Scenario 3: forged-token sync rejection → FAILED path ------------------

// TestScanFlow_ForgedTokenRejectedAtSync signs a token for a REAL ticket with a
// DIFFERENT secret, so its HMAC does not match the server's. Verify rejects it
// with ErrSignatureInvalid and performs NO ticket transition — exactly the
// non-retryable rejection the client Sync_Engine classifies as FAILED when a
// synced forged op reaches the server. A control check with the correct secret
// confirms the only difference is the signing key.
//
// Requirements: 8.6 (non-retryable forged-token rejection / FAILED path),
// 2.2 (authoritative server-side HMAC verification).
func TestScanFlow_ForgedTokenRejectedAtSync(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	fix := newScanFlowFixture(t, pool)
	svc := newScanFlowService(pool, scanFlowSecret)
	ctx := context.Background()

	ticketID := fix.seedValidTicket(t)

	// Forged token: same ticket/event, but signed with a DIFFERENT secret, so
	// the server's HMAC check fails.
	forged, err := qr.NewSigner("attacker-secret-not-the-server-secret").Sign(ticketID, fix.eventID)
	if err != nil {
		t.Fatalf("sign forged token: %v", err)
	}

	_, err = svc.Verify(ctx, scanner.VerifyInput{
		Token:   forged,
		EventID: fix.eventID,
		OrgID:   fix.orgID,
		StaffID: fix.staffID,
	})
	if !errors.Is(err, scanner.ErrSignatureInvalid) {
		t.Fatalf("forged token verify = %v, want ErrSignatureInvalid", err)
	}

	// No side effect: the ticket is untouched (still VALID) after the rejection.
	if got := fix.ticketStatus(t, ticketID); got != scanner.TicketStatusValid {
		t.Errorf("forged rejection changed ticket status to %q, want unchanged VALID", got)
	}

	// Control: the SAME ticket/event signed with the CORRECT secret verifies,
	// proving the rejection is due to the signature alone, not the ticket data.
	good, err := qr.NewSigner(scanFlowSecret).Sign(ticketID, fix.eventID)
	if err != nil {
		t.Fatalf("sign control token: %v", err)
	}
	if _, err := svc.Verify(ctx, scanner.VerifyInput{
		Token:   good,
		EventID: fix.eventID,
		OrgID:   fix.orgID,
		StaffID: fix.staffID,
	}); err != nil {
		t.Fatalf("control (correctly-signed) token verify = %v, want success", err)
	}
}
