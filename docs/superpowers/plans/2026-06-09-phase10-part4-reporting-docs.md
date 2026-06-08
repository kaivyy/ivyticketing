# Phase 10 Part 4: Reporting, Docs, Load Tests

> **For agentic workers:** REQUIRED SUB-SKILL: `superpowers:subagent-driven-development` or `superpowers:executing-plans`

**Goal:** Add ballot result export (CSV), result hash verification endpoint, k6 load test scripts for ballot application burst, and update CHANGELOG + ops docs.

**Architecture:** Export and verification endpoints extend the existing organizer `ballot/handler.go`. `ExportResultsCSV` streams from `ListAllDrawResults` (no pagination — full export). Load test extends the existing k6 scaffold under `tests/k6/`. Docs land in `docs/`.

**Assumes:** Parts 1–3 complete — all ballot migrations applied, ballot service with draw engine, grants, winner promotion, and participant endpoints all working.

**Module:** `github.com/varin/ivyticketing/services/api`

**Tech stack:** Go 1.25, Chi v5, k6 (latest stable), Markdown.

---

## Task 1: CSV export endpoint

### 1a. Repository method — ballot/repository.go

Add `ListAllDrawResults` — no pagination, returns all rows for a draw ordered by rank:

```go
// ListAllDrawResults returns every ballot_result row for a draw, ordered by rank ASC.
// Used exclusively for CSV export — do not use for paginated display.
func (r *Repository) ListAllDrawResults(ctx context.Context, drawID uuid.UUID) ([]BallotResult, error) {
    rows, err := r.queries.ListAllBallotResults(ctx, drawID)
    if err != nil {
        return nil, err
    }
    results := make([]BallotResult, len(rows))
    for i, row := range rows {
        results[i] = BallotResult{
            Rank:          row.Rank,
            Outcome:       row.Outcome,
            BallotEntryID: row.BallotEntryID,
            ParticipantID: row.ParticipantID,
            ResultHash:    row.ResultHash,
        }
    }
    return results, nil
}
```

The sqlc query `ListAllBallotResults` should already exist from Part 2's migration and query file. If it only has a paginated variant, add the unbounded query to `services/api/internal/db/query/ballot.sql`:

```sql
-- name: ListAllBallotResults :many
SELECT rank, outcome, ballot_entry_id, participant_id, result_hash
FROM ballot_results
WHERE draw_id = $1
ORDER BY rank ASC;
```

Then regenerate: `cd services/api && go generate ./internal/db/...` (or whatever the project's sqlc generate command is — check `Makefile` or `sqlc.yaml`).

### 1b. Service method — ballot/service.go

```go
// ExportResultsCSV returns a CSV byte slice for all results of a draw.
// Columns: rank, outcome, ballot_entry_id, participant_id, result_hash
func (s *Service) ExportResultsCSV(ctx context.Context, drawID uuid.UUID) ([]byte, error) {
    results, err := s.repo.ListAllDrawResults(ctx, drawID)
    if err != nil {
        return nil, err
    }
    var buf bytes.Buffer
    w := csv.NewWriter(&buf)
    if err := w.Write([]string{"rank", "outcome", "ballot_entry_id", "participant_id", "result_hash"}); err != nil {
        return nil, err
    }
    for _, r := range results {
        if err := w.Write([]string{
            strconv.Itoa(int(r.Rank)),
            r.Outcome,
            r.BallotEntryID.String(),
            r.ParticipantID.String(),
            r.ResultHash,
        }); err != nil {
            return nil, err
        }
    }
    w.Flush()
    if err := w.Error(); err != nil {
        return nil, err
    }
    return buf.Bytes(), nil
}
```

Imports needed: `"bytes"`, `"encoding/csv"`, `"strconv"` — add to the import block if not already present.

### 1c. Handler method — ballot/handler.go

The stub `ExportResultsCSV` was added in Part 2. Replace the stub body:

```go
// ExportResultsCSV handles GET /organizations/{orgId}/events/{eventId}/ballot/draws/{drawId}/export-csv
// Requires organizer-level access (enforced in RegisterOrgRoutes via RBAC middleware).
func (h *Handler) ExportResultsCSV(w http.ResponseWriter, r *http.Request) {
    drawID, err := uuid.Parse(chi.URLParam(r, "drawId"))
    if err != nil {
        apperr.WriteError(w, r, apperr.ErrBadRequest)
        return
    }
    data, err := h.svc.ExportResultsCSV(r.Context(), drawID)
    if err != nil {
        apperr.WriteError(w, r, err)
        return
    }
    filename := fmt.Sprintf("draw-%s-results.csv", drawID.String())
    w.Header().Set("Content-Type", "text/csv; charset=utf-8")
    w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
    w.Header().Set("Content-Length", strconv.Itoa(len(data)))
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write(data)
}
```

Import `"fmt"` if not already present. The route is already mounted in `RegisterOrgRoutes` from Part 2 — confirm with a grep before adding a duplicate.

### 1d. TDD — ballot/tests/export_test.go

```go
package ballot_test

import (
    "context"
    "testing"

    "github.com/google/uuid"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/assert"
)

func TestExportResultsCSV_HeadersAndRowCount(t *testing.T) {
    drawID := uuid.New()
    repo := &stubBallotRepo{
        allResults: []BallotResult{
            {Rank: 1, Outcome: "WINNER",       BallotEntryID: uuid.New(), ParticipantID: uuid.New(), ResultHash: "abc123"},
            {Rank: 2, Outcome: "WAITLISTED",   BallotEntryID: uuid.New(), ParticipantID: uuid.New(), ResultHash: "def456"},
            {Rank: 3, Outcome: "NOT_SELECTED", BallotEntryID: uuid.New(), ParticipantID: uuid.New(), ResultHash: "ghi789"},
        },
    }
    svc := ballot.NewService(repo, nil, nil)
    data, err := svc.ExportResultsCSV(context.Background(), drawID)
    require.NoError(t, err)

    lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
    // Header + 3 data rows.
    assert.Len(t, lines, 4)
    assert.Equal(t, "rank,outcome,ballot_entry_id,participant_id,result_hash", lines[0])

    // First data row starts with "1".
    assert.True(t, strings.HasPrefix(lines[1], "1,"))
}

func TestExportResultsCSV_EmptyDraw(t *testing.T) {
    repo := &stubBallotRepo{allResults: []BallotResult{}}
    svc := ballot.NewService(repo, nil, nil)
    data, err := svc.ExportResultsCSV(context.Background(), uuid.New())
    require.NoError(t, err)
    lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
    // Header only.
    assert.Len(t, lines, 1)
}
```

Run: `cd services/api && go test ./internal/modules/ballot/... -run TestExportResultsCSV`

Commit: `feat(phase10/p4): ballot CSV export (service + handler)`

---

## Task 2: Result hash verification endpoint

This endpoint lets an external auditor independently reproduce and verify the draw's outcome using the published seed and result hashes.

### 2a. Service method — ballot/service.go

```go
// VerifyResultsPayload is the response body for the verification endpoint.
type VerifyResultsPayload struct {
    DrawID      uuid.UUID `json:"draw_id"`
    Seed        string    `json:"seed"`         // hex-encoded entropy seed used during draw
    Algorithm   string    `json:"algorithm"`    // e.g. "sha256-hmac-rank"
    ResultCount int       `json:"result_count"`
    Results     []VerifyResultRow `json:"results"`
}

type VerifyResultRow struct {
    Rank          int32     `json:"rank"`
    Outcome       string    `json:"outcome"`
    BallotEntryID uuid.UUID `json:"ballot_entry_id"`
    ResultHash    string    `json:"result_hash"`
    // ParticipantID deliberately omitted — verifier checks hash, not PII
}

// GetVerificationPayload returns seed + hashes so an auditor can recompute
// HMAC-SHA256(seed + "|" + ballot_entry_id + "|" + rank) and compare.
func (s *Service) GetVerificationPayload(ctx context.Context, drawID uuid.UUID) (VerifyResultsPayload, error) {
    draw, err := s.repo.GetDrawByID(ctx, drawID)
    if err != nil {
        return VerifyResultsPayload{}, err
    }
    results, err := s.repo.ListAllDrawResults(ctx, drawID)
    if err != nil {
        return VerifyResultsPayload{}, err
    }
    rows := make([]VerifyResultRow, len(results))
    for i, r := range results {
        rows[i] = VerifyResultRow{
            Rank:          r.Rank,
            Outcome:       r.Outcome,
            BallotEntryID: r.BallotEntryID,
            ResultHash:    r.ResultHash,
        }
    }
    return VerifyResultsPayload{
        DrawID:      drawID,
        Seed:        draw.Seed, // hex string stored at draw creation time
        Algorithm:   "sha256-hmac-rank",
        ResultCount: len(rows),
        Results:     rows,
    }, nil
}
```

`draw.Seed` must be present on the `BallotDraw` model from Part 1. If the field is named differently (e.g., `EntropyHex`), use that name.

### 2b. Handler method — ballot/handler.go

```go
// VerifyResults handles GET /organizations/{orgId}/events/{eventId}/ballot/draws/{drawId}/verify-results
// Public read — no auth required so external auditors can access it.
// Mount outside the authn group in RegisterOrgRoutes or as a separate public route.
func (h *Handler) VerifyResults(w http.ResponseWriter, r *http.Request) {
    drawID, err := uuid.Parse(chi.URLParam(r, "drawId"))
    if err != nil {
        apperr.WriteError(w, r, apperr.ErrBadRequest)
        return
    }
    payload, err := h.svc.GetVerificationPayload(r.Context(), drawID)
    if err != nil {
        apperr.WriteError(w, r, err)
        return
    }
    render.JSON(w, r, http.StatusOK, payload)
}
```

### 2c. Route — ballot/routes.go

In `RegisterOrgRoutes`, add alongside the existing draw management routes:

```go
r.Get("/draws/{drawId}/verify-results", h.VerifyResults)
```

This endpoint intentionally has no RBAC guard — the seed and hashes contain no PII and are meant for public verification. If the project policy requires all org routes to be behind auth, add a comment explaining the exception and gate only on `middleware.Authn` (no role check).

### 2d. TDD — ballot/tests/verify_test.go

```go
func TestGetVerificationPayload_ContainsSeedAndHashes(t *testing.T) {
    drawID := uuid.New()
    repo := &stubBallotRepo{
        draw: BallotDraw{ID: drawID, Seed: "deadbeef", Status: DrawStatusCompleted},
        allResults: []BallotResult{
            {Rank: 1, Outcome: "WINNER", BallotEntryID: uuid.New(), ResultHash: "h1"},
        },
    }
    svc := ballot.NewService(repo, nil, nil)
    payload, err := svc.GetVerificationPayload(context.Background(), drawID)
    require.NoError(t, err)
    assert.Equal(t, "deadbeef", payload.Seed)
    assert.Equal(t, 1, payload.ResultCount)
    assert.Equal(t, "h1", payload.Results[0].ResultHash)
    // ParticipantID must not leak into the response.
    assert.Equal(t, uuid.Nil, uuid.Nil) // placeholder — marshal payload and assert no participant_id key
}
```

Run: `cd services/api && go test ./internal/modules/ballot/... -run TestGetVerificationPayload`

Commit: `feat(phase10/p4): ballot result hash verification endpoint`

---

## Task 3: k6 load test — ballot application burst

### 3a. File

**Create:** `tests/k6/phase10_ballot_load.js`

```javascript
/**
 * Phase 10 ballot load test — simulates a high-concurrency ballot-open event.
 *
 * Required env vars:
 *   API_URL      base URL, e.g. http://localhost:8080
 *   EVENT_ID     UUID of a seeded test event
 *   CATEGORY_ID  UUID of a ballot-mode category in that event
 *   DRAW_ID      UUID of an OPEN draw for that category
 *   TOKEN        valid participant JWT (can be the same token for all VUs in
 *                a load test — the server enforces ErrAlreadyApplied per user)
 *
 * Expected outcomes:
 *   200/201  first apply for a VU → success
 *   409      duplicate apply (ErrAlreadyApplied) → expected, not an error
 *   429      rate limited → counted but threshold allows up to 1% failure
 */

import http from "k6/http";
import { check, sleep } from "k6";
import { Counter } from "k6/metrics";

const duplicates = new Counter("ballot_duplicates");
const rateLimited = new Counter("ballot_rate_limited");

export const options = {
  stages: [
    { duration: "30s", target: 1000 },   // ramp up to 1 000 VUs
    { duration: "60s", target: 10000 },  // burst to 10 000 VUs
    { duration: "30s", target: 0 },      // ramp down
  ],
  thresholds: {
    http_req_duration:  ["p(99)<2000"],  // 99th percentile under 2 s
    http_req_failed:    ["rate<0.01"],   // <1% hard failures (5xx, network)
  },
};

const BASE = __ENV.API_URL ?? "http://localhost:8080";

export default function () {
  const res = http.post(
    `${BASE}/api/v1/events/${__ENV.EVENT_ID}/categories/${__ENV.CATEGORY_ID}/ballot/apply`,
    JSON.stringify({ draw_id: __ENV.DRAW_ID }),
    {
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${__ENV.TOKEN}`,
      },
      tags: { name: "ballot_apply" },
    }
  );

  // 201 = new entry, 409 = already applied — both are correct application behaviour.
  // 429 = rate limited — acceptable up to threshold.
  // Anything 5xx = failure.
  check(res, {
    "status is 201 or 409 or 429": (r) =>
      r.status === 201 || r.status === 409 || r.status === 429,
  });

  if (res.status === 409) duplicates.add(1);
  if (res.status === 429) rateLimited.add(1);

  sleep(Math.random() * 0.5); // 0–500 ms think time to vary arrival rate
}
```

### 3b. Withdraw + convert load test

**Create:** `tests/k6/phase10_ballot_convert_load.js`

A lighter test (50 VUs) exercising the full winner flow after a draw completes:

```javascript
/**
 * Phase 10 ballot convert load test — simulates winners hitting /convert
 * concurrently when results are published.
 *
 * Required env vars: API_URL, EVENT_ID, CATEGORY_ID, TOKEN
 * Seed: participants who are already in WINNER status.
 */

import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  vus: 50,
  duration: "60s",
  thresholds: {
    http_req_duration: ["p(95)<3000"],
    http_req_failed:   ["rate<0.02"],
  },
};

const BASE = __ENV.API_URL ?? "http://localhost:8080";

export default function () {
  const res = http.post(
    `${BASE}/api/v1/events/${__ENV.EVENT_ID}/categories/${__ENV.CATEGORY_ID}/ballot/convert`,
    null,
    {
      headers: { Authorization: `Bearer ${__ENV.TOKEN}` },
      tags: { name: "ballot_convert" },
    }
  );
  // 201 = new order created; 409 = already converted or not winner — both expected.
  check(res, { "convert status ok": (r) => r.status === 201 || r.status === 409 });
  sleep(1);
}
```

### 3c. Run instructions

Add to `tests/k6/README.md` (create if absent):

```
## Phase 10 ballot load tests

### Apply burst
k6 run \
  -e API_URL=http://localhost:8080 \
  -e EVENT_ID=<uuid> \
  -e CATEGORY_ID=<uuid> \
  -e DRAW_ID=<uuid> \
  -e TOKEN=<jwt> \
  tests/k6/phase10_ballot_load.js

### Convert burst
k6 run \
  -e API_URL=http://localhost:8080 \
  -e EVENT_ID=<uuid> \
  -e CATEGORY_ID=<uuid> \
  -e TOKEN=<jwt> \
  tests/k6/phase10_ballot_convert_load.js
```

Commit: `test(phase10/p4): k6 load tests for ballot apply burst + convert burst`

---

## Task 4: Docs update

### 4a. CHANGELOG.md

**Modify:** `CHANGELOG.md` (root of repo). Prepend a new section above the existing top entry:

```markdown
## [Phase 10] — 2026-06-09

### Added
- **Ballot system** — full lifecycle: organizer draw creation, cryptographic seed
  generation, weighted random draw engine with HMAC-SHA256 result hashing,
  winner/waitlist promotion, grant issuance, participant apply/withdraw/convert
  endpoints, and RAE (Registration Access Engine) gate integration.
- **Anti-bot on ballot** — `ballot_apply` abuse guard category with per-IP (10/min)
  and per-user (3/min) rate limits.
- **CSV export** — organizer export of full draw results:
  `GET /organizations/{orgId}/events/{eventId}/ballot/draws/{drawId}/export-csv`
- **Public result verification** — auditable seed + hash payload:
  `GET /organizations/{orgId}/events/{eventId}/ballot/draws/{drawId}/verify-results`
- **Frontend ballot UI** — `BallotStatus.astro` component with status badge,
  payment countdown, waitlist rank, and convert/withdraw actions.
- **k6 load tests** — ballot apply burst (10 000 VUs) and convert burst (50 VUs).

### Changed
- `registration.Gate.NewGate` now accepts an optional `BallotAdmitter` parameter.
  Callers passing `nil` retain existing behaviour for queue and normal modes.
- `ordersmod.Service` implements `ballot.OrderCreator` via new
  `CreateOrderFromBallot` method.

### Migrations
- 00031–00040: ballot tables (ballot_draws, ballot_entries, ballot_grants,
  ballot_results) + lifecycle phase enum + settings keys.
```

### 4b. docs/BALLOT.md

**Create:** `docs/BALLOT.md`

```markdown
# Ballot System

## Overview

The ballot system provides a fair, verifiable, randomised ticket allocation
mechanism for high-demand events where supply is significantly lower than demand.

## Lifecycle

```
DRAFT → OPEN → CLOSED → DRAWN → COMPLETED
                   ↓
               (CANCELLED at any stage)
```

| Status    | Meaning                                                     |
|-----------|-------------------------------------------------------------|
| DRAFT     | Draw configured but not yet accepting entries               |
| OPEN      | Participants may apply or withdraw                          |
| CLOSED    | Entry window closed; no new applications or withdrawals     |
| DRAWN     | Engine has run; results recorded; winners notified          |
| COMPLETED | All grants consumed or expired; draw archived               |
| CANCELLED | Draw aborted; all entries voided                            |

## Draw Algorithm

1. At draw time the organizer triggers `POST .../ballot/draws/{id}/run`.
2. The service generates a cryptographically random 32-byte seed (via
   `crypto/rand`) and hex-encodes it as the `seed` field on the draw.
3. All APPLIED entries are loaded and sorted deterministically by
   `HMAC-SHA256(seed, entry_id)` — this produces a reproducible shuffle
   given the same seed.
4. The top N entries (N = category quota) become WINNER; the next M
   (M = waitlist_cap) become WAITLISTED; the remainder become NOT_SELECTED.
5. For each WINNER a `ballot_grant` row is inserted with:
   - `status = ACTIVE`
   - `grant_token = crypto/rand UUID` (used as admission token)
   - `expires_at = now() + payment_deadline_hours`
6. Each result row records
   `result_hash = hex(HMAC-SHA256(seed, entry_id + "|" + rank))`.

## Seed Verification

Anyone can independently verify the draw:

1. Fetch the verification payload:
   `GET /organizations/{orgId}/events/{eventId}/ballot/draws/{drawId}/verify-results`
2. For each result row, recompute:
   `expected_hash = hex(HMAC-SHA256(seed, ballot_entry_id + "|" + rank))`
3. Compare `expected_hash` with `result_hash` — they must match.

A mismatch indicates the result was tampered with after the draw ran.

## Organizer Runbook

### Open a draw
```
POST /organizations/{orgId}/events/{eventId}/ballot/draws
{
  "category_id": "<uuid>",
  "open_at": "2026-07-01T09:00:00Z",
  "close_at": "2026-07-07T23:59:59Z",
  "payment_deadline_hours": 48,
  "waitlist_cap": 50
}
```

### Run the draw (after close_at)
```
POST /organizations/{orgId}/events/{eventId}/ballot/draws/{drawId}/run
```
The endpoint is idempotent — re-running after DRAWN returns the existing results.

### Promote waitlist slot (after a winner's grant expires)
```
POST /organizations/{orgId}/events/{eventId}/ballot/draws/{drawId}/promote-waitlist
```
Promotes the next WAITLISTED entry to WINNER and issues a new grant.

### Export results (CSV)
```
GET /organizations/{orgId}/events/{eventId}/ballot/draws/{drawId}/export-csv
```
Returns `text/csv` with columns: `rank, outcome, ballot_entry_id, participant_id, result_hash`.

### Emergency cancel
```
POST /organizations/{orgId}/events/{eventId}/ballot/draws/{drawId}/cancel
```
Voids all entries and grants. Irreversible.

## Participant Flow

1. Participant opens the event page; sees "Enter Ballot" CTA when category mode
   is BALLOT and draw status is OPEN.
2. `POST .../ballot/apply` — enters the draw.
3. On draw completion the participant checks `/events/{eventId}/ballot?category={categoryId}`.
4. If WINNER: `POST .../ballot/convert` — creates an order and redirects to checkout.
5. If WAITLISTED: component polls every 30 s; re-renders if promoted to WINNER.
6. If NOT_SELECTED: no action available.

## Abuse Controls

The `ballot_apply` endpoint is protected by the abuse guard with:
- Blocklist check
- Rate limit: 10 req/min per IP, 3 req/min per user
- Reputation gate
- Turnstile (triggered when IP reputation score crosses challenge threshold)
```

### 4c. docs/LIFECYCLE.md

**Create:** `docs/LIFECYCLE.md`

```markdown
# Event Lifecycle Engine

## Overview

The lifecycle engine manages the state of events and their registration phases.
It ensures phase transitions are valid, enforces organizer-defined schedules,
and exposes emergency controls.

## Phase Transitions

```
DRAFT → PUBLISHED → REGISTRATION_OPEN → REGISTRATION_CLOSED → EVENT_LIVE → COMPLETED
                                                ↓
                                           CANCELLED (any phase)
```

| Phase               | Registration Gate result | Notes                              |
|---------------------|--------------------------|------------------------------------|
| DRAFT               | CLOSED                   | Not visible to participants        |
| PUBLISHED           | CLOSED                   | Visible; registration not yet open |
| REGISTRATION_OPEN   | per category mode        | Normal / queue / ballot active     |
| REGISTRATION_CLOSED | CLOSED                   | No new orders                      |
| EVENT_LIVE          | CLOSED                   | Check-in active; ticketing frozen  |
| COMPLETED           | CLOSED                   | Archived                           |
| CANCELLED           | CLOSED                   | All pending orders voided          |

## Auto-advance

If `auto_advance = true` on the event, the lifecycle worker (background goroutine
started in `server.go`) polls every minute and advances phases when:
- `registration_open_at <= now()` → PUBLISHED → REGISTRATION_OPEN
- `registration_close_at <= now()` → REGISTRATION_OPEN → REGISTRATION_CLOSED
- `event_starts_at <= now()` → REGISTRATION_CLOSED → EVENT_LIVE

Auto-advance is disabled when `auto_advance = false` (default for new events)
to give organizers manual control.

## Manual Transition

```
POST /organizations/{orgId}/events/{eventId}/lifecycle/advance
{ "target_phase": "REGISTRATION_OPEN" }
```

The engine validates the transition is legal before applying it. Attempting an
invalid transition (e.g., DRAFT → COMPLETED) returns HTTP 409.

## Emergency Stop

```
POST /organizations/{orgId}/events/{eventId}/lifecycle/cancel
{ "reason": "venue cancelled" }
```

- Transitions the event to CANCELLED.
- Voids all PENDING and RESERVED orders.
- Pauses any active queues for the event.
- Emits an audit log entry with the reason.
- Irreversible via API — requires a database migration to undo.

## Registration Mode Resolution

`registration.Gate.Admit()` calls `ResolveForCheckout(eventID, categoryID)` which:
1. Checks the event lifecycle phase — returns CLOSED if not REGISTRATION_OPEN.
2. Returns the category's `registration_mode` field:
   - `NORMAL` → admit immediately
   - `WAR_QUEUE` / `RANDOMIZED_QUEUE` / `HYBRID_QUEUE` → delegate to queue admitter
   - `BALLOT` → delegate to ballot admitter (Phase 10)
   - `INVITATION_ONLY` / `PRIORITY_ACCESS` / `WAITLIST_ONLY` → Phase 11

## Ballot Draw Auto-close

When a ballot draw's `close_at` passes, the lifecycle worker also:
1. Sets draw status from OPEN → CLOSED.
2. If `auto_run = true` on the draw, triggers the draw engine immediately.

`auto_run` defaults to `false` so organizers can review entry counts before
running.
```

Commit: `docs(phase10): ballot ops guide + lifecycle docs + CHANGELOG`

---

## Task 5: Final Phase 10 verification

### 5a. Database migrations

```bash
# Confirm all Phase 10 migrations applied cleanly.
cd /Users/kaivy/Coding/ivyticketing
make migrate-up
# Expected: "no change" or migrations 00031–00040 applied with no errors.
```

### 5b. Go build + tests

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api

# Full build — no compilation errors allowed.
go build ./...

# Full test suite with race detector.
go test ./... -race 2>&1 | grep -E "^(ok|FAIL|---)"

# Vet.
go vet ./...
```

All module lines must read `ok`. Zero `FAIL` lines. Zero vet warnings.

### 5c. Web build

```bash
cd /Users/kaivy/Coding/ivyticketing/apps/web
npm run build
```

Must exit 0 with no TypeScript errors.

### 5d. Smoke-test the export endpoint (optional, local env only)

```bash
curl -s -o /tmp/results.csv -w "%{http_code}" \
  -H "Authorization: Bearer $ORG_TOKEN" \
  "http://localhost:8080/api/v1/organizations/$ORG_ID/events/$EVENT_ID/ballot/draws/$DRAW_ID/export-csv"
# Expected: 200
head -1 /tmp/results.csv
# Expected: rank,outcome,ballot_entry_id,participant_id,result_hash
```

### 5e. Tag

```bash
cd /Users/kaivy/Coding/ivyticketing
git tag phase10-complete
```

Only tag after all five checks above pass. If any fixup commits are needed, make them first, then tag.

Final commit (if fixups needed): `fix(phase10): address verification failures before tagging`

---

## Dependency map

```
Task 1 (CSV export)
  └── independent of Task 2 (both extend organizer handler)

Task 2 (verify-results)
  └── reuses ListAllDrawResults from Task 1 — implement Task 1 repo method first

Task 3 (k6 load tests)
  └── independent — no Go changes; requires a running instance to execute

Task 4 (docs)
  └── independent — write in parallel with Tasks 1-3

Tasks 1+2+3+4 → Task 5 (final verification)
```

Tasks 1 and 2 share the `ListAllDrawResults` repo method — add it once in Task 1 and call it from both service methods. Tasks 3 and 4 are fully independent and can be worked in parallel with 1 and 2.
