# Phase 7 Plan — Part 1: Foundation (config, migrations, QR package)

> Part of the Phase 7 implementation plan. Index: [2026-06-08-phase7-participant-dashboard-ticket.md](2026-06-08-phase7-participant-dashboard-ticket.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **EXTEND, DON'T REWRITE.** New files + additive changes only. Do not alter Phase 1-6 behavior.

---

## Task 1: Config — TICKET_QR_SECRET (required)

**Files:**
- Modify: `services/api/internal/app/config.go`
- Modify: `services/api/internal/app/config_test.go`
- Modify: `services/api/.env.example`
- Modify: `.env.example` (root)

- [ ] **Step 1: Write the failing tests**

Add to `services/api/internal/app/config_test.go` (keep existing tests):
```go
func TestLoadConfig_TicketQRSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/ivyticketing?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("TICKET_QR_SECRET", "qr-secret")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TicketQRSecret != "qr-secret" {
		t.Errorf("TicketQRSecret = %q, want %q", cfg.TicketQRSecret, "qr-secret")
	}
}

func TestLoadConfig_MissingTicketQRSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/ivyticketing?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("TICKET_QR_SECRET", "")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error for missing TICKET_QR_SECRET, got nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
cd services/api && go test ./internal/app/ -run TestLoadConfig_TicketQRSecret -v; go test ./internal/app/ -run TestLoadConfig_MissingTicketQRSecret -v; cd ../..
```
Expected: FAIL — `cfg.TicketQRSecret` undefined.

- [ ] **Step 3: Add field + load + validate**

In `services/api/internal/app/config.go`, add to the `Config` struct (next to `JWTSecret string`):
```go
	TicketQRSecret string
```
In `LoadConfig`, inside the initial `cfg := Config{...}` literal, add after `JWTSecret: os.Getenv("JWT_SECRET"),`:
```go
		TicketQRSecret: os.Getenv("TICKET_QR_SECRET"),
```
Then immediately after the existing `if cfg.JWTSecret == "" { ... }` block, add:
```go
	if cfg.TicketQRSecret == "" {
		return Config{}, fmt.Errorf("config: TICKET_QR_SECRET is required")
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:
```bash
cd services/api && go test ./internal/app/ -run TestLoadConfig -v; cd ../..
```
Expected: PASS (all existing config tests still green — they already set the env they need; the two new tests set TICKET_QR_SECRET).

> NOTE: existing config tests that call `LoadConfig()` and expect success but do NOT set `TICKET_QR_SECRET` will now fail. Fix each by adding `t.Setenv("TICKET_QR_SECRET", "qr-secret")` alongside their existing `JWT_SECRET` setup. Run the full `./internal/app/` package and patch every newly-failing success-path test this way.

- [ ] **Step 5: Update .env.example files**

Append to `services/api/.env.example` and root `.env.example` (under the auth/secret section):
```
# Ticket QR signing (Phase 7) — HMAC secret, separate from JWT_SECRET
TICKET_QR_SECRET=
```

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/app/config.go services/api/internal/app/config_test.go services/api/.env.example .env.example
git commit -m "feat(phase7): add required TICKET_QR_SECRET config"
```

---

## Task 2: Migration — create tickets table

**Files:**
- Create: `database/migrations/00018_create_tickets.sql`

- [ ] **Step 1: Write the migration**

Create `database/migrations/00018_create_tickets.sql`:
```sql
-- +goose Up
CREATE TABLE tickets (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    category_id     uuid NOT NULL REFERENCES event_categories(id) ON DELETE RESTRICT,
    order_id        uuid NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    participant_id  uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    ticket_number   text NOT NULL UNIQUE,
    status          text NOT NULL DEFAULT 'VALID',
    holder_name     text NOT NULL,
    holder_email    text NOT NULL,
    event_title     text NOT NULL,
    category_name   text NOT NULL,
    qr_version      int NOT NULL DEFAULT 1,
    issued_at       timestamptz NOT NULL DEFAULT now(),
    used_at         timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tickets_status_check CHECK (status IN ('VALID','USED','CANCELLED')),
    CONSTRAINT tickets_order_unique UNIQUE (order_id)
);
CREATE INDEX idx_tickets_participant ON tickets(participant_id);
CREATE INDEX idx_tickets_event ON tickets(event_id);
CREATE INDEX idx_tickets_status ON tickets(status);

-- +goose Down
DROP TABLE tickets;
```

- [ ] **Step 2: Run migration up/down roundtrip**

Run (uses local DATABASE_URL; adjust if needed):
```bash
make migrate-up && make migrate-down && make migrate-up
```
Expected: up applies `00018`, down drops it cleanly, up re-applies — no errors.

- [ ] **Step 3: Commit**

```bash
git add database/migrations/00018_create_tickets.sql
git commit -m "feat(phase7): add tickets table migration"
```

---

## Task 3: Migration — seed ticket.view permission

**Files:**
- Create: `database/migrations/00019_seed_ticket_view.sql`

- [ ] **Step 1: Write the seed migration** (idempotent, mirrors `00017_seed_payment_manage.sql`)

Create `database/migrations/00019_seed_ticket_view.sql`:
```sql
-- +goose Up
INSERT INTO permissions (key, description)
VALUES ('ticket.view', 'View tickets in org/event')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.key = 'ticket.view'
WHERE r.organization_id IS NULL AND r.is_system = true AND r.slug IN ('owner','manager','customer-service')
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions
WHERE permission_id = (SELECT id FROM permissions WHERE key = 'ticket.view');
DELETE FROM permissions WHERE key = 'ticket.view';
```

- [ ] **Step 2: Run migration up/down roundtrip**

Run:
```bash
make migrate-up && make migrate-down && make migrate-up
```
Expected: clean up/down/up.

- [ ] **Step 3: Commit**

```bash
git add database/migrations/00019_seed_ticket_view.sql
git commit -m "feat(phase7): seed ticket.view permission"
```

---

## Task 4: sqlc queries for tickets + regenerate

**Files:**
- Create: `database/queries/tickets.sql`
- Regenerate: `services/api/internal/db/*` (via `make sqlc`)

- [ ] **Step 1: Write the queries**

Create `database/queries/tickets.sql`:
```sql
-- name: CreateTicket :one
INSERT INTO tickets (
    organization_id, event_id, category_id, order_id, participant_id,
    ticket_number, holder_name, holder_email, event_title, category_name, qr_version
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT (order_id) DO NOTHING
RETURNING *;

-- name: GetTicketByID :one
SELECT * FROM tickets WHERE id = $1;

-- name: GetTicketByOrderID :one
SELECT * FROM tickets WHERE order_id = $1;

-- name: ListTicketsByParticipant :many
SELECT * FROM tickets WHERE participant_id = $1 ORDER BY issued_at DESC;

-- name: ListTicketsByEvent :many
SELECT * FROM tickets
WHERE organization_id = $1 AND event_id = $2
ORDER BY issued_at DESC;
```

Note: `CreateTicket` uses `ON CONFLICT (order_id) DO NOTHING RETURNING *` — on a duplicate it returns **zero rows** (`pgx.ErrNoRows`); the issuer treats that as the idempotent no-op (Task 8).

- [ ] **Step 2: Regenerate sqlc**

Run:
```bash
make sqlc
```
Expected: `services/api/internal/db/` gains `tickets.sql.go` with `CreateTicket`, `GetTicketByID`, `GetTicketByOrderID`, `ListTicketsByParticipant`, `ListTicketsByEvent`, plus a `Ticket` struct. No errors.

- [ ] **Step 3: Verify generated types compile**

Run:
```bash
cd services/api && go build ./internal/db/...; cd ../..
```
Expected: builds clean.

- [ ] **Step 4: Commit**

```bash
git add database/queries/tickets.sql services/api/internal/db
git commit -m "feat(phase7): add tickets sqlc queries"
```

---

## Task 5: QR signed token package (tickets/qr)

**Files:**
- Create: `services/api/internal/modules/tickets/qr/qr.go`
- Create: `services/api/internal/modules/tickets/qr/qr_test.go`

- [ ] **Step 1: Write the failing tests**

Create `services/api/internal/modules/tickets/qr/qr_test.go`:
```go
package qr

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestSignVerify_Roundtrip(t *testing.T) {
	s := NewSigner("secret-key")
	tid := uuid.New()
	eid := uuid.New()

	tok, err := s.Sign(tid, eid)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	ref, err := s.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if ref.TicketID != tid || ref.EventID != eid {
		t.Fatalf("ref mismatch: got %+v", ref)
	}
	if ref.Version != 1 {
		t.Fatalf("version = %d, want 1", ref.Version)
	}
}

func TestVerify_WrongSecret(t *testing.T) {
	tok, _ := NewSigner("secret-a").Sign(uuid.New(), uuid.New())
	if _, err := NewSigner("secret-b").Verify(tok); err == nil {
		t.Fatal("expected error for wrong secret, got nil")
	}
}

func TestVerify_TamperedPayload(t *testing.T) {
	s := NewSigner("secret-key")
	tok, _ := s.Sign(uuid.New(), uuid.New())
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected token shape: %q", tok)
	}
	// flip a char in the payload segment
	payload := []byte(parts[1])
	payload[0] ^= 0x01
	tampered := parts[0] + "." + string(payload) + "." + parts[2]
	if _, err := s.Verify(tampered); err == nil {
		t.Fatal("expected error for tampered payload, got nil")
	}
}

func TestVerify_MalformedToken(t *testing.T) {
	s := NewSigner("secret-key")
	for _, bad := range []string{"", "a.b", "a.b.c.d", "not-a-token"} {
		if _, err := s.Verify(bad); err == nil {
			t.Fatalf("expected error for malformed token %q, got nil", bad)
		}
	}
}

func TestVerify_UnknownVersion(t *testing.T) {
	s := NewSigner("secret-key")
	tok, _ := s.Sign(uuid.New(), uuid.New())
	parts := strings.Split(tok, ".")
	// replace version segment with "9"
	bad := "9." + parts[1] + "." + parts[2]
	if _, err := s.Verify(bad); err == nil {
		t.Fatal("expected error for unknown version, got nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
cd services/api && go test ./internal/modules/tickets/qr/ -v; cd ../..
```
Expected: FAIL — package/`NewSigner` not defined.

- [ ] **Step 3: Implement the signer**

Create `services/api/internal/modules/tickets/qr/qr.go`:
```go
// Package qr signs and verifies ticket QR tokens using HMAC-SHA256.
// Payload carries only ticket_id, event_id, and a version — never PII.
package qr

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// CurrentVersion is the QR payload schema/secret version.
const CurrentVersion = 1

// TicketRef is the data carried in a QR token.
type TicketRef struct {
	TicketID uuid.UUID
	EventID  uuid.UUID
	Version  int
}

type payload struct {
	TID string `json:"tid"`
	EID string `json:"eid"`
	V   int    `json:"v"`
}

// Signer signs and verifies QR tokens with a single HMAC secret.
type Signer struct {
	secret []byte
}

func NewSigner(secret string) *Signer {
	return &Signer{secret: []byte(secret)}
}

var enc = base64.RawURLEncoding

func (s *Signer) mac(versionAndPayload string) string {
	m := hmac.New(sha256.New, s.secret)
	m.Write([]byte(versionAndPayload))
	return enc.EncodeToString(m.Sum(nil))
}

// Sign returns "<version>.<base64url(payload)>.<base64url(hmac)>".
func (s *Signer) Sign(ticketID, eventID uuid.UUID) (string, error) {
	body, err := json.Marshal(payload{TID: ticketID.String(), EID: eventID.String(), V: CurrentVersion})
	if err != nil {
		return "", err
	}
	verSeg := strconv.Itoa(CurrentVersion)
	payloadSeg := enc.EncodeToString(body)
	signingInput := verSeg + "." + payloadSeg
	return signingInput + "." + s.mac(signingInput), nil
}

// Verify checks the signature and returns the decoded TicketRef.
func (s *Signer) Verify(token string) (TicketRef, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return TicketRef{}, fmt.Errorf("qr: malformed token")
	}
	verSeg, payloadSeg, sigSeg := parts[0], parts[1], parts[2]

	ver, err := strconv.Atoi(verSeg)
	if err != nil || ver != CurrentVersion {
		return TicketRef{}, fmt.Errorf("qr: unsupported version")
	}

	expected := s.mac(verSeg + "." + payloadSeg)
	if !hmac.Equal([]byte(expected), []byte(sigSeg)) {
		return TicketRef{}, fmt.Errorf("qr: invalid signature")
	}

	raw, err := enc.DecodeString(payloadSeg)
	if err != nil {
		return TicketRef{}, fmt.Errorf("qr: bad payload encoding")
	}
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return TicketRef{}, fmt.Errorf("qr: bad payload json")
	}
	tid, err := uuid.Parse(p.TID)
	if err != nil {
		return TicketRef{}, fmt.Errorf("qr: bad ticket id")
	}
	eid, err := uuid.Parse(p.EID)
	if err != nil {
		return TicketRef{}, fmt.Errorf("qr: bad event id")
	}
	return TicketRef{TicketID: tid, EventID: eid, Version: p.V}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:
```bash
cd services/api && go test ./internal/modules/tickets/qr/ -v; cd ../..
```
Expected: PASS (all 5 tests).

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/tickets/qr
git commit -m "feat(phase7): add HMAC QR signer (no PII payload)"
```

---

Part 1 complete. Next: [Part 2 — Tickets module (issuer + service)](2026-06-08-phase7-part2-tickets-module.md).
