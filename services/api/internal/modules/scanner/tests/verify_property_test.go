package scanner_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"pgregory.net/rapid"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/racepack"
	"github.com/varin/ivyticketing/services/api/internal/modules/scanner"
	"github.com/varin/ivyticketing/services/api/internal/modules/tickets/qr"
)

// -----------------------------------------------------------------------------
// In-memory fakes for the scanner.Service collaborators. They let the pure-logic
// properties (3, 5, 6) and the flag-computation property (7) exercise
// Service.Verify deterministically without a database.
// -----------------------------------------------------------------------------

// fakeQR is a controllable QRVerifier: it returns a fixed TicketRef/err pair.
type fakeQR struct {
	ref qr.TicketRef
	err error
}

func (f fakeQR) Verify(_ string) (qr.TicketRef, error) { return f.ref, f.err }

// fakeTicketReader returns a fixed DisplayInfo and records how many times it was
// called so a property can assert that no display read happens on early
// rejection.
type fakeTicketReader struct {
	info  scanner.DisplayInfo
	err   error
	calls int
}

func (f *fakeTicketReader) GetDisplayInfo(_ context.Context, _ uuid.UUID) (scanner.DisplayInfo, error) {
	f.calls++
	return f.info, f.err
}

// fakePickup is a controllable PickupExecutor.
type fakePickup struct {
	rec db.RacepackPickupRecord
	err error
}

func (f fakePickup) GetPickupStatusByTicket(_ context.Context, _ uuid.UUID) (db.RacepackPickupRecord, error) {
	return f.rec, f.err
}

// fakeRepo implements scanner.Repository in memory. Only GetTicketByID is
// exercised by Verify; the remaining methods satisfy the interface and are
// stubbed since CheckIn is out of scope for these properties.
type fakeRepo struct {
	ticket         db.Ticket
	ticketErr      error
	getTicketCalls int
}

func (f *fakeRepo) ExecTx(ctx context.Context, fn func(scanner.Repository) error) error {
	return fn(f)
}

func (f *fakeRepo) GetTicketByID(_ context.Context, _ uuid.UUID) (db.Ticket, error) {
	f.getTicketCalls++
	return f.ticket, f.ticketErr
}

func (f *fakeRepo) LockTicketForUpdate(_ context.Context, _ uuid.UUID) (db.Ticket, error) {
	return db.Ticket{}, nil
}

func (f *fakeRepo) MarkTicketUsed(_ context.Context, _ db.MarkTicketUsedParams) (db.Ticket, error) {
	return db.Ticket{}, nil
}

func (f *fakeRepo) GetEventOrganizationID(_ context.Context, _ uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}

func (f *fakeRepo) ListScannableEventsForUser(_ context.Context, _ uuid.UUID) ([]db.ListScannableEventsForUserRow, error) {
	return nil, nil
}

// UserCanScanEvent is permissive here: these Verify properties exercise QR /
// display / duplicate-flag logic, not authorization (Property 4 covers authz
// separately), so the staff is always treated as permitted for the event.
func (f *fakeRepo) UserCanScanEvent(_ context.Context, _ uuid.UUID, _ uuid.UUID) (bool, error) {
	return true, nil
}

func (f *fakeRepo) GetIdempotencyKey(_ context.Context, _ db.GetIdempotencyKeyParams) (db.GetIdempotencyKeyRow, error) {
	return db.GetIdempotencyKeyRow{}, nil
}

func (f *fakeRepo) InsertIdempotencyKey(_ context.Context, _ db.InsertIdempotencyKeyParams) (db.IdempotencyKey, error) {
	return db.IdempotencyKey{}, nil
}

// uuidGen draws a random UUID from 16 random bytes so rapid controls the input
// space and can shrink counterexamples.
func uuidGen() *rapid.Generator[uuid.UUID] {
	return rapid.Custom(func(t *rapid.T) uuid.UUID {
		var b [16]byte
		for i := range b {
			b[i] = rapid.Byte().Draw(t, "uuidByte")
		}
		return uuid.UUID(b)
	})
}

// timeGen draws a plausible timestamp (seconds resolution, UTC) so pgtype/JSON
// round-trips compare cleanly with time.Time.Equal.
func timeGen(label string) *rapid.Generator[time.Time] {
	return rapid.Custom(func(t *rapid.T) time.Time {
		sec := rapid.Int64Range(1_000_000_000, 2_000_000_000).Draw(t, label)
		return time.Unix(sec, 0).UTC()
	})
}

// Feature: scanner-pwa, Property 3: Event-mismatch rejection
//
// For any token whose embedded event_id differs from the selected
// Permitted_Event, Verify SHALL be rejected with ErrEventMismatch and SHALL NOT
// leak a VerifyResult or reach the display/ticket reads (no side effect).
//
// Validates: Requirements 2.5
func TestProperty_EventMismatchRejection(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		tokenEventID := uuidGen().Draw(t, "tokenEventID")
		selectedEventID := uuidGen().Draw(t, "selectedEventID")
		// Guarantee the embedded event differs from the selected event.
		if tokenEventID == selectedEventID {
			selectedEventID[0] ^= 0xFF
		}
		ticketID := uuidGen().Draw(t, "ticketID")

		qrv := fakeQR{ref: qr.TicketRef{TicketID: ticketID, EventID: tokenEventID, Version: qr.CurrentVersion}}
		reader := &fakeTicketReader{info: scanner.DisplayInfo{}}
		pickup := fakePickup{err: racepack.ErrTicketNotFound}
		repo := &fakeRepo{ticket: db.Ticket{ID: ticketID, EventID: tokenEventID, Status: scanner.TicketStatusValid}}

		svc := scanner.NewService(qrv, reader, pickup, repo, nil, nil)

		res, err := svc.Verify(context.Background(), scanner.VerifyInput{
			Token:   "token",
			EventID: selectedEventID,
			OrgID:   uuidGen().Draw(t, "orgID"),
			StaffID: uuidGen().Draw(t, "staffID"),
		})

		if !errors.Is(err, scanner.ErrEventMismatch) {
			t.Fatalf("expected ErrEventMismatch, got %v", err)
		}
		if res != (scanner.VerifyResult{}) {
			t.Fatalf("expected zero VerifyResult on mismatch, got %+v", res)
		}
		// No side effects: an event mismatch is rejected before any read.
		if reader.calls != 0 {
			t.Fatalf("GetDisplayInfo must not be called on event mismatch (calls=%d)", reader.calls)
		}
		if repo.getTicketCalls != 0 {
			t.Fatalf("GetTicketByID must not be called on event mismatch (calls=%d)", repo.getTicketCalls)
		}
	})
}

// Feature: scanner-pwa, Property 5: Display information completeness
//
// For any validated ticket, VerifyResult.Display SHALL include the participant
// name, BIB number, category, and current ticket status sourced from the
// TicketReader.
//
// Validates: Requirements 3.1
func TestProperty_DisplayInformationCompleteness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		eventID := uuidGen().Draw(t, "eventID")
		ticketID := uuidGen().Draw(t, "ticketID")

		status := rapid.SampledFrom([]string{
			scanner.TicketStatusValid,
			scanner.TicketStatusUsed,
			scanner.TicketStatusCancelled,
		}).Draw(t, "status")

		info := scanner.DisplayInfo{
			ParticipantName: rapid.StringMatching(`[A-Za-z][A-Za-z ]{0,30}`).Draw(t, "name"),
			BibNumber:       rapid.StringMatching(`[0-9]{0,6}`).Draw(t, "bib"),
			CategoryName:    rapid.StringMatching(`[A-Za-z][A-Za-z0-9 ]{0,20}`).Draw(t, "category"),
			TicketStatus:    status,
		}

		qrv := fakeQR{ref: qr.TicketRef{TicketID: ticketID, EventID: eventID, Version: qr.CurrentVersion}}
		reader := &fakeTicketReader{info: info}
		pickup := fakePickup{err: racepack.ErrTicketNotFound}
		repo := &fakeRepo{ticket: db.Ticket{ID: ticketID, EventID: eventID, Status: status}}

		svc := scanner.NewService(qrv, reader, pickup, repo, nil, nil)

		res, err := svc.Verify(context.Background(), scanner.VerifyInput{Token: "token", EventID: eventID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// All four whitelisted display fields must be present and sourced from
		// the TicketReader projection.
		if res.Display != info {
			t.Fatalf("display mismatch: got %+v, want %+v", res.Display, info)
		}
		if res.Display.ParticipantName != info.ParticipantName {
			t.Fatalf("participant name not sourced from ticket: got %q, want %q", res.Display.ParticipantName, info.ParticipantName)
		}
		if res.Display.BibNumber != info.BibNumber {
			t.Fatalf("bib not sourced from ticket: got %q, want %q", res.Display.BibNumber, info.BibNumber)
		}
		if res.Display.CategoryName != info.CategoryName {
			t.Fatalf("category not sourced from ticket: got %q, want %q", res.Display.CategoryName, info.CategoryName)
		}
		if res.Display.TicketStatus != info.TicketStatus {
			t.Fatalf("status not sourced from ticket: got %q, want %q", res.Display.TicketStatus, info.TicketStatus)
		}
	})
}

// whitelistedVerifyKeys is the complete set of JSON object keys the VerifyResult
// (and its nested DisplayInfo) is permitted to serialize. Any key outside this
// set is a leak.
var whitelistedVerifyKeys = map[string]bool{
	// VerifyResult
	"ticketId":         true,
	"eventId":          true,
	"display":          true,
	"alreadyPickedUp":  true,
	"pickedUpAt":       true,
	"alreadyCheckedIn": true,
	"checkedInAt":      true,
	// DisplayInfo
	"participantName": true,
	"bibNumber":       true,
	"category":        true,
	"ticketStatus":    true,
}

// forbiddenFieldSubstrings are substrings that must never appear in any key of
// the serialized verify result (Req 3.4: no card data, passwords, contact
// details).
var forbiddenFieldSubstrings = []string{"email", "phone", "password", "payment", "card"}

// collectJSONKeys recursively gathers every object key found in a decoded JSON
// value.
func collectJSONKeys(v any, out *[]string) {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			*out = append(*out, k)
			collectJSONKeys(child, out)
		}
	case []any:
		for _, child := range t {
			collectJSONKeys(child, out)
		}
	}
}

// Feature: scanner-pwa, Property 6: No sensitive data in display
//
// For any validated ticket, the serialized verify result SHALL contain only
// whitelisted display fields and SHALL NOT contain payment card data,
// passwords, or full contact details.
//
// Validates: Requirements 3.4
func TestProperty_NoSensitiveDataInDisplay(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		eventID := uuidGen().Draw(t, "eventID")
		ticketID := uuidGen().Draw(t, "ticketID")

		status := rapid.SampledFrom([]string{
			scanner.TicketStatusValid,
			scanner.TicketStatusUsed,
			scanner.TicketStatusCancelled,
		}).Draw(t, "status")

		info := scanner.DisplayInfo{
			ParticipantName: rapid.StringMatching(`[A-Za-z][A-Za-z ]{0,30}`).Draw(t, "name"),
			BibNumber:       rapid.StringMatching(`[0-9]{0,6}`).Draw(t, "bib"),
			CategoryName:    rapid.StringMatching(`[A-Za-z][A-Za-z0-9 ]{0,20}`).Draw(t, "category"),
			TicketStatus:    status,
		}

		// Vary the check-in timestamp so the pickedUpAt/checkedInAt keys appear
		// on some iterations (they are omitempty).
		var ticketUsedAt pgtype.Timestamptz
		if status == scanner.TicketStatusUsed {
			ticketUsedAt = pgtype.Timestamptz{Time: timeGen("usedAt").Draw(t, "usedAtVal"), Valid: true}
		}

		var pickup fakePickup
		if rapid.Bool().Draw(t, "hasPickup") {
			pickup = fakePickup{rec: db.RacepackPickupRecord{
				TicketID:        ticketID,
				Status:          scanner.PickupRecordStatusPickedUp,
				PickupTimestamp: pgtype.Timestamptz{Time: timeGen("pickupAt").Draw(t, "pickupAtVal"), Valid: true},
			}}
		} else {
			pickup = fakePickup{err: racepack.ErrTicketNotFound}
		}

		qrv := fakeQR{ref: qr.TicketRef{TicketID: ticketID, EventID: eventID, Version: qr.CurrentVersion}}
		reader := &fakeTicketReader{info: info}
		repo := &fakeRepo{ticket: db.Ticket{ID: ticketID, EventID: eventID, Status: status, UsedAt: ticketUsedAt}}

		svc := scanner.NewService(qrv, reader, pickup, repo, nil, nil)

		res, err := svc.Verify(context.Background(), scanner.VerifyInput{Token: "token", EventID: eventID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		raw, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("marshal VerifyResult: %v", err)
		}
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("unmarshal VerifyResult: %v", err)
		}

		var keys []string
		collectJSONKeys(decoded, &keys)

		for _, k := range keys {
			if !whitelistedVerifyKeys[k] {
				t.Fatalf("verify result JSON leaked non-whitelisted key %q (json=%s)", k, raw)
			}
			lower := strings.ToLower(k)
			for _, bad := range forbiddenFieldSubstrings {
				if strings.Contains(lower, bad) {
					t.Fatalf("verify result JSON contains forbidden field %q (json=%s)", k, raw)
				}
			}
		}
	})
}

// Feature: scanner-pwa, Property 7: Duplicate flags and original timestamps
//
// For any ticket, VerifyResult.alreadyPickedUp SHALL be true exactly when an
// active PICKED_UP record exists (returning that record's timestamp), and
// alreadyCheckedIn SHALL be true exactly when ticket status == USED (returning
// the used_at timestamp).
//
// Validates: Requirements 3.2, 3.3, 6.1, 6.4
func TestProperty_DuplicateFlagsAndTimestamps(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		eventID := uuidGen().Draw(t, "eventID")
		ticketID := uuidGen().Draw(t, "ticketID")

		status := rapid.SampledFrom([]string{
			scanner.TicketStatusValid,
			scanner.TicketStatusUsed,
			scanner.TicketStatusCancelled,
		}).Draw(t, "status")

		usedAt := timeGen("usedAt").Draw(t, "usedAtVal")
		var ticketUsedAt pgtype.Timestamptz
		if status == scanner.TicketStatusUsed {
			ticketUsedAt = pgtype.Timestamptz{Time: usedAt, Valid: true}
		}
		ticket := db.Ticket{ID: ticketID, EventID: eventID, Status: status, UsedAt: ticketUsedAt}

		hasPickup := rapid.Bool().Draw(t, "hasPickup")
		pickupAt := timeGen("pickupAt").Draw(t, "pickupAtVal")
		var pickup fakePickup
		if hasPickup {
			pickup = fakePickup{rec: db.RacepackPickupRecord{
				TicketID:        ticketID,
				Status:          scanner.PickupRecordStatusPickedUp,
				PickupTimestamp: pgtype.Timestamptz{Time: pickupAt, Valid: true},
			}}
		} else {
			pickup = fakePickup{err: racepack.ErrTicketNotFound}
		}

		qrv := fakeQR{ref: qr.TicketRef{TicketID: ticketID, EventID: eventID, Version: qr.CurrentVersion}}
		reader := &fakeTicketReader{info: scanner.DisplayInfo{TicketStatus: status}}
		repo := &fakeRepo{ticket: ticket}

		svc := scanner.NewService(qrv, reader, pickup, repo, nil, nil)

		res, err := svc.Verify(context.Background(), scanner.VerifyInput{Token: "token", EventID: eventID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// alreadyPickedUp is true exactly when an active PICKED_UP record exists.
		if res.AlreadyPickedUp != hasPickup {
			t.Fatalf("alreadyPickedUp = %v, want %v", res.AlreadyPickedUp, hasPickup)
		}
		if hasPickup {
			if res.PickedUpAt == nil {
				t.Fatalf("pickedUpAt must be set when a PICKED_UP record exists")
			}
			if !res.PickedUpAt.Equal(pickupAt) {
				t.Fatalf("pickedUpAt = %v, want original %v", res.PickedUpAt, pickupAt)
			}
		} else if res.PickedUpAt != nil {
			t.Fatalf("pickedUpAt must be nil when no PICKED_UP record exists, got %v", res.PickedUpAt)
		}

		// alreadyCheckedIn is true exactly when the ticket status is USED.
		wantCheckedIn := status == scanner.TicketStatusUsed
		if res.AlreadyCheckedIn != wantCheckedIn {
			t.Fatalf("alreadyCheckedIn = %v, want %v (status=%s)", res.AlreadyCheckedIn, wantCheckedIn, status)
		}
		if wantCheckedIn {
			if res.CheckedInAt == nil {
				t.Fatalf("checkedInAt must be set when status is USED")
			}
			if !res.CheckedInAt.Equal(usedAt) {
				t.Fatalf("checkedInAt = %v, want original used_at %v", res.CheckedInAt, usedAt)
			}
		} else if res.CheckedInAt != nil {
			t.Fatalf("checkedInAt must be nil when status is not USED, got %v", res.CheckedInAt)
		}
	})
}
