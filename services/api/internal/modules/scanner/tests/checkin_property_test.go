package scanner_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"pgregory.net/rapid"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/modules/scanner"
	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
)

// -----------------------------------------------------------------------------
// statefulRepo is an in-memory scanner.Repository that ACTUALLY MODELS the
// ticket VALID->USED transition semantics and the shared idempotency_keys
// table. Unlike the read-only fakeRepo used by the Verify properties, this one
// mutates state so the CheckIn state machine (Property 10) and the handler-level
// exactly-once idempotency guarantee (Property 11) can be exercised end to end
// without a database.
//
// Modelling choices that mirror the real SQL:
//   - LockTicketForUpdate returns the current row (or pgx.ErrNoRows).
//   - MarkTicketUsed applies the guarded transition: it only flips VALID->USED
//     (setting used_at = COALESCE(arg.UsedAt, now())) and returns pgx.ErrNoRows
//     when the row was not VALID, exactly like the guarded UPDATE ... WHERE
//     status='VALID'.
//   - ExecTx runs fn against the same shared state (single-threaded replays are
//     sufficient for the exactly-once property, which the idempotency table —
//     not row locking — is responsible for at the handler level).
//   - The idempotency map is keyed by (key, scope) and behaves like the real
//     table: GetIdempotencyKey returns pgx.ErrNoRows on a miss; the handler only
//     inserts after a lookup miss, so a single effect is stored and every replay
//     reads it back verbatim.
type statefulRepo struct {
	tickets map[uuid.UUID]db.Ticket
	idem    map[string]db.IdempotencyKey

	markTicketUsedCalls int // counts real VALID->USED transitions applied
	now                 time.Time
}

func newStatefulRepo(now time.Time) *statefulRepo {
	return &statefulRepo{
		tickets: make(map[uuid.UUID]db.Ticket),
		idem:    make(map[string]db.IdempotencyKey),
		now:     now,
	}
}

func (r *statefulRepo) putTicket(t db.Ticket) { r.tickets[t.ID] = t }

func (r *statefulRepo) ExecTx(ctx context.Context, fn func(scanner.Repository) error) error {
	// Single-threaded model: run fn against the same shared state. Errors are
	// only produced before any mutation in CheckIn, so no rollback is required.
	return fn(r)
}

func (r *statefulRepo) GetTicketByID(_ context.Context, id uuid.UUID) (db.Ticket, error) {
	t, ok := r.tickets[id]
	if !ok {
		return db.Ticket{}, pgx.ErrNoRows
	}
	return t, nil
}

func (r *statefulRepo) LockTicketForUpdate(_ context.Context, id uuid.UUID) (db.Ticket, error) {
	t, ok := r.tickets[id]
	if !ok {
		return db.Ticket{}, pgx.ErrNoRows
	}
	return t, nil
}

func (r *statefulRepo) MarkTicketUsed(_ context.Context, arg db.MarkTicketUsedParams) (db.Ticket, error) {
	t, ok := r.tickets[arg.ID]
	if !ok {
		return db.Ticket{}, pgx.ErrNoRows
	}
	// Guarded transition: WHERE status='VALID'. No row updated otherwise.
	if t.Status != scanner.TicketStatusValid {
		return db.Ticket{}, pgx.ErrNoRows
	}
	used := arg.UsedAt
	if !used.Valid {
		used = pgtype.Timestamptz{Time: r.now, Valid: true}
	}
	t.Status = scanner.TicketStatusUsed
	t.UsedAt = used
	r.tickets[arg.ID] = t
	r.markTicketUsedCalls++
	return t, nil
}

func (r *statefulRepo) GetEventOrganizationID(_ context.Context, _ uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}

func (r *statefulRepo) ListScannableEventsForUser(_ context.Context, _ uuid.UUID) ([]db.ListScannableEventsForUserRow, error) {
	return nil, nil
}

// UserCanScanEvent is permissive here: the check-in transition and idempotency
// properties exercise the state machine, not authorization (Property 4 covers
// authz separately), so the staff is always treated as permitted.
func (r *statefulRepo) UserCanScanEvent(_ context.Context, _ uuid.UUID, _ uuid.UUID) (bool, error) {
	return true, nil
}

func idemMapKey(key, scope string) string { return scope + "|" + key }

func (r *statefulRepo) GetIdempotencyKey(_ context.Context, arg db.GetIdempotencyKeyParams) (db.GetIdempotencyKeyRow, error) {
	row, ok := r.idem[idemMapKey(arg.Key, arg.Scope)]
	if !ok {
		return db.GetIdempotencyKeyRow{}, pgx.ErrNoRows
	}
	return db.GetIdempotencyKeyRow{
		Key:            row.Key,
		RequestHash:    row.RequestHash,
		ResponseStatus: row.ResponseStatus,
		ResponseBody:   row.ResponseBody,
		CreatedAt:      row.CreatedAt,
	}, nil
}

func (r *statefulRepo) InsertIdempotencyKey(_ context.Context, arg db.InsertIdempotencyKeyParams) (db.IdempotencyKey, error) {
	k := idemMapKey(arg.Key, arg.Scope)
	// Mimic the primary key: first writer wins, later inserts do not overwrite
	// the stored response (the handler only inserts after a lookup miss anyway).
	if existing, ok := r.idem[k]; ok {
		return existing, nil
	}
	rec := db.IdempotencyKey{
		Key:            arg.Key,
		Scope:          arg.Scope,
		RequestHash:    arg.RequestHash,
		ResponseStatus: arg.ResponseStatus,
		ResponseBody:   arg.ResponseBody,
		CreatedAt:      pgtype.Timestamptz{Time: r.now, Valid: true},
	}
	r.idem[k] = rec
	return rec, nil
}

// Feature: scanner-pwa, Property 10: Check-in transition and idempotence
//
// For any ticket, confirming a check-in SHALL transition it from VALID to USED
// at most once: a VALID ticket becomes USED with a check-in timestamp, a
// CANCELLED ticket is rejected with no transition, and an already-USED ticket
// returns a duplicate result with no further transition. A ticket whose event
// does not match the selected event is rejected with an event-mismatch error
// (and, matching the service's guard order, a CANCELLED ticket is rejected as
// cancelled even when the event also mismatches).
//
// Validates: Requirements 5.1, 5.2, 6.2, 6.3
func TestProperty_CheckInTransitionAndIdempotence(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		eventID := uuidGen().Draw(t, "eventID")
		ticketID := uuidGen().Draw(t, "ticketID")

		initialStatus := rapid.SampledFrom([]string{
			scanner.TicketStatusValid,
			scanner.TicketStatusUsed,
			scanner.TicketStatusCancelled,
		}).Draw(t, "initialStatus")

		// Choose the event the staff selected: matching or deliberately different.
		mismatch := rapid.Bool().Draw(t, "mismatch")
		selectedEvent := eventID
		if mismatch {
			other := uuidGen().Draw(t, "otherEvent")
			if other == eventID {
				other[0] ^= 0xFF
			}
			selectedEvent = other
		}

		// USED tickets carry an original used_at that MUST survive a duplicate
		// check-in unchanged.
		origUsedAt := timeGen("origUsedAt").Draw(t, "origUsedAtVal")
		var initUsedAt pgtype.Timestamptz
		if initialStatus == scanner.TicketStatusUsed {
			initUsedAt = pgtype.Timestamptz{Time: origUsedAt, Valid: true}
		}

		scannedAt := timeGen("scannedAt").Draw(t, "scannedAtVal")
		serverNow := timeGen("serverNow").Draw(t, "serverNowVal")

		repo := newStatefulRepo(serverNow)
		repo.putTicket(db.Ticket{
			ID:      ticketID,
			EventID: eventID,
			Status:  initialStatus,
			UsedAt:  initUsedAt,
		})

		// CheckIn only touches repo + audit; the other collaborators are unused.
		svc := scanner.NewService(nil, nil, nil, repo, nil, nil)

		in := scanner.CheckInInput{
			EventID:   selectedEvent,
			TicketID:  ticketID,
			ScannedAt: &scannedAt,
		}

		res, err := svc.CheckIn(context.Background(), in)

		switch {
		case initialStatus == scanner.TicketStatusCancelled:
			// Cancelled is rejected before the event check (service guard order),
			// and never transitions.
			if !errors.Is(err, scanner.ErrTicketCancelled) {
				t.Fatalf("cancelled ticket: expected ErrTicketCancelled, got %v", err)
			}
			if repo.tickets[ticketID].Status != scanner.TicketStatusCancelled {
				t.Fatalf("cancelled ticket transitioned to %q", repo.tickets[ticketID].Status)
			}
			if repo.markTicketUsedCalls != 0 {
				t.Fatalf("cancelled ticket must not transition (transitions=%d)", repo.markTicketUsedCalls)
			}

		case mismatch:
			// Non-cancelled ticket with a mismatched event → rejected, no change.
			if !errors.Is(err, scanner.ErrEventMismatch) {
				t.Fatalf("event mismatch: expected ErrEventMismatch, got %v", err)
			}
			if repo.tickets[ticketID].Status != initialStatus {
				t.Fatalf("event mismatch changed status %q -> %q", initialStatus, repo.tickets[ticketID].Status)
			}
			if repo.markTicketUsedCalls != 0 {
				t.Fatalf("event mismatch must not transition (transitions=%d)", repo.markTicketUsedCalls)
			}

		case initialStatus == scanner.TicketStatusUsed:
			// Already USED → duplicate result, no further transition, original
			// timestamp preserved.
			if err != nil {
				t.Fatalf("used ticket: unexpected error %v", err)
			}
			if !res.Duplicate {
				t.Fatalf("used ticket: expected Duplicate=true")
			}
			if res.Status != scanner.TicketStatusUsed {
				t.Fatalf("used ticket: status = %q, want USED", res.Status)
			}
			if !res.CheckedInAt.Equal(origUsedAt) {
				t.Fatalf("used ticket: checkedInAt = %v, want original %v", res.CheckedInAt, origUsedAt)
			}
			if repo.markTicketUsedCalls != 0 {
				t.Fatalf("used ticket must not transition again (transitions=%d)", repo.markTicketUsedCalls)
			}
			if got := repo.tickets[ticketID].UsedAt.Time; !got.Equal(origUsedAt) {
				t.Fatalf("used ticket: stored used_at changed to %v, want %v", got, origUsedAt)
			}

		default: // VALID
			// VALID → USED exactly once, with the provided scan timestamp.
			if err != nil {
				t.Fatalf("valid ticket: unexpected error %v", err)
			}
			if res.Duplicate {
				t.Fatalf("valid ticket: expected Duplicate=false on first check-in")
			}
			if res.Status != scanner.TicketStatusUsed {
				t.Fatalf("valid ticket: status = %q, want USED", res.Status)
			}
			if !res.CheckedInAt.Equal(scannedAt) {
				t.Fatalf("valid ticket: checkedInAt = %v, want scannedAt %v", res.CheckedInAt, scannedAt)
			}
			if repo.tickets[ticketID].Status != scanner.TicketStatusUsed {
				t.Fatalf("valid ticket did not transition to USED (status=%q)", repo.tickets[ticketID].Status)
			}
			if repo.markTicketUsedCalls != 1 {
				t.Fatalf("valid ticket must transition exactly once (transitions=%d)", repo.markTicketUsedCalls)
			}

			// Idempotence at the service level: a second CheckIn on the now-USED
			// ticket returns a duplicate and performs no further transition.
			res2, err2 := svc.CheckIn(context.Background(), in)
			if err2 != nil {
				t.Fatalf("valid ticket replay: unexpected error %v", err2)
			}
			if !res2.Duplicate {
				t.Fatalf("valid ticket replay: expected Duplicate=true")
			}
			if !res2.CheckedInAt.Equal(scannedAt) {
				t.Fatalf("valid ticket replay: checkedInAt changed to %v, want %v", res2.CheckedInAt, scannedAt)
			}
			if repo.markTicketUsedCalls != 1 {
				t.Fatalf("valid ticket replay must not transition again (transitions=%d)", repo.markTicketUsedCalls)
			}
		}
	})
}

// newCheckInRouter wires the real scanner.Handler behind a chi router that
// populates the {orgId}/{eventId} URL params and injects an authenticated
// staff identity, exactly like the mounted route does in server.go. This lets
// the idempotency property exercise the true handler lookup/store path over
// net/http rather than reaching into internals.
func newCheckInRouter(h *scanner.Handler, staffID uuid.UUID) http.Handler {
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

// Feature: scanner-pwa, Property 11: Server idempotency is exactly-once
//
// For any scan operation replayed any number of times with the same
// Idempotency-Key, the Scan_API SHALL apply the effect exactly once and return
// the identical original response for every subsequent replay. The test drives
// the real POST /scan/check-in handler over httptest against a VALID ticket,
// replays N times with one key, and asserts: exactly one VALID->USED transition
// is applied, and every replay returns byte-identical status + body.
//
// Validates: Requirements 8.3
func TestProperty_ServerIdempotencyExactlyOnce(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		orgID := uuidGen().Draw(t, "orgID")
		eventID := uuidGen().Draw(t, "eventID")
		ticketID := uuidGen().Draw(t, "ticketID")
		staffID := uuidGen().Draw(t, "staffID")
		idempKey := uuidGen().Draw(t, "idempKey").String()

		replays := rapid.IntRange(2, 6).Draw(t, "replays")
		scannedAt := timeGen("scannedAt").Draw(t, "scannedAtVal")
		serverNow := timeGen("serverNow").Draw(t, "serverNowVal")

		repo := newStatefulRepo(serverNow)
		repo.putTicket(db.Ticket{
			ID:      ticketID,
			EventID: eventID,
			Status:  scanner.TicketStatusValid,
		})

		svc := scanner.NewService(nil, nil, nil, repo, nil, nil)
		handler := scanner.NewHandler(svc)
		router := newCheckInRouter(handler, staffID)

		reqBody, err := json.Marshal(scanner.CheckInRequest{
			TicketID:  ticketID.String(),
			ScannedAt: &scannedAt,
		})
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}

		path := "/organizations/" + orgID.String() + "/events/" + eventID.String() + "/scan/check-in"

		var firstStatus int
		var firstBody []byte
		for i := 0; i < replays; i++ {
			req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", idempKey)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			status := rec.Code
			// Normalise HTTP framing whitespace: the fresh response is written
			// with json.Encoder (trailing "\n") while cached replays return the
			// stored json.Marshal bytes. Requirement 8.3 concerns idempotency
			// semantics (the response content), not byte-level framing, so we
			// compare the trimmed JSON payload.
			body := bytes.TrimSpace(rec.Body.Bytes())

			if status != http.StatusOK {
				t.Fatalf("replay %d: status = %d, want 200 (body=%s)", i, status, body)
			}
			if i == 0 {
				firstStatus = status
				firstBody = append([]byte(nil), body...)
				continue
			}
			// Every replay returns the identical original response.
			if status != firstStatus {
				t.Fatalf("replay %d: status = %d, want identical original %d", i, status, firstStatus)
			}
			if !bytes.Equal(body, firstBody) {
				t.Fatalf("replay %d: body = %s, want identical original %s", i, body, firstBody)
			}
		}

		// The effect was applied exactly once across all replays.
		if repo.markTicketUsedCalls != 1 {
			t.Fatalf("expected exactly one VALID->USED transition across %d replays, got %d",
				replays, repo.markTicketUsedCalls)
		}
		if repo.tickets[ticketID].Status != scanner.TicketStatusUsed {
			t.Fatalf("ticket status = %q, want USED", repo.tickets[ticketID].Status)
		}

		// The cached original response reflects the real (non-duplicate) effect.
		var firstResult scanner.CheckInResult
		if err := json.Unmarshal(firstBody, &firstResult); err != nil {
			t.Fatalf("unmarshal first response: %v", err)
		}
		if firstResult.Duplicate {
			t.Fatalf("original response should reflect the applied effect (Duplicate=false), got Duplicate=true")
		}
		if firstResult.Status != scanner.TicketStatusUsed {
			t.Fatalf("original response status = %q, want USED", firstResult.Status)
		}
	})
}
