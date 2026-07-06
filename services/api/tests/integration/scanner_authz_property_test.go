//go:build integration

// Property-based integration test for the Scanner PWA's permitted-event
// authorization guarantee (design Property 4). This is fundamentally a
// SQL-authorization property: it depends on the exact join through
// organization_members -> member_roles -> role_permissions -> permissions that
// scanner.ListScannableEventsForUser / UserCanScanEvent perform, so it runs
// against a real Postgres rather than a stub.
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
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"pgregory.net/rapid"

	"github.com/varin/ivyticketing/services/api/internal/modules/scanner"
)

// scanProfile is the permission shape assigned to a staff member within one
// organization. It models the independent axes the property must cover: an org
// may grant the racepack scan permission, the check-in scan permission, both,
// neither, or the staff may not be a member of the org at all.
type scanProfile int

const (
	profileNone      scanProfile = iota // member, but role holds no scan permission
	profileRacepack                     // racepack.execute
	profileCheckin                      // checkin.execute
	profileBoth                         // racepack.execute + checkin.execute
	profileNonMember                    // staff is NOT a member of this org
)

// grantsScan reports whether this profile makes the org's events Permitted_Events.
func (p scanProfile) grantsScan() bool {
	return p == profileRacepack || p == profileCheckin || p == profileBoth
}

// permKeys returns the permission keys granted to the org role under this profile.
// profileNone grants an unrelated permission (ticket.view) so the test proves the
// query filters on the *scan* permissions, not merely on role membership.
func (p scanProfile) permKeys() []string {
	switch p {
	case profileRacepack:
		return []string{"racepack.execute"}
	case profileCheckin:
		return []string{"checkin.execute"}
	case profileBoth:
		return []string{"racepack.execute", "checkin.execute"}
	case profileNone:
		return []string{"ticket.view"}
	default: // profileNonMember: no role at all
		return nil
	}
}

// permissionID looks up a permission's UUID by its catalog key.
func permissionID(t *testing.T, pool *pgxpool.Pool, key string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM permissions WHERE key = $1`, key).Scan(&id); err != nil {
		t.Fatalf("lookup permission %q: %v", key, err)
	}
	return id
}

// seedScannerStaffUser inserts a fresh staff user and returns its ID. A fresh
// user per rapid iteration is what gives the property per-iteration isolation:
// ListScannableEventsForUser(staffID) can only ever surface events this staff
// was joined to in the current iteration, so generated assignments never bleed
// across iterations even though rows accumulate in the shared test DB.
func seedScannerStaffUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	id := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name)
		 VALUES ($1,$2,'x','Scanner Authz Staff')`,
		id, fmt.Sprintf("scanauthz-%s@test.com", id.String()[:8])); err != nil {
		t.Fatalf("seed staff user: %v", err)
	}
	return id
}

// seedScanOrg creates one organization, a role in it granting permKeys, an event
// per event slot, and (unless nonmember) makes staff a member with that role. It
// returns the IDs of the events it created so the caller can build the expected
// permitted-event set. Uses unique UUIDs throughout so nothing collides with
// prior iterations.
func seedScanOrg(t *testing.T, pool *pgxpool.Pool, staffID uuid.UUID, profile scanProfile, numEvents int, permIDs map[string]uuid.UUID) []uuid.UUID {
	t.Helper()
	ctx := context.Background()

	orgID := uuid.New()
	short := orgID.String()[:8]
	if _, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug) VALUES ($1,$2,$3)`,
		orgID, "Authz Org "+short, "authz-"+short); err != nil {
		t.Fatalf("seed org: %v", err)
	}

	// Events for this org.
	eventIDs := make([]uuid.UUID, 0, numEvents)
	for i := 0; i < numEvents; i++ {
		eventID := uuid.New()
		es := eventID.String()[:8]
		if _, err := pool.Exec(ctx,
			`INSERT INTO events (id, organization_id, name, slug, event_type, status)
			 VALUES ($1,$2,$3,$4,'marathon','published')`,
			eventID, orgID, "Authz Event "+es, "authz-ev-"+es); err != nil {
			t.Fatalf("seed event: %v", err)
		}
		eventIDs = append(eventIDs, eventID)
	}

	// A non-member org: staff has no membership/role here at all.
	if profile == profileNonMember {
		return eventIDs
	}

	// Create an org-scoped role and grant the profile's permissions.
	roleID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO roles (id, organization_id, name, slug)
		 VALUES ($1,$2,$3,$4)`,
		roleID, orgID, "Authz Role "+short, "authz-role-"+short); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	for _, key := range profile.permKeys() {
		if _, err := pool.Exec(ctx,
			`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2)`,
			roleID, permIDs[key]); err != nil {
			t.Fatalf("grant permission %q: %v", key, err)
		}
	}

	// Membership + role assignment for the staff user.
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

	return eventIDs
}

// --- Property 4 -----------------------------------------------------------

// Feature: scanner-pwa, Property 4: Operations are authorized only for permitted events
//
// For any set of events and staff permission assignments, the set of events
// returned to the staff (ListScannableEventsForUser / Service.ListPermittedEvents)
// SHALL be exactly those for which the staff holds racepack.execute or
// checkin.execute; AND for any scan operation whose target event is not permitted
// for the staff, the operation SHALL be rejected with an authorization error
// (AssertEventPermitted -> ErrUnauthorizedEvent).
//
// The property draws a random set of organizations, assigns each a permission
// profile (none / racepack / checkin / both / non-member), and gives each org a
// random number of events. The EXPECTED permitted-event set is computed directly
// from the generated assignments (events in orgs whose profile grants a scan
// permission). It then asserts:
//   - the returned event set equals the expected set EXACTLY (no extras via a
//     duplicated join row, no omissions), and
//   - AssertEventPermitted returns nil iff the event is expected, and
//     ErrUnauthorizedEvent otherwise.
//
// Validates: Requirements 1.2, 1.4, 1.5
func TestProperty_OperationsAuthorizedOnlyForPermittedEvents(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)

	repo := scanner.NewRepository(pool)
	// Only repo-backed authorization methods are exercised; the QR verifier,
	// ticket reader, racepack executor, audit recorder, and logger are unused
	// by ListPermittedEvents / AssertEventPermitted, so nil is safe here.
	svc := scanner.NewService(nil, nil, nil, repo, nil, nil)

	permIDs := map[string]uuid.UUID{
		"racepack.execute": permissionID(t, pool, "racepack.execute"),
		"checkin.execute":  permissionID(t, pool, "checkin.execute"),
		"ticket.view":      permissionID(t, pool, "ticket.view"),
	}

	profiles := []scanProfile{profileNone, profileRacepack, profileCheckin, profileBoth, profileNonMember}

	rapid.Check(t, func(rt *rapid.T) {
		// Fresh staff per iteration → clean isolation in the shared test DB.
		staffID := seedScannerStaffUser(t, pool)

		numOrgs := rapid.IntRange(1, 5).Draw(rt, "numOrgs")

		expected := map[uuid.UUID]bool{} // events that SHOULD be permitted
		allEvents := map[uuid.UUID]bool{} // every event created this iteration

		for o := 0; o < numOrgs; o++ {
			profile := rapid.SampledFrom(profiles).Draw(rt, fmt.Sprintf("profile-%d", o))
			numEvents := rapid.IntRange(1, 3).Draw(rt, fmt.Sprintf("numEvents-%d", o))
			eventIDs := seedScanOrg(t, pool, staffID, profile, numEvents, permIDs)
			for _, id := range eventIDs {
				allEvents[id] = true
				if profile.grantsScan() {
					expected[id] = true
				}
			}
		}

		// (1) The returned permitted-event set must equal the expected set EXACTLY.
		permitted, err := svc.ListPermittedEvents(context.Background(), staffID)
		if err != nil {
			rt.Fatalf("ListPermittedEvents: %v", err)
		}
		got := map[uuid.UUID]bool{}
		for _, e := range permitted {
			id, perr := uuid.Parse(e.EventID)
			if perr != nil {
				rt.Fatalf("returned event id %q not a uuid: %v", e.EventID, perr)
			}
			if got[id] {
				rt.Fatalf("event %s returned more than once (join fan-out leaked a duplicate)", id)
			}
			got[id] = true
		}
		if !sameSet(got, expected) {
			rt.Fatalf("permitted set mismatch:\n  got:      %s\n  expected: %s",
				sortedIDs(got), sortedIDs(expected))
		}

		// (2) Per-operation authorization: nil iff the event is permitted, else
		// ErrUnauthorizedEvent.
		for id := range allEvents {
			err := svc.AssertEventPermitted(context.Background(), staffID, id)
			if expected[id] {
				if err != nil {
					rt.Fatalf("AssertEventPermitted(%s) = %v, want nil (event is permitted)", id, err)
				}
			} else {
				if !errors.Is(err, scanner.ErrUnauthorizedEvent) {
					rt.Fatalf("AssertEventPermitted(%s) = %v, want ErrUnauthorizedEvent (event not permitted)", id, err)
				}
			}
		}
	})
}

// sameSet reports whether two uuid sets contain exactly the same keys.
func sameSet(a, b map[uuid.UUID]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// sortedIDs renders a uuid set as a stable, sorted string for failure messages.
func sortedIDs(m map[uuid.UUID]bool) string {
	ids := make([]string, 0, len(m))
	for k := range m {
		ids = append(ids, k.String())
	}
	sort.Strings(ids)
	return fmt.Sprintf("%v", ids)
}
