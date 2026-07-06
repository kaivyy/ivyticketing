//go:build integration

// Property-based integration tests for the racepack pickup guarantees that the
// Scanner PWA reuses verbatim (design D3: "Reuse racepack.ExecutePickup for
// pickup sync"). These exercise the SAME server-side path the scanner replays
// its offline pickups to (POST /racepack/pickups → Service.ExecutePickup),
// against a real Postgres so the SELECT ... FOR UPDATE window and the unique
// partial index uniq_racepack_pickup_records_ticket_active are actually in play.
//
// Run with: make test-db-setup && make test-integration
// (or: TEST_DATABASE_URL=... go test -tags=integration ./tests/integration/...)
//
// Uses pgregory.net/rapid; rapid's default is 100 checks per property.
package integration

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"pgregory.net/rapid"

	"github.com/varin/ivyticketing/services/api/internal/modules/racepack"
)

// --- scanner-pickup fixtures ---------------------------------------------

// scannerPickupFixture holds the shared, reusable rows (org/event/category and
// an active counter) that every pickup attempt in a property targets. Only the
// ticket + order + participant vary per rapid iteration, which keeps the DB
// churn bounded while still exercising a fresh ticket (and therefore a fresh
// unique-index slot) every time.
type scannerPickupFixture struct {
	pool       *pgxpool.Pool
	orgID      uuid.UUID
	eventID    uuid.UUID
	categoryID uuid.UUID
	counterID  uuid.UUID
	staffID    uuid.UUID
}

// newScannerPickupFixture seeds one org/event/category (via the existing
// concurrency seed helper), one active racepack counter, and a staff user
// (racepack_pickup_records.staff_id carries a FK to users).
func newScannerPickupFixture(t *testing.T, pool *pgxpool.Pool) scannerPickupFixture {
	t.Helper()
	orgID, eventID, categoryID := seedPublishedCategory(t, pool, 1000, 1000)

	counterID := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO racepack_counters (id, organization_id, event_id, name, active)
		 VALUES ($1,$2,$3,'Scanner Counter', true)`,
		counterID, orgID, eventID)
	if err != nil {
		t.Fatalf("seed counter: %v", err)
	}

	staffID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name)
		 VALUES ($1,$2,'x','Scanner Staff')`,
		staffID, fmt.Sprintf("scanstaff-%s@test.com", staffID.String()[:8])); err != nil {
		t.Fatalf("seed staff user: %v", err)
	}

	return scannerPickupFixture{
		pool:       pool,
		orgID:      orgID,
		eventID:    eventID,
		categoryID: categoryID,
		counterID:  counterID,
		staffID:    staffID,
	}
}

// seedTicket inserts a participant (user), an order with the given status, and
// a ticket with the given status, all wired to the shared fixture. Returns the
// ticket ID. When hasBib is false the ticket's bib_number is SQL NULL (no
// assigned BIB); when true it gets a per-ticket unique BIB (the shared event
// carries a partial unique index on (event_id, bib_number)).
func (f scannerPickupFixture) seedTicket(t testingTB, ticketStatus string, hasBib bool, orderStatus string) uuid.UUID {
	ctx := context.Background()
	participantID := uuid.New()
	short := participantID.String()[:8]
	bib := ""
	if hasBib {
		bib = "B-" + short // unique per participant → unique per event
	}

	if _, err := f.pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name)
		 VALUES ($1,$2,'x','Scanner Participant')`,
		participantID, fmt.Sprintf("scanpart-%s@test.com", short)); err != nil {
		t.Fatalf("seed participant: %v", err)
	}

	orderID := uuid.New()
	if _, err := f.pool.Exec(ctx,
		`INSERT INTO orders
		   (id, organization_id, event_id, category_id, participant_id, order_number, status, subtotal, fee, discount, total)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,100000,0,0,100000)`,
		orderID, f.orgID, f.eventID, f.categoryID, participantID,
		"ORD-"+short, orderStatus); err != nil {
		t.Fatalf("seed order: %v", err)
	}

	ticketID := uuid.New()
	var bibArg any
	if bib != "" {
		bibArg = bib
	} else {
		bibArg = nil
	}
	if _, err := f.pool.Exec(ctx,
		`INSERT INTO tickets
		   (id, organization_id, event_id, category_id, order_id, participant_id,
		    ticket_number, status, holder_name, holder_email, event_title, category_name, bib_number)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'Scanner Participant',$9,'Scanner Event','10K',$10)`,
		ticketID, f.orgID, f.eventID, f.categoryID, orderID, participantID,
		"TKT-"+short, ticketStatus, fmt.Sprintf("scanpart-%s@test.com", short), bibArg); err != nil {
		t.Fatalf("seed ticket: %v", err)
	}
	return ticketID
}

// countActivePickups returns the number of PICKED_UP records for a ticket.
func (f scannerPickupFixture) countActivePickups(t testingTB, ticketID uuid.UUID) int {
	var n int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM racepack_pickup_records WHERE ticket_id=$1 AND status='PICKED_UP'`,
		ticketID).Scan(&n); err != nil {
		t.Fatalf("count pickups: %v", err)
	}
	return n
}

// testingTB is the tiny subset of testing.TB used by the fixture helpers so
// they can be called with either *testing.T or *rapid.T.
type testingTB interface {
	Fatalf(format string, args ...any)
}

// --- Property 8 -----------------------------------------------------------

// Feature: scanner-pwa, Property 8: Pickup eligibility enforcement
//
// For any ticket that is CANCELLED, has no assigned BIB, or whose order is not
// PAID, confirming a racepack pickup (racepack.ExecutePickup — the same path the
// scanner reuses) SHALL be rejected with the corresponding error and SHALL NOT
// create a pickup record.
//
// The property draws the three independent ineligibility dimensions and forces
// at least one to hold, then asserts the returned error matches the service's
// documented guard precedence (cancelled → bib-missing → order-not-paid) and
// that no PICKED_UP record was written.
//
// Validates: Requirements 4.2, 4.3, 4.4
func TestProperty_PickupEligibilityEnforcement(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	fix := newScannerPickupFixture(t, pool)
	svc := racepack.NewService(racepack.NewRepository(pool), nil, nil)

	// A representative set of non-PAID order statuses (all must block pickup).
	nonPaidStatuses := []string{"DRAFT", "PENDING_PAYMENT", "EXPIRED", "CANCELLED", "REFUNDED"}

	rapid.Check(t, func(rt *rapid.T) {
		defCancel := rapid.Bool().Draw(rt, "defectCancelled")
		defBib := rapid.Bool().Draw(rt, "defectBibMissing")
		defOrder := rapid.Bool().Draw(rt, "defectOrderNotPaid")
		// Guarantee the ticket is ineligible on at least one dimension.
		if !defCancel && !defBib && !defOrder {
			defOrder = true
		}

		ticketStatus := racepack.TicketStatusValid
		if defCancel {
			ticketStatus = racepack.TicketStatusCancelled
		}
		hasBib := !defBib
		orderStatus := racepack.OrderStatusPaid
		if defOrder {
			orderStatus = rapid.SampledFrom(nonPaidStatuses).Draw(rt, "orderStatus")
		}

		// Expected error follows the service's guard order in ExecutePickup:
		// CANCELLED is checked first, then BIB presence, then order status.
		var wantErr error
		switch {
		case defCancel:
			wantErr = racepack.ErrTicketCancelled
		case defBib:
			wantErr = racepack.ErrBibMissing
		default:
			wantErr = racepack.ErrOrderNotPaid
		}

		ticketID := fix.seedTicket(rt, ticketStatus, hasBib, orderStatus)

		_, err := svc.ExecutePickup(context.Background(), racepack.ExecutePickupInput{
			OrgID:     fix.orgID,
			EventID:   fix.eventID,
			TicketID:  ticketID,
			CounterID: fix.counterID,
			StaffID:   fix.staffID,
			Method:    racepack.PickupMethodSelf,
		})

		if !errors.Is(err, wantErr) {
			rt.Fatalf("ticketStatus=%s hasBib=%t orderStatus=%s: got err %v, want %v",
				ticketStatus, hasBib, orderStatus, err, wantErr)
		}
		// The core invariant: an ineligible pickup writes NO record.
		if n := fix.countActivePickups(rt, ticketID); n != 0 {
			rt.Fatalf("ineligible pickup created %d record(s), want 0 (ticketStatus=%s hasBib=%t orderStatus=%s)",
				n, ticketStatus, hasBib, orderStatus)
		}
	})
}

// --- Property 9 -----------------------------------------------------------

// Feature: scanner-pwa, Property 9: Pickup creates exactly one record
//
// For an eligible ticket, any number of pickup confirmations (including
// concurrent and retried attempts) SHALL result in exactly one PICKED_UP record
// for that ticket. This drives many parallel goroutines (varied fan-out and
// per-goroutine retry counts) at the SAME real racepack.ExecutePickup path the
// scanner replays to, relying on SELECT ... FOR UPDATE + the unique partial
// index uniq_racepack_pickup_records_ticket_active as the no-duplicate guard —
// mirroring TestExecutePickup_ParallelRace but over real Postgres and randomised
// concurrency.
//
// Validates: Requirements 4.1, 6.3
func TestProperty_PickupCreatesExactlyOneRecord(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	fix := newScannerPickupFixture(t, pool)
	svc := racepack.NewService(racepack.NewRepository(pool), nil, nil)

	rapid.Check(t, func(rt *rapid.T) {
		// Randomise the concurrency shape each iteration.
		goroutines := rapid.IntRange(2, 10).Draw(rt, "goroutines")
		retriesPer := rapid.IntRange(1, 3).Draw(rt, "retriesPerGoroutine")

		// A fresh, fully eligible ticket per iteration (VALID, BIB assigned,
		// order PAID) → a fresh unique-index slot.
		ticketID := fix.seedTicket(rt, racepack.TicketStatusValid, true, racepack.OrderStatusPaid)

		var success, already, other int32
		var otherErr atomic.Value
		var wg sync.WaitGroup
		for g := 0; g < goroutines; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for r := 0; r < retriesPer; r++ {
					_, err := svc.ExecutePickup(context.Background(), racepack.ExecutePickupInput{
						OrgID:     fix.orgID,
						EventID:   fix.eventID,
						TicketID:  ticketID,
						CounterID: fix.counterID,
						StaffID:   fix.staffID,
						Method:    racepack.PickupMethodSelf,
					})
					switch {
					case err == nil:
						atomic.AddInt32(&success, 1)
					case errors.Is(err, racepack.ErrAlreadyPickedUp):
						atomic.AddInt32(&already, 1)
					default:
						atomic.AddInt32(&other, 1)
						otherErr.Store(err.Error())
					}
				}
			}()
		}
		wg.Wait()

		totalAttempts := int32(goroutines * retriesPer)

		// Exactly one attempt may succeed; the rest are rejected as duplicates;
		// no attempt fails for any other reason.
		if other != 0 {
			rt.Fatalf("goroutines=%d retries=%d: %d attempt(s) failed unexpectedly (want 0): sample err = %v",
				goroutines, retriesPer, other, otherErr.Load())
		}
		if success != 1 {
			rt.Fatalf("goroutines=%d retries=%d: %d successful pickups, want exactly 1",
				goroutines, retriesPer, success)
		}
		if already != totalAttempts-1 {
			rt.Fatalf("goroutines=%d retries=%d: %d already-picked-up, want %d",
				goroutines, retriesPer, already, totalAttempts-1)
		}

		// The database is the source of truth: exactly one PICKED_UP record.
		if n := fix.countActivePickups(rt, ticketID); n != 1 {
			rt.Fatalf("goroutines=%d retries=%d: DB has %d PICKED_UP records, want exactly 1",
				goroutines, retriesPer, n)
		}
	})
}
