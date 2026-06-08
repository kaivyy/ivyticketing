# Phase 10 Part 3: Participant Flow + RAE Gate Integration

> **For agentic workers:** REQUIRED SUB-SKILL: `superpowers:subagent-driven-development` or `superpowers:executing-plans`

**Goal:** Add participant ballot endpoints (apply, my-entry, withdraw, convert), integrate `BallotAdmitter` into the Registration Access Engine, wire the abuse guard on ballot-apply, and ship the frontend ballot UI.

**Architecture:** Participant endpoints added to `ballot/handler.go` + `routes.go`. `BallotAdmitter` interface declared in `registration/gate.go`; `ballot.Service` implements it. `server.go` injects ballot service into `NewGate`. Frontend: new `BallotStatus.astro` component + apply CTA on event page.

**Assumes:** Parts 1 and 2 complete — migrations 00031–00040 applied, `ballot` package exists with `Service`, `Repository`, `Handler` (organizer routes only), draw engine, and grant/winner promotion logic.

**Module:** `github.com/varin/ivyticketing/services/api`

**Tech stack:** Go 1.25, Chi v5, Astro 4, TypeScript.

---

## Task 1: BallotAdmitter interface + gate.go injection

**Modify:** `services/api/internal/modules/registration/gate.go`

### 1a. Extend the Gate struct and constructor

Add the `BallotAdmitter` interface and wire it into `Gate`. The existing `QueueAdmitter` pattern is the direct model — same shape, new interface.

```go
// BallotAdmitter checks whether a participant holds a valid active grant
// for a given category. Declared here so registration depends on an
// interface, not the concrete ballot package.
type BallotAdmitter interface {
    CheckBallotAdmission(ctx context.Context, participantID, categoryID uuid.UUID, admissionToken string) error
}

// Gate implements orders.RegistrationGate.
type Gate struct {
    svc       *Service
    queue     QueueAdmitter  // may be nil until Part 3 (already present)
    ballot    BallotAdmitter // may be nil until Phase 10 Part 3
}

// NewGate now accepts an optional BallotAdmitter.
// Pass nil during tests that only exercise queue or normal modes.
func NewGate(svc *Service, queue QueueAdmitter, ballot BallotAdmitter) *Gate {
    return &Gate{svc: svc, queue: queue, ballot: ballot}
}
```

### 1b. Add ModeBallot case to Admit()

```go
func (g *Gate) Admit(ctx context.Context, participantID, eventID, categoryID uuid.UUID, admissionToken string) error {
    mode, err := g.svc.ResolveForCheckout(ctx, eventID, categoryID)
    if err != nil {
        return err
    }
    switch mode {
    case ModeNormal:
        return nil
    case ModeClosed:
        return ErrClosed
    case ModeWarQueue, ModeRandomizedQueue, ModeHybridQueue:
        if g.queue == nil {
            return ErrModeNotAvailable
        }
        return g.queue.CheckAdmission(ctx, participantID, eventID, admissionToken)
    case ModeBallot:
        if g.ballot == nil {
            return ErrModeNotAvailable
        }
        return g.ballot.CheckBallotAdmission(ctx, participantID, categoryID, admissionToken)
    default:
        // INVITATION_ONLY / PRIORITY_ACCESS / WAITLIST_ONLY — Phase 11
        return ErrModeNotAvailable
    }
}
```

`ModeBallot` must already be declared in `registration/service.go` as a `RegistrationMode` constant from Part 1. Confirm before compiling.

### 1c. Implement CheckBallotAdmission on ballot.Service

**Modify:** `services/api/internal/modules/ballot/service.go`

```go
// CheckBallotAdmission implements registration.BallotAdmitter.
// It verifies the participant holds a live ACTIVE grant for the category
// whose token matches admissionToken.
func (s *Service) CheckBallotAdmission(
    ctx context.Context,
    participantID, categoryID uuid.UUID,
    admissionToken string,
) error {
    grant, err := s.repo.GetActiveGrantForParticipant(ctx, participantID, categoryID)
    if err != nil {
        return ErrNotWinner
    }
    if grant.GrantToken != admissionToken {
        return ErrInvalidAdmissionToken
    }
    if grant.Status != GrantStatusActive {
        return ErrGrantNotActive
    }
    if grant.ExpiresAt.Before(time.Now()) {
        return ErrGrantExpired
    }
    // Confirm the underlying ballot entry is WINNER, not just the grant.
    entry, err := s.repo.GetBallotEntryByID(ctx, grant.BallotEntryID)
    if err != nil || entry.Status != EntryStatusWinner {
        return ErrNotWinner
    }
    return nil
}
```

Add the new sentinel errors to `ballot/errors.go`:

```go
var (
    ErrNotWinner             = apperr.New(http.StatusForbidden, "BALLOT_NOT_WINNER", "no active ballot grant for this category")
    ErrInvalidAdmissionToken = apperr.New(http.StatusForbidden, "BALLOT_INVALID_TOKEN", "admission token does not match")
    ErrGrantNotActive        = apperr.New(http.StatusConflict, "BALLOT_GRANT_NOT_ACTIVE", "grant is not in ACTIVE status")
    ErrGrantExpired          = apperr.New(http.StatusConflict, "BALLOT_GRANT_EXPIRED", "winner grant has expired")
)
```

`GetActiveGrantForParticipant` is a sqlc query returning the single active grant row for `(participant_id, category_id)`. Add it to `ballot/repository.go` wrapping the generated sqlc call from Part 2.

### 1d. TDD — registration/tests/gate_ballot_test.go

```go
package registration_test

import (
    "context"
    "errors"
    "testing"

    "github.com/google/uuid"
    "github.com/stretchr/testify/require"

    "github.com/varin/ivyticketing/services/api/internal/modules/registration"
)

// stubBallotAdmitter satisfies registration.BallotAdmitter in tests.
type stubBallotAdmitter struct{ err error }

func (s *stubBallotAdmitter) CheckBallotAdmission(_ context.Context, _, _ uuid.UUID, _ string) error {
    return s.err
}

// stubModeSvc satisfies the minimal interface Gate.svc needs for ResolveForCheckout.
// (Reuse or adapt whatever stub pattern Parts 1-2 established.)

func TestAdmit_BallotMode_NilAdmitter(t *testing.T) {
    // Gate constructed with nil ballot admitter.
    svc := newStubRegistrationSvc(registration.ModeBallot)
    gate := registration.NewGate(svc, nil, nil)
    err := gate.Admit(context.Background(), uuid.New(), uuid.New(), uuid.New(), "tok")
    require.ErrorIs(t, err, registration.ErrModeNotAvailable)
}

func TestAdmit_BallotMode_ValidGrant(t *testing.T) {
    svc := newStubRegistrationSvc(registration.ModeBallot)
    gate := registration.NewGate(svc, nil, &stubBallotAdmitter{err: nil})
    err := gate.Admit(context.Background(), uuid.New(), uuid.New(), uuid.New(), "tok")
    require.NoError(t, err)
}

func TestAdmit_BallotMode_InvalidToken(t *testing.T) {
    svc := newStubRegistrationSvc(registration.ModeBallot)
    sentinel := errors.New("bad token")
    gate := registration.NewGate(svc, nil, &stubBallotAdmitter{err: sentinel})
    err := gate.Admit(context.Background(), uuid.New(), uuid.New(), uuid.New(), "wrong")
    require.ErrorIs(t, err, sentinel)
}
```

Run: `cd services/api && go test ./internal/modules/registration/... -run TestAdmit_Ballot`

Commit: `feat(phase10/p3): BallotAdmitter interface + gate.go ModeBallot case`

---

## Task 2: Participant ballot endpoints

**Modify:** `services/api/internal/modules/ballot/handler.go`, `ballot/routes.go`, `ballot/service.go`

### 2a. Request/response DTOs — ballot/dto.go

Add to the existing DTO file (Part 2 has organizer DTOs already):

```go
// ApplyRequest is the body for POST .../ballot/apply.
type ApplyRequest struct {
    DrawID uuid.UUID `json:"draw_id"`
}

// BallotEntryResponse is returned to the participant for all entry reads.
type BallotEntryResponse struct {
    ID              uuid.UUID  `json:"id"`
    DrawID          uuid.UUID  `json:"draw_id"`
    Status          string     `json:"status"`           // APPLIED | WINNER | WAITLISTED | NOT_SELECTED | CONVERTED | WITHDRAWN
    WaitlistRank    *int       `json:"waitlist_rank,omitempty"`
    PaymentDeadline *time.Time `json:"payment_deadline,omitempty"` // set when status=WINNER
    ConvertedAt     *time.Time `json:"converted_at,omitempty"`
}

// ConvertRequest carries no extra fields — the entry ID comes from the path.
// Kept as a struct for future extensibility (e.g., payment method hint).
type ConvertRequest struct{}
```

### 2b. Handler methods — ballot/handler.go

```go
// Apply handles POST /events/{eventId}/categories/{categoryId}/ballot/apply
func (h *Handler) Apply(w http.ResponseWriter, r *http.Request) {
    participantID, ok := authedParticipantID(r)
    if !ok {
        apperr.WriteError(w, r, apperr.ErrUnauthenticated)
        return
    }
    eventID, err := uuid.Parse(chi.URLParam(r, "eventId"))
    if err != nil {
        apperr.WriteError(w, r, apperr.ErrBadRequest)
        return
    }
    categoryID, err := uuid.Parse(chi.URLParam(r, "categoryId"))
    if err != nil {
        apperr.WriteError(w, r, apperr.ErrBadRequest)
        return
    }
    var req ApplyRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        apperr.WriteError(w, r, apperr.ErrBadRequest)
        return
    }
    entry, err := h.svc.Apply(r.Context(), participantID, eventID, categoryID, req.DrawID)
    if err != nil {
        apperr.WriteError(w, r, err)
        return
    }
    render.JSON(w, r, http.StatusCreated, toBallotEntryResponse(entry))
}

// MyEntry handles GET /events/{eventId}/categories/{categoryId}/ballot/my-entry
func (h *Handler) MyEntry(w http.ResponseWriter, r *http.Request) {
    participantID, ok := authedParticipantID(r)
    if !ok {
        apperr.WriteError(w, r, apperr.ErrUnauthenticated)
        return
    }
    categoryID, err := uuid.Parse(chi.URLParam(r, "categoryId"))
    if err != nil {
        apperr.WriteError(w, r, apperr.ErrBadRequest)
        return
    }
    entry, err := h.svc.GetMyEntry(r.Context(), participantID, categoryID)
    if err != nil {
        apperr.WriteError(w, r, err)
        return
    }
    render.JSON(w, r, http.StatusOK, toBallotEntryResponse(entry))
}

// Withdraw handles DELETE /events/{eventId}/categories/{categoryId}/ballot/my-entry
func (h *Handler) Withdraw(w http.ResponseWriter, r *http.Request) {
    participantID, ok := authedParticipantID(r)
    if !ok {
        apperr.WriteError(w, r, apperr.ErrUnauthenticated)
        return
    }
    categoryID, err := uuid.Parse(chi.URLParam(r, "categoryId"))
    if err != nil {
        apperr.WriteError(w, r, apperr.ErrBadRequest)
        return
    }
    if err := h.svc.Withdraw(r.Context(), participantID, categoryID); err != nil {
        apperr.WriteError(w, r, err)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

// Convert handles POST /events/{eventId}/categories/{categoryId}/ballot/convert
func (h *Handler) Convert(w http.ResponseWriter, r *http.Request) {
    participantID, ok := authedParticipantID(r)
    if !ok {
        apperr.WriteError(w, r, apperr.ErrUnauthenticated)
        return
    }
    categoryID, err := uuid.Parse(chi.URLParam(r, "categoryId"))
    if err != nil {
        apperr.WriteError(w, r, apperr.ErrBadRequest)
        return
    }
    orderID, err := h.svc.ConvertWinner(r.Context(), participantID, categoryID)
    if err != nil {
        apperr.WriteError(w, r, err)
        return
    }
    render.JSON(w, r, http.StatusCreated, map[string]string{"order_id": orderID.String()})
}
```

`authedParticipantID` is a helper that calls `authctx.FromContext` and returns the user UUID — follow the same pattern as queue handler's participant extraction. `render.JSON` follows the pattern already used across the codebase (check orders or queue handler for exact import).

### 2c. Service methods — ballot/service.go

```go
// Apply enters a participant into an open ballot draw.
// Errors: ErrBallotClosed (draw not OPEN), ErrAlreadyApplied (duplicate entry).
func (s *Service) Apply(
    ctx context.Context,
    participantID, eventID, categoryID, drawID uuid.UUID,
) (BallotEntry, error) {
    draw, err := s.repo.GetDrawByID(ctx, drawID)
    if err != nil {
        return BallotEntry{}, err
    }
    if draw.Status != DrawStatusOpen {
        return BallotEntry{}, ErrBallotClosed
    }
    if draw.CategoryID != categoryID || draw.EventID != eventID {
        return BallotEntry{}, apperr.ErrBadRequest
    }
    existing, err := s.repo.GetEntryByParticipantAndDraw(ctx, participantID, drawID)
    if err == nil && existing.ID != uuid.Nil {
        return BallotEntry{}, ErrAlreadyApplied
    }
    entry, err := s.repo.InsertBallotEntry(ctx, InsertBallotEntryParams{
        ID:            uuid.New(),
        DrawID:        drawID,
        ParticipantID: participantID,
        Status:        EntryStatusApplied,
        AppliedAt:     time.Now(),
    })
    return entry, err
}

// GetMyEntry returns the participant's entry for a category (any draw status).
func (s *Service) GetMyEntry(ctx context.Context, participantID, categoryID uuid.UUID) (BallotEntry, error) {
    return s.repo.GetEntryByParticipantAndCategory(ctx, participantID, categoryID)
}

// Withdraw removes an APPLIED entry. Not allowed once draw is CLOSED or later.
func (s *Service) Withdraw(ctx context.Context, participantID, categoryID uuid.UUID) error {
    entry, err := s.repo.GetEntryByParticipantAndCategory(ctx, participantID, categoryID)
    if err != nil {
        return err
    }
    if entry.Status != EntryStatusApplied {
        return ErrBallotWithdrawNotAllowed
    }
    draw, err := s.repo.GetDrawByID(ctx, entry.DrawID)
    if err != nil {
        return err
    }
    if draw.Status != DrawStatusOpen {
        return ErrBallotWithdrawNotAllowed
    }
    return s.repo.UpdateBallotEntryStatus(ctx, entry.ID, EntryStatusWithdrawn, time.Now())
}

// ConvertWinner converts a WINNER entry into an order via the OrderCreator seam.
// It atomically marks the entry CONVERTED and consumes the grant.
func (s *Service) ConvertWinner(ctx context.Context, participantID, categoryID uuid.UUID) (uuid.UUID, error) {
    entry, err := s.repo.GetEntryByParticipantAndCategory(ctx, participantID, categoryID)
    if err != nil {
        return uuid.Nil, err
    }
    if entry.Status != EntryStatusWinner {
        return uuid.Nil, ErrNotWinner
    }
    grant, err := s.repo.GetActiveGrantForParticipant(ctx, participantID, categoryID)
    if err != nil {
        return uuid.Nil, ErrGrantNotActive
    }
    if grant.ExpiresAt.Before(time.Now()) {
        return uuid.Nil, ErrGrantExpired
    }
    orderID, err := s.orderCreator.CreateOrderFromBallot(
        ctx, participantID, entry.EventID, categoryID, grant.ID,
    )
    if err != nil {
        return uuid.Nil, err
    }
    if err := s.repo.UpdateBallotEntryStatus(ctx, entry.ID, EntryStatusConverted, time.Now()); err != nil {
        return uuid.Nil, err
    }
    if err := s.repo.ConsumeGrant(ctx, grant.ID); err != nil {
        return uuid.Nil, err
    }
    return orderID, nil
}
```

### 2d. OrderCreator interface — ballot/service.go

```go
// OrderCreator is implemented by ordersmod.Service. Declared here as an
// interface so the ballot package does not import orders directly.
type OrderCreator interface {
    CreateOrderFromBallot(
        ctx context.Context,
        participantID, eventID, categoryID uuid.UUID,
        grantID uuid.UUID,
    ) (uuid.UUID, error)
}
```

Wire into `Service` struct:

```go
type Service struct {
    repo         Repository       // interface wrapping sqlc queries
    orderCreator OrderCreator     // injected from server.go
    auditLog     audit.Logger
}

func NewService(repo Repository, orderCreator OrderCreator, auditLog audit.Logger) *Service {
    return &Service{repo: repo, orderCreator: orderCreator, auditLog: auditLog}
}
```

If Part 1/2 already defined `NewService` with different parameters, adjust the signature to add `orderCreator` while preserving existing params.

Implement `CreateOrderFromBallot` on `ordersmod.Service`:

```go
// CreateOrderFromBallot creates a reserved order for a ballot winner.
// It skips the normal registration gate check — the grant IS the admission.
func (s *Service) CreateOrderFromBallot(
    ctx context.Context,
    participantID, eventID, categoryID uuid.UUID,
    grantID uuid.UUID,
) (uuid.UUID, error) {
    // Reserve inventory slot (same as normal checkout, quantity=1).
    // Insert order row with source="BALLOT", grant_id=grantID.
    // Return new order UUID.
    // Reuse existing inventory reservation logic from phase 5.
}
```

The exact body follows `ordersmod`'s existing `CreateOrder` pattern — read `orders/service.go` before implementing to avoid diverging.

### 2e. New errors — ballot/errors.go

```go
var (
    ErrBallotClosed             = apperr.New(http.StatusConflict, "BALLOT_CLOSED", "ballot draw is not open")
    ErrAlreadyApplied           = apperr.New(http.StatusConflict, "BALLOT_ALREADY_APPLIED", "you have already entered this ballot")
    ErrBallotWithdrawNotAllowed = apperr.New(http.StatusConflict, "BALLOT_WITHDRAW_NOT_ALLOWED", "withdrawal is only allowed while draw is open and entry is APPLIED")
)
```

### 2f. Participant routes — ballot/routes.go

```go
// RegisterParticipantRoutes mounts participant-facing ballot endpoints.
// The abuseGuard for ballot_apply is passed in to keep the ballot package
// free of a direct abuse dependency — same pattern as queue's join guard.
func (h *Handler) RegisterParticipantRoutes(r chi.Router, applyGuard func(http.Handler) http.Handler) {
    r.Route("/events/{eventId}/categories/{categoryId}/ballot", func(r chi.Router) {
        r.With(applyGuard).Post("/apply", h.Apply)
        r.Get("/my-entry", h.MyEntry)
        r.Delete("/my-entry", h.Withdraw)
        r.Post("/convert", h.Convert)
    })
}
```

### 2g. Add CategoryBallotApply to abuse/model.go

```go
// Add to Endpoint categories block:
CategoryBallotApply = "ballot_apply"
```

Add to `categoryLimits`:

```go
case CategoryBallotApply:
    return RateLimit{PerIP: 10, PerUser: 3}
```

Limits rationale: a legitimate user applies once per draw. `PerUser: 3` gives a small retry budget while blocking burst automation. `PerIP: 10` allows shared-IP households.

### 2h. TDD — ballot/tests/participant_test.go

```go
func TestApply_DrawOpen_Success(t *testing.T) { ... }
func TestApply_DrawClosed_ErrBallotClosed(t *testing.T) { ... }   // draw.Status=CLOSED
func TestApply_DuplicateEntry_ErrAlreadyApplied(t *testing.T) { ... }
func TestWithdraw_EntryApplied_DrawOpen_Success(t *testing.T) { ... }
func TestWithdraw_DrawClosed_ErrWithdrawNotAllowed(t *testing.T) { ... }
func TestWithdraw_EntryWinner_ErrWithdrawNotAllowed(t *testing.T) { ... }
func TestConvertWinner_ValidGrant_ReturnsOrderID(t *testing.T) { ... }
func TestConvertWinner_GrantExpired_ErrGrantExpired(t *testing.T) { ... }
func TestConvertWinner_EntryNotWinner_ErrNotWinner(t *testing.T) { ... }
```

Use table-driven stubs for `Repository` and `OrderCreator`. Pattern: stub structs implementing the interfaces, same approach as queue tests in `queue/tests/`.

Run: `cd services/api && go test ./internal/modules/ballot/... -run TestApply -run TestWithdraw -run TestConvertWinner`

Commit: `feat(phase10/p3): participant ballot endpoints (apply, my-entry, withdraw, convert)`

---

## Task 3: Wire participant routes + ballot service injection in server.go

**Modify:** `services/api/internal/app/server.go`

### 3a. Construct ballot service and handler

Locate the existing ballot construction block from Part 2. Extend it to pass `ordersSvc` as the `OrderCreator`:

```go
// Ballot (Phase 10) — extend Part 2 construction.
ballotRepo   := ballotmod.NewRepository(pool)
ballotSvc    := ballotmod.NewService(ballotRepo, ordersSvc, auditLog)
ballotHandler := ballotmod.NewHandler(ballotSvc)
```

`ordersSvc` is already declared above when building `ordersHandler`. If the variable is named differently, use that name.

### 3b. Inject ballot service into NewGate

```go
// Update from Part 1/2 signature:
registrationGate := registrationmod.NewGate(registrationSvc, queueSvc, ballotSvc)
```

`ballotSvc` satisfies `registration.BallotAdmitter` because `ballot.Service` now implements `CheckBallotAdmission`.

### 3c. Mount participant routes inside the authn group

Inside the `r.Group(func(r chi.Router) { r.Use(middleware.Authn(signer)) ... })` block, alongside the existing `queueHandler.RegisterRoutes` call:

```go
ballotHandler.RegisterParticipantRoutes(r, abuseGuard.Middleware(abusemod.CategoryBallotApply))
```

### 3d. Mount organizer routes inside the per-org sub-resource block

Inside the `r.Route("/organizations/{orgId}", ...)` → `eventHandler.RegisterRoutes(r, loader, func(r chi.Router) { ... })` closure, alongside `queueHandler.RegisterOrgRoutes`:

```go
ballotHandler.RegisterOrgRoutes(r, loader)
```

(Organizer routes were added in Part 2 — this just confirms the mount point.)

### 3e. Add import

```go
ballotmod "github.com/varin/ivyticketing/services/api/internal/modules/ballot"
```

### 3f. Build check

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go build ./...
```

Fix any compilation errors before committing.

Commit: `feat(phase10/p3): wire ballot service into server.go + registration gate`

---

## Task 4: Frontend — ballot apply CTA + status component

### 4a. ballot.ts — services/api client library

**Create:** `apps/web/src/lib/ballot.ts`

```typescript
import { authedFetch } from "./api";

export interface BallotEntry {
  id: string;
  draw_id: string;
  status: "APPLIED" | "WINNER" | "WAITLISTED" | "NOT_SELECTED" | "CONVERTED" | "WITHDRAWN";
  waitlist_rank?: number;
  payment_deadline?: string; // ISO-8601
  converted_at?: string;
}

export function applyBallot(eventId: string, categoryId: string, drawId: string): Promise<BallotEntry> {
  return authedFetch<BallotEntry>(`/events/${eventId}/categories/${categoryId}/ballot/apply`, {
    method: "POST",
    body: { draw_id: drawId },
  });
}

export function getMyBallotEntry(eventId: string, categoryId: string): Promise<BallotEntry | null> {
  return authedFetch<BallotEntry>(`/events/${eventId}/categories/${categoryId}/ballot/my-entry`)
    .catch((err: Error) => {
      // 404 means no entry — return null instead of throwing.
      if (err.message.startsWith("HTTP 404") || err.message === "HTTP 404") return null;
      throw err;
    });
}

export function withdrawBallot(eventId: string, categoryId: string): Promise<void> {
  return authedFetch<void>(`/events/${eventId}/categories/${categoryId}/ballot/my-entry`, {
    method: "DELETE",
  });
}

export function convertBallotWinner(eventId: string, categoryId: string): Promise<{ order_id: string }> {
  return authedFetch<{ order_id: string }>(`/events/${eventId}/categories/${categoryId}/ballot/convert`, {
    method: "POST",
  });
}
```

All calls go through `authedFetch` which handles 401 refresh and error normalisation — same pattern as `queue.ts` and `tickets.ts`.

### 4b. BallotStatus.astro component

**Create:** `apps/web/src/components/ballot/BallotStatus.astro`

```astro
---
export const prerender = false;
const { eventId, categoryId } = Astro.props;
---
<div
  id="ballot-status"
  data-event-id={eventId}
  data-category-id={categoryId}
  class="rounded-lg border border-slate-200 bg-white p-6"
>
  <p class="text-slate-500 text-center">Memuat status ballot…</p>
</div>

<script>
  import { getMyBallotEntry, withdrawBallot, convertBallotWinner } from "../../lib/ballot";

  const root = document.getElementById("ballot-status")!;
  const eventId = root.dataset.eventId!;
  const categoryId = root.dataset.categoryId!;

  function deadlineCountdown(isoDeadline: string): string {
    const diff = Math.max(0, new Date(isoDeadline).getTime() - Date.now());
    const h = Math.floor(diff / 3_600_000);
    const m = Math.floor((diff % 3_600_000) / 60_000);
    return diff === 0 ? "Waktu habis" : `${h}j ${m}m tersisa`;
  }

  function render(entry: Awaited<ReturnType<typeof getMyBallotEntry>>) {
    if (!entry) {
      root.innerHTML = `<p class="text-slate-500 text-center text-sm">Kamu belum mendaftar ballot ini.</p>`;
      return;
    }

    const statusLabel: Record<string, string> = {
      APPLIED:      "Terdaftar — menunggu pengundian",
      WINNER:       "Menang!",
      WAITLISTED:   "Waitlist",
      NOT_SELECTED: "Tidak terpilih",
      CONVERTED:    "Pesanan dibuat",
      WITHDRAWN:    "Ditarik",
    };

    const badge = (status: string) =>
      `<span class="inline-block rounded-full px-3 py-1 text-xs font-medium
        ${status === "WINNER" ? "bg-green-100 text-green-700" :
          status === "WAITLISTED" ? "bg-amber-100 text-amber-700" :
          status === "NOT_SELECTED" ? "bg-red-100 text-red-600" :
          "bg-slate-100 text-slate-600"}">${statusLabel[status] ?? status}</span>`;

    let extra = "";
    if (entry.status === "WINNER" && entry.payment_deadline) {
      const countdown = deadlineCountdown(entry.payment_deadline);
      extra = `
        <p class="text-sm text-amber-700 mt-2">Batas waktu pembayaran: ${countdown}</p>
        <button id="btn-convert"
          class="mt-3 rounded bg-slate-900 px-4 py-2 text-white text-sm hover:bg-slate-700">
          Bayar Sekarang →
        </button>`;
    }
    if (entry.status === "WAITLISTED" && entry.waitlist_rank != null) {
      extra = `<p class="text-sm text-slate-500 mt-2">Posisi waitlist: #${entry.waitlist_rank}</p>`;
    }
    if (entry.status === "APPLIED") {
      extra = `
        <button id="btn-withdraw"
          class="mt-3 text-xs text-red-500 underline">
          Tarik pendaftaran
        </button>`;
    }

    root.innerHTML = `
      <div class="text-center">
        ${badge(entry.status)}
        ${extra}
      </div>`;

    document.getElementById("btn-convert")?.addEventListener("click", async () => {
      try {
        const { order_id } = await convertBallotWinner(eventId, categoryId);
        window.location.href = `/orders/${order_id}`;
      } catch (e: unknown) {
        alert((e as Error).message);
      }
    });

    document.getElementById("btn-withdraw")?.addEventListener("click", async () => {
      if (!confirm("Tarik pendaftaran ballot ini?")) return;
      try {
        await withdrawBallot(eventId, categoryId);
        await refresh();
      } catch (e: unknown) {
        alert((e as Error).message);
      }
    });
  }

  async function refresh() {
    try {
      render(await getMyBallotEntry(eventId, categoryId));
    } catch {
      root.innerHTML = `<p class="text-xs text-red-500 text-center">Gagal memuat status ballot.</p>`;
    }
  }

  refresh();
  // Poll every 30 s while draw is running (winner promotion happens server-side).
  setInterval(refresh, 30_000);
  document.addEventListener("visibilitychange", () => { if (!document.hidden) refresh(); });
</script>
```

Pattern mirrors `WaitingRoom.astro`: inline `<script>`, `data-*` props passed from Astro frontmatter, visibility-change re-poll, no framework dependency.

### 4c. Ballot page

**Create:** `apps/web/src/pages/events/[eventId]/ballot.astro`

```astro
---
export const prerender = false;
import ParticipantLayout from "../../../layouts/ParticipantLayout.astro";
import BallotStatus from "../../../components/ballot/BallotStatus.astro";
const { eventId } = Astro.params;
const categoryId = Astro.url.searchParams.get("category") ?? "";
---
<ParticipantLayout title="Status Ballot">
  <div class="max-w-md mx-auto mt-12 px-4">
    <h1 class="text-xl font-bold mb-6 text-center">Status Ballot</h1>
    {eventId && categoryId
      ? <BallotStatus eventId={eventId} categoryId={categoryId} />
      : <p class="text-center text-red-600">Parameter tidak lengkap.</p>}
  </div>
</ParticipantLayout>
```

Mirrors `queue.astro` layout exactly. `categoryId` comes from `?category=<uuid>` query param because the route only has `eventId` as a path segment.

### 4d. Build check

```bash
cd /Users/kaivy/Coding/ivyticketing/apps/web && npm run build
```

Fix any TypeScript or import errors before committing.

Commit: `feat(phase10/p3): frontend ballot.ts + BallotStatus.astro + ballot page`

---

## Task 5: Part 3 full verification

```bash
# Go tests (race detector on)
cd /Users/kaivy/Coding/ivyticketing/services/api
go test ./... -race 2>&1 | grep -E "^(ok|FAIL|---)"

# Vet
go vet ./...

# Web build
cd /Users/kaivy/Coding/ivyticketing/apps/web
npm run build
```

All lines must read `ok` (no `FAIL`). Web build must exit 0.

If any test fails: fix in the same task before tagging complete. Do not skip failures with `-run` flags to hide them.

Final commit: `feat(phase10): part 3 — participant ballot flow + RAE gate integration`

---

## Dependency map

```
Task 1 (gate.go BallotAdmitter)
  └── Task 3 (server.go wiring — needs NewGate signature)
Task 2 (service + handler + routes)
  └── Task 3 (server.go wiring — needs RegisterParticipantRoutes + ballotSvc)
Tasks 1+2+3 → Task 4 (frontend calls live API)
Tasks 1-4 → Task 5 (full verification)
```

Tasks 1 and 2 can be worked in parallel. Task 3 requires both. Task 4 is independent of 1-3 (mocks the API) but best done after Task 3 so the build environment is consistent.
