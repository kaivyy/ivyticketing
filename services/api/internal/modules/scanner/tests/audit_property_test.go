package scanner_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"pgregory.net/rapid"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/scanner"
	"github.com/varin/ivyticketing/services/api/internal/modules/tickets/qr"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
)

// -----------------------------------------------------------------------------
// capturingAudit is an in-memory scanner.AuditRecorder that records every
// audit.Entry the service writes, so the audit properties (18, 19) and the
// rejection unit test (6.9) can assert on the written entries without a
// database. It satisfies scanner.AuditRecorder exactly:
//
//	Record(ctx context.Context, e audit.Entry)
//
// The platform audit.Logger implements the same method against Postgres; here
// we simply keep the entries in a slice. A mutex guards the slice so it is safe
// even if the service ever records concurrently.
type capturingAudit struct {
	mu      sync.Mutex
	entries []audit.Entry
}

func (c *capturingAudit) Record(_ context.Context, e audit.Entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, e)
}

// snapshot returns a copy of the captured entries.
func (c *capturingAudit) snapshot() []audit.Entry {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]audit.Entry, len(c.entries))
	copy(out, c.entries)
	return out
}

// entriesWithAction returns the captured entries whose Action matches action.
func (c *capturingAudit) entriesWithAction(action string) []audit.Entry {
	var out []audit.Entry
	for _, e := range c.snapshot() {
		if e.Action == action {
			out = append(out, e)
		}
	}
	return out
}

// Compile-time assertion that capturingAudit satisfies the interface the
// service depends on. If the interface changes, this fails to compile rather
// than silently drifting.
var _ scanner.AuditRecorder = (*capturingAudit)(nil)

// pastTimeGen draws a timestamp strictly in the past (years ~2001–2020) so it
// is guaranteed distinct from server "now". Used by Property 19 to prove the
// audit "at" is the original scan time in the offline case and server-now in
// the online case.
func pastTimeGen(label string) *rapid.Generator[time.Time] {
	return rapid.Custom(func(t *rapid.T) time.Time {
		sec := rapid.Int64Range(1_000_000_000, 1_600_000_000).Draw(t, label)
		return time.Unix(sec, 0).UTC()
	})
}

// Feature: scanner-pwa, Property 18: Audit content completeness
//
// For any recorded pickup OR check-in, an audit entry SHALL be written
// containing the ticket identifier, the staff identifier, and a timestamp.
//
// This property validates the SCANNER side: for any successful CheckIn
// (VALID->USED transition) across random staff/event/ticket/timestamp inputs,
// EXACTLY ONE SCANNER_CHECKIN_COMPLETED audit entry is captured, and that entry
// carries:
//   - the staff identifier   (ActorUserID == staffID),
//   - the ticket identifier  (TargetID == ticketID AND metadata["ticket_id"]),
//   - a timestamp            (metadata["at"] present and RFC3339-parseable).
//
// The pickup side is audited by racepack.ExecutePickup
// (RACEPACK_PICKUP_COMPLETED) and is covered by racepack's own tests; the
// scanner composes that endpoint unchanged. The scanner-side check-in audit is
// what this property validates here.
//
// Validates: Requirements 10.1, 10.2
func TestProperty_AuditContentCompleteness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		orgID := uuidGen().Draw(t, "orgID")
		eventID := uuidGen().Draw(t, "eventID")
		ticketID := uuidGen().Draw(t, "ticketID")
		staffID := uuidGen().Draw(t, "staffID")

		scannedAt := timeGen("scannedAt").Draw(t, "scannedAtVal")
		serverNow := timeGen("serverNow").Draw(t, "serverNowVal")

		repo := newStatefulRepo(serverNow)
		repo.putTicket(db.Ticket{
			ID:      ticketID,
			EventID: eventID,
			Status:  scanner.TicketStatusValid,
		})

		rec := &capturingAudit{}
		// CheckIn only touches repo + audit; the QR/reader/pickup collaborators
		// are unused for the transition path. UserCanScanEvent returns true on
		// the statefulRepo so authorization passes.
		svc := scanner.NewService(nil, nil, nil, repo, rec, nil)

		res, err := svc.CheckIn(context.Background(), scanner.CheckInInput{
			OrgID:     orgID,
			EventID:   eventID,
			TicketID:  ticketID,
			StaffID:   staffID,
			ScannedAt: &scannedAt,
		})
		if err != nil {
			t.Fatalf("CheckIn returned unexpected error: %v", err)
		}
		if res.Duplicate {
			t.Fatalf("expected a real VALID->USED transition, got Duplicate=true")
		}

		// Exactly one check-in audit entry must have been written.
		completed := rec.entriesWithAction(scanner.AuditActionCheckinCompleted)
		if len(completed) != 1 {
			t.Fatalf("expected exactly one %s audit entry, got %d",
				scanner.AuditActionCheckinCompleted, len(completed))
		}
		e := completed[0]

		// Staff identifier: ActorUserID must equal the acting staff.
		if e.ActorUserID == nil {
			t.Fatalf("audit entry missing ActorUserID (staff identifier)")
		}
		if *e.ActorUserID != staffID {
			t.Fatalf("audit ActorUserID = %v, want staffID %v", *e.ActorUserID, staffID)
		}

		// Ticket identifier: present both as the audit target and in metadata.
		if e.TargetID != ticketID.String() {
			t.Fatalf("audit TargetID = %q, want ticketID %q", e.TargetID, ticketID.String())
		}
		gotTicket, ok := e.Metadata["ticket_id"].(string)
		if !ok {
			t.Fatalf("audit metadata missing string ticket_id (metadata=%v)", e.Metadata)
		}
		if gotTicket != ticketID.String() {
			t.Fatalf("audit metadata ticket_id = %q, want %q", gotTicket, ticketID.String())
		}

		// Timestamp: metadata "at" must be present and RFC3339-parseable.
		atStr, ok := e.Metadata["at"].(string)
		if !ok {
			t.Fatalf("audit metadata missing string \"at\" timestamp (metadata=%v)", e.Metadata)
		}
		if _, err := time.Parse(time.RFC3339, atStr); err != nil {
			t.Fatalf("audit metadata \"at\" = %q is not RFC3339-parseable: %v", atStr, err)
		}
	})
}

// Feature: scanner-pwa, Property 19: Offline-synced audit uses the original scan timestamp
//
// For any offline-synced check-in carrying a ScannedAt, the captured audit
// entry's metadata "at" SHALL equal the ORIGINAL scan time (formatted as the
// code does: RFC3339 in UTC), NOT the sync/server time. Conversely, when
// ScannedAt is nil (an online scan), the "at" SHALL be a server-now time — not
// the supplied original.
//
// The test draws a random ScannedAt strictly in the past (guaranteed distinct
// from server "now") and runs two independent check-ins:
//   1. offline-synced: ScannedAt provided -> "at" == original scan time;
//   2. online:         ScannedAt nil       -> "at" within the server-now window
//                       and != the original scan time.
//
// Validates: Requirements 10.3
func TestProperty_OfflineSyncedAuditUsesOriginalTimestamp(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		orgID := uuidGen().Draw(t, "orgID")
		eventID := uuidGen().Draw(t, "eventID")
		staffID := uuidGen().Draw(t, "staffID")

		// Original scan time, strictly in the past -> distinct from "now".
		originalScan := pastTimeGen("originalScan").Draw(t, "originalScanVal")
		wantAt := originalScan.UTC().Format(time.RFC3339)

		// serverNow only drives the repo's used_at fallback; the audit "at" for
		// the online case comes from the real wall clock (time.Now), so we
		// bracket the call to assert the recorded time lands in that window.
		serverNow := timeGen("serverNow").Draw(t, "serverNowVal")

		// --- Case 1: offline-synced op carrying the original scan timestamp ---
		offlineTicket := uuidGen().Draw(t, "offlineTicket")
		offlineRepo := newStatefulRepo(serverNow)
		offlineRepo.putTicket(db.Ticket{
			ID:      offlineTicket,
			EventID: eventID,
			Status:  scanner.TicketStatusValid,
		})
		offlineRec := &capturingAudit{}
		offlineSvc := scanner.NewService(nil, nil, nil, offlineRepo, offlineRec, nil)

		if _, err := offlineSvc.CheckIn(context.Background(), scanner.CheckInInput{
			OrgID:     orgID,
			EventID:   eventID,
			TicketID:  offlineTicket,
			StaffID:   staffID,
			ScannedAt: &originalScan,
		}); err != nil {
			t.Fatalf("offline CheckIn error: %v", err)
		}

		offlineEntries := offlineRec.entriesWithAction(scanner.AuditActionCheckinCompleted)
		if len(offlineEntries) != 1 {
			t.Fatalf("offline: expected 1 check-in audit entry, got %d", len(offlineEntries))
		}
		offlineAt, ok := offlineEntries[0].Metadata["at"].(string)
		if !ok {
			t.Fatalf("offline: audit metadata missing string \"at\"")
		}
		if offlineAt != wantAt {
			t.Fatalf("offline audit \"at\" = %q, want original scan time %q (must NOT be sync time)",
				offlineAt, wantAt)
		}

		// --- Case 2: online op with no ScannedAt -> server-now, not original ---
		onlineTicket := uuidGen().Draw(t, "onlineTicket")
		onlineRepo := newStatefulRepo(serverNow)
		onlineRepo.putTicket(db.Ticket{
			ID:      onlineTicket,
			EventID: eventID,
			Status:  scanner.TicketStatusValid,
		})
		onlineRec := &capturingAudit{}
		onlineSvc := scanner.NewService(nil, nil, nil, onlineRepo, onlineRec, nil)

		before := time.Now().UTC().Add(-2 * time.Second)
		if _, err := onlineSvc.CheckIn(context.Background(), scanner.CheckInInput{
			OrgID:     orgID,
			EventID:   eventID,
			TicketID:  onlineTicket,
			StaffID:   staffID,
			ScannedAt: nil,
		}); err != nil {
			t.Fatalf("online CheckIn error: %v", err)
		}
		after := time.Now().UTC().Add(2 * time.Second)

		onlineEntries := onlineRec.entriesWithAction(scanner.AuditActionCheckinCompleted)
		if len(onlineEntries) != 1 {
			t.Fatalf("online: expected 1 check-in audit entry, got %d", len(onlineEntries))
		}
		onlineAtStr, ok := onlineEntries[0].Metadata["at"].(string)
		if !ok {
			t.Fatalf("online: audit metadata missing string \"at\"")
		}
		onlineAt, err := time.Parse(time.RFC3339, onlineAtStr)
		if err != nil {
			t.Fatalf("online: audit \"at\" = %q not RFC3339-parseable: %v", onlineAtStr, err)
		}
		// The online timestamp must be server-now (within the bracketed window)...
		if onlineAt.Before(before) || onlineAt.After(after) {
			t.Fatalf("online audit \"at\" = %q not within server-now window [%v, %v]",
				onlineAtStr, before, after)
		}
		// ...and it must NOT be the supplied original scan time.
		if onlineAtStr == wantAt {
			t.Fatalf("online audit \"at\" = %q must be server-now, not the original scan time", onlineAtStr)
		}
	})
}

// Feature: scanner-pwa, unit test (Property 18 companion): invalid-signature
// online scan writes a rejection audit.
//
// Example (non-property) test: a Verify call whose QRVerifier returns
// qr.ErrInvalidSignature results in EXACTLY ONE SCANNER_QR_REJECTED audit entry
// captured (with the rejection reason and the selected event_id in metadata),
// and Verify returns ErrSignatureInvalid with no VerifyResult.
//
// Validates: Requirements 10.4
func TestUnit_InvalidSignatureOnlineScanWritesRejectionAudit(t *testing.T) {
	orgID := uuid.New()
	eventID := uuid.New()
	staffID := uuid.New()
	ticketID := uuid.New()

	// The QR verifier rejects the token with an invalid-signature error, as it
	// would for a forged/tampered token on an online scan.
	qrv := fakeQR{err: qr.ErrInvalidSignature}
	// fakeRepo.UserCanScanEvent returns true so authorization passes and Verify
	// reaches the qr.Verify step (where the rejection + audit happen).
	repo := &fakeRepo{ticket: db.Ticket{ID: ticketID, EventID: eventID, Status: scanner.TicketStatusValid}}
	reader := &fakeTicketReader{}
	rec := &capturingAudit{}

	svc := scanner.NewService(qrv, reader, fakePickup{}, repo, rec, nil)

	res, err := svc.Verify(context.Background(), scanner.VerifyInput{
		Token:   "forged.token.value",
		EventID: eventID,
		OrgID:   orgID,
		StaffID: staffID,
	})

	// Verify returns ErrSignatureInvalid with no VerifyResult.
	if !errors.Is(err, scanner.ErrSignatureInvalid) {
		t.Fatalf("expected ErrSignatureInvalid, got %v", err)
	}
	if res != (scanner.VerifyResult{}) {
		t.Fatalf("expected zero VerifyResult on rejection, got %+v", res)
	}
	// No display read should have happened on a signature rejection.
	if reader.calls != 0 {
		t.Fatalf("GetDisplayInfo must not be called on signature rejection (calls=%d)", reader.calls)
	}

	// Exactly one SCANNER_QR_REJECTED audit entry, with reason + event_id.
	rejected := rec.entriesWithAction(scanner.AuditActionQRRejected)
	if len(rejected) != 1 {
		t.Fatalf("expected exactly one %s audit entry, got %d",
			scanner.AuditActionQRRejected, len(rejected))
	}
	e := rejected[0]

	if e.ActorUserID == nil || *e.ActorUserID != staffID {
		t.Fatalf("rejection audit ActorUserID = %v, want staffID %v", e.ActorUserID, staffID)
	}
	if e.OrganizationID == nil || *e.OrganizationID != orgID {
		t.Fatalf("rejection audit OrganizationID = %v, want orgID %v", e.OrganizationID, orgID)
	}

	reason, ok := e.Metadata["reason"].(string)
	if !ok {
		t.Fatalf("rejection audit metadata missing string \"reason\" (metadata=%v)", e.Metadata)
	}
	if reason != scanner.ErrSignatureInvalid.Error() {
		t.Fatalf("rejection audit reason = %q, want %q", reason, scanner.ErrSignatureInvalid.Error())
	}

	gotEvent, ok := e.Metadata["event_id"].(string)
	if !ok {
		t.Fatalf("rejection audit metadata missing string \"event_id\" (metadata=%v)", e.Metadata)
	}
	if gotEvent != eventID.String() {
		t.Fatalf("rejection audit event_id = %q, want %q", gotEvent, eventID.String())
	}

	// A rejected online scan must never transition a ticket.
	if repo.getTicketCalls != 0 {
		t.Fatalf("GetTicketByID must not be called on signature rejection (calls=%d)", repo.getTicketCalls)
	}
}
