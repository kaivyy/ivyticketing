# Phase 6: Payment Gateway V1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Peserta dengan order `PENDING_PAYMENT` bisa membayar via Duitku/Xendit (QRIS/VA/e-wallet); callback gateway (diterima binary webhook terpisah) memvalidasi signature, dideduplikasi idempotent, lalu mengubah order → `PAID` dan reservasi → `COMPLETED` secara atomik. Plus reconcile manual.

**Architecture:** Modul baru `payments` di `services/api/internal/modules/payments` dengan interface `Gateway` (adapter Duitku & Xendit), `Processor.ProcessCallback` sebagai inti idempotent yang dibagi antara API & webhook. Binary webhook tipis `services/api/cmd/webhook` (port terpisah) meng-import processor yang sama (store-then-process). Transisi order/reservasi memakai `orders`/`inventory` Phase 5 (`ExecTx`, `inv.Release`).

**Tech Stack:** Go 1.25, Chi v5, pgx v5, sqlc, goose, crypto (MD5/SHA256/HMAC untuk signature), Postgres.

**Reference spec:** `docs/superpowers/specs/2026-06-07-phase6-payment-gateway-design.md`

**Dependency note:** Phase 6 dibangun DI ATAS Phase 5 (orders/inventory/checkout). Phase 5 ada di worktree `phase5-orders-inventory-checkout` dan HARUS sudah ter-merge ke baseline yang dipakai sebelum Task 2+ dijalankan. Patterns yang dipakai (verified): `orders.Repository.ExecTx`, `inventory.Release(ctx, repo, orderID, status)`, `StatusPendingPayment`/`StatusPaid`, reservation status `"COMPLETED"`, `audit.Logger.Record(ctx, audit.Entry)`, error envelope `apperr.New/WriteError/WriteJSON`.

**Extend-only:** Dilarang mengubah behavior/API Phase 1-5. Semua kerja = file/migrasi/binary BARU + wiring tambahan.

---

## File Structure

```txt
services/api/
├── cmd/webhook/main.go                              # BINARY BARU: receiver callback (port 8090)
├── internal/
│   ├── app/config.go                                # MODIFY: env gateway + webhook
│   ├── app/server.go                                # MODIFY: wire payments handler
│   └── modules/payments/
│       ├── gateway/
│       │   ├── gateway.go                            # interface + tipe (Input/Result/CallbackResult/Status)
│       │   ├── registry.go                           # build map[string]Gateway dari config
│       │   ├── duitku/duitku.go                      # adapter Duitku
│       │   └── xendit/xendit.go                      # adapter Xendit
│       ├── errors.go                                 # typed errors → error codes
│       ├── model.go                                  # PaymentStatus enum + normalisasi + konstanta
│       ├── dto.go                                    # request/response
│       ├── merchantref.go                            # generator PAY-YYYYMMDD-XXXXXX
│       ├── repository.go                             # sqlc repo + ExecTx (payments + webhooks + order/reservation)
│       ├── service.go                                # create charge, get/list, ownership/eligibility
│       ├── processor.go                              # ProcessCallback (INTI idempotent) — dibagi api & webhook
│       ├── reconcile.go                              # reconcile manual via gateway.QueryStatus + processor
│       ├── handler.go                                # HTTP handlers (peserta + organizer)
│       ├── routes.go                                 # route registration
│       ├── webhook/http/
│       │   ├── server.go                             # chi router webhook + middleware
│       │   ├── duitku_handler.go                     # POST /webhooks/duitku
│       │   ├── xendit_handler.go                     # POST /webhooks/xendit
│       │   └── server_test.go
│       └── tests/ (file _test.go di package payments)
database/
├── migrations/
│   ├── 000XX_create_payments.sql
│   ├── 000XX_create_payment_webhooks.sql
│   └── 000XX_seed_payment_manage.sql
└── queries/
    ├── payments.sql
    └── payment_webhooks.sql
docs/
├── PAYMENT_FLOW.md  WEBHOOK_PROCESSING.md  GATEWAY_INTEGRATION.md
├── PAYMENT_RECONCILIATION.md  PHASE6_DECISIONS.md
└── payment/DUITKU.md  payment/XENDIT.md  payment/CALLBACK_SECURITY.md
```

Nomor migrasi (`000XX`) = lanjutan dari migrasi terakhir yang sudah ter-commit saat implementasi (Phase 5 berhenti di ~00014/00015). Cek `ls database/migrations` dulu, pakai nomor berurutan berikutnya.

---

## Task 1: Config — env gateway & webhook

**Files:**
- Modify: `services/api/internal/app/config.go`
- Test: `services/api/internal/app/config_test.go` (tambah kasus)
- Modify: `.env.example`, `services/api/.env.example`

- [ ] **Step 1: Tambah test untuk gateway config**

Tambahkan ke `services/api/internal/app/config_test.go`:
```go
func TestLoadConfig_PaymentDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/x?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "secret")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WebhookPort != "8090" {
		t.Errorf("WebhookPort = %q, want 8090", cfg.WebhookPort)
	}
	if cfg.PaymentDefaultExpiry != 15*time.Minute {
		t.Errorf("PaymentDefaultExpiry = %v, want 15m", cfg.PaymentDefaultExpiry)
	}
}

func TestLoadConfig_DuitkuEnabledRequiresCreds(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/x?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("DUITKU_ENABLED", "true")
	t.Setenv("DUITKU_MERCHANT_CODE", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when DUITKU_ENABLED=true but creds missing")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/app/ -run TestLoadConfig_Payment -v; go test ./internal/app/ -run TestLoadConfig_Duitku -v; cd ../..
```
Expected: FAIL — `cfg.WebhookPort undefined`, `cfg.PaymentDefaultExpiry undefined`.

- [ ] **Step 3: Tambah field & parsing di config.go**

Tambah field ke `Config` struct:
```go
	// Payments / webhook
	WebhookPort           string
	PaymentCallbackBaseURL string
	PaymentDefaultExpiry  time.Duration

	DuitkuEnabled      bool
	DuitkuMerchantCode string
	DuitkuAPIKey       string
	DuitkuEnv          string

	XenditEnabled       bool
	XenditSecretKey     string
	XenditCallbackToken string
	XenditEnv           string
```

Tambah di akhir `LoadConfig()` sebelum `return cfg, nil`:
```go
	cfg.WebhookPort = getEnv("WEBHOOK_PORT", "8090")
	cfg.PaymentCallbackBaseURL = os.Getenv("PAYMENT_CALLBACK_BASE_URL")

	payExpiry, err := getDuration("PAYMENT_DEFAULT_EXPIRY", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	cfg.PaymentDefaultExpiry = payExpiry

	cfg.DuitkuEnabled = getEnv("DUITKU_ENABLED", "false") == "true"
	cfg.DuitkuMerchantCode = os.Getenv("DUITKU_MERCHANT_CODE")
	cfg.DuitkuAPIKey = os.Getenv("DUITKU_API_KEY")
	cfg.DuitkuEnv = getEnv("DUITKU_ENV", "sandbox")
	if cfg.DuitkuEnabled && (cfg.DuitkuMerchantCode == "" || cfg.DuitkuAPIKey == "") {
		return Config{}, fmt.Errorf("config: DUITKU_MERCHANT_CODE/DUITKU_API_KEY required when DUITKU_ENABLED=true")
	}

	cfg.XenditEnabled = getEnv("XENDIT_ENABLED", "false") == "true"
	cfg.XenditSecretKey = os.Getenv("XENDIT_SECRET_KEY")
	cfg.XenditCallbackToken = os.Getenv("XENDIT_CALLBACK_TOKEN")
	cfg.XenditEnv = getEnv("XENDIT_ENV", "sandbox")
	if cfg.XenditEnabled && (cfg.XenditSecretKey == "" || cfg.XenditCallbackToken == "") {
		return Config{}, fmt.Errorf("config: XENDIT_SECRET_KEY/XENDIT_CALLBACK_TOKEN required when XENDIT_ENABLED=true")
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/app/ -v; cd ../..
```
Expected: PASS.

- [ ] **Step 5: Update env templates**

Tambah ke `.env.example` dan `services/api/.env.example`:
```bash
# Payments / webhook
WEBHOOK_PORT=8090
PAYMENT_CALLBACK_BASE_URL=http://localhost:8090
PAYMENT_DEFAULT_EXPIRY=15m

DUITKU_ENABLED=false
DUITKU_MERCHANT_CODE=
DUITKU_API_KEY=
DUITKU_ENV=sandbox

XENDIT_ENABLED=false
XENDIT_SECRET_KEY=
XENDIT_CALLBACK_TOKEN=
XENDIT_ENV=sandbox
```

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/app/config.go services/api/internal/app/config_test.go .env.example services/api/.env.example
git commit -m "feat(config): add payment gateway and webhook env"
```

---

## Task 2: Migrations — payments, payment_webhooks, seed permission

**Files:**
- Create: `database/migrations/000XX_create_payments.sql`
- Create: `database/migrations/000XX_create_payment_webhooks.sql`
- Create: `database/migrations/000XX_seed_payment_manage.sql`

- [ ] **Step 1: Cek nomor migrasi terakhir**

Run:
```bash
ls database/migrations | sort | tail -3
```
Gunakan nomor berurutan berikutnya untuk ketiga file (mis. jika terakhir `00015`, pakai `00016/00017/00018`).

- [ ] **Step 2: Tulis migrasi payments**

Create `database/migrations/000XX_create_payments.sql`:
```sql
-- +goose Up
CREATE TABLE payments (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id            uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    order_id            uuid NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    participant_id      uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    gateway             text NOT NULL,
    method              text NOT NULL,
    channel             text,
    status              text NOT NULL DEFAULT 'PENDING',
    amount              bigint NOT NULL,
    currency            text NOT NULL DEFAULT 'IDR',
    gateway_reference   text,
    merchant_reference  text NOT NULL,
    pay_url             text,
    qr_string           text,
    va_number           text,
    instructions        jsonb,
    expires_at          timestamptz,
    paid_at             timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT payments_gateway_check CHECK (gateway IN ('duitku','xendit')),
    CONSTRAINT payments_method_check CHECK (method IN ('qris','va','ewallet')),
    CONSTRAINT payments_status_check CHECK (status IN ('PENDING','PAID','EXPIRED','FAILED')),
    CONSTRAINT payments_amount_check CHECK (amount >= 0),
    CONSTRAINT payments_merchant_ref_unique UNIQUE (merchant_reference)
);
CREATE INDEX idx_payments_order ON payments(order_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_gateway_ref ON payments(gateway, gateway_reference);
-- satu payment AKTIF (PENDING/PAID) per order:
CREATE UNIQUE INDEX uq_payments_order_active ON payments(order_id) WHERE status IN ('PENDING','PAID');

-- +goose Down
DROP TABLE payments;
```

- [ ] **Step 3: Tulis migrasi payment_webhooks**

Create `database/migrations/000XX_create_payment_webhooks.sql`:
```sql
-- +goose Up
CREATE TABLE payment_webhooks (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    gateway              text NOT NULL,
    event_type           text,
    merchant_reference   text,
    gateway_reference    text,
    signature            text,
    signature_valid      boolean NOT NULL DEFAULT false,
    payload              jsonb NOT NULL,
    dedupe_key           text,
    processing_status    text NOT NULL DEFAULT 'RECEIVED',
    processed_payment_id uuid REFERENCES payments(id),
    error_detail         text,
    received_at          timestamptz NOT NULL DEFAULT now(),
    processed_at         timestamptz,
    CONSTRAINT payment_webhooks_gateway_check CHECK (gateway IN ('duitku','xendit')),
    CONSTRAINT payment_webhooks_status_check CHECK (processing_status IN ('RECEIVED','PROCESSED','REJECTED','DUPLICATE','FAILED'))
);
CREATE UNIQUE INDEX uq_payment_webhooks_dedupe ON payment_webhooks(dedupe_key) WHERE dedupe_key IS NOT NULL;
CREATE INDEX idx_payment_webhooks_ref ON payment_webhooks(merchant_reference);
CREATE INDEX idx_payment_webhooks_status ON payment_webhooks(processing_status);

-- +goose Down
DROP TABLE payment_webhooks;
```

- [ ] **Step 4: Tulis migrasi seed permission**

Create `database/migrations/000XX_seed_payment_manage.sql` (idempotent, ikut pola seed Phase 2):
```sql
-- +goose Up
INSERT INTO permissions (key, description)
VALUES ('payment.manage', 'Reconcile/manage payments in org')
ON CONFLICT (key) DO NOTHING;

-- assign ke role template Owner & Finance (organization_id NULL, is_system=true)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.key = 'payment.manage'
WHERE r.organization_id IS NULL AND r.is_system = true AND r.slug IN ('owner','finance')
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM role_permissions
WHERE permission_id = (SELECT id FROM permissions WHERE key = 'payment.manage');
DELETE FROM permissions WHERE key = 'payment.manage';
```

Catatan: verifikasi nama kolom/slug role template di `00007_seed_rbac_catalog.sql` sebelum jalan; sesuaikan `slug IN (...)` bila berbeda.

- [ ] **Step 5: Run migrate up/down roundtrip**

Run:
```bash
make migrate-up && make migrate-down && make migrate-up
```
Expected: ketiga migrasi apply, rollback, re-apply tanpa error.

- [ ] **Step 6: Commit**

```bash
git add database/migrations/
git commit -m "feat(db): add payments, payment_webhooks, seed payment.manage"
```

---

## Task 3: sqlc queries + regenerate

**Files:**
- Create: `database/queries/payments.sql`
- Create: `database/queries/payment_webhooks.sql`

- [ ] **Step 1: Tulis query payments**

Create `database/queries/payments.sql`:
```sql
-- name: CreatePayment :one
INSERT INTO payments (
    organization_id, event_id, order_id, participant_id, gateway, method, channel,
    status, amount, currency, gateway_reference, merchant_reference, pay_url, qr_string,
    va_number, instructions, expires_at
) VALUES (
    $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17
)
RETURNING *;

-- name: GetPaymentByID :one
SELECT * FROM payments WHERE id = $1;

-- name: GetPaymentByMerchantRef :one
SELECT * FROM payments WHERE merchant_reference = $1;

-- name: LockPaymentByMerchantRef :one
SELECT * FROM payments WHERE merchant_reference = $1 FOR UPDATE;

-- name: GetPaymentByMerchantRefForUpdate :one
SELECT * FROM payments WHERE merchant_reference = $1 FOR UPDATE;

-- name: ListPaymentsByOrder :many
SELECT * FROM payments WHERE order_id = $1 ORDER BY created_at DESC;

-- name: ListPaymentsByOrgEvent :many
SELECT * FROM payments WHERE organization_id = $1 AND event_id = $2 ORDER BY created_at DESC;

-- name: GetActivePaymentByOrder :one
SELECT * FROM payments WHERE order_id = $1 AND status IN ('PENDING','PAID') LIMIT 1;

-- name: MarkPaymentPaid :one
UPDATE payments
SET status = 'PAID', paid_at = $2, gateway_reference = COALESCE($3, gateway_reference), updated_at = now()
WHERE id = $1 AND status = 'PENDING'
RETURNING *;

-- name: UpdatePaymentStatus :one
UPDATE payments
SET status = $2, updated_at = now()
WHERE id = $1 AND status = 'PENDING'
RETURNING *;
```

- [ ] **Step 2: Tulis query payment_webhooks**

Create `database/queries/payment_webhooks.sql`:
```sql
-- name: CreatePaymentWebhook :one
INSERT INTO payment_webhooks (
    gateway, event_type, merchant_reference, gateway_reference, signature,
    signature_valid, payload, processing_status
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
RETURNING *;

-- name: ClaimWebhookDedupe :one
UPDATE payment_webhooks
SET dedupe_key = $2
WHERE id = $1
RETURNING *;

-- name: MarkWebhookProcessed :exec
UPDATE payment_webhooks
SET processing_status = $2, processed_payment_id = $3, error_detail = $4, processed_at = now()
WHERE id = $1;
```

Catatan dedupe: insert dedupe via `ClaimWebhookDedupe` mengandalkan UNIQUE index parsial; konflik → pgx mengembalikan error unique-violation yang ditangani service sebagai DUPLICATE.

- [ ] **Step 3: Regenerate sqlc**

Run:
```bash
make sqlc
```
Expected: `services/api/internal/db/` ter-update dengan `CreatePayment`, `LockPaymentByMerchantRef`, dll, tanpa error.

- [ ] **Step 4: Verify build**

Run:
```bash
cd services/api && go build ./... && cd ../..
```
Expected: build OK.

- [ ] **Step 5: Commit**

```bash
git add database/queries/ services/api/internal/db/
git commit -m "feat(db): add payments + payment_webhooks sqlc queries"
```

---

## Task 4: Gateway abstraction — interface, types, registry

**Files:**
- Create: `services/api/internal/modules/payments/gateway/gateway.go`
- Create: `services/api/internal/modules/payments/gateway/registry.go`
- Test: `services/api/internal/modules/payments/gateway/registry_test.go`

- [ ] **Step 1: Tulis interface + tipe**

Create `services/api/internal/modules/payments/gateway/gateway.go`:
```go
package gateway

import (
	"context"
	"net/http"
	"time"
)

// PaymentStatus ternormalisasi lintas-gateway.
type PaymentStatus string

const (
	StatusPending PaymentStatus = "PENDING"
	StatusPaid    PaymentStatus = "PAID"
	StatusExpired PaymentStatus = "EXPIRED"
	StatusFailed  PaymentStatus = "FAILED"
)

type CreateChargeInput struct {
	MerchantReference string
	Amount            int64
	Method            string // qris|va|ewallet
	Channel           string // BCA|OVO|... opsional
	CustomerEmail     string
	CustomerName      string
	ExpiresAt         time.Time
}

type CreateChargeResult struct {
	GatewayReference string
	PayURL           string
	QRString         string
	VANumber         string
	Instructions     map[string]any
	ExpiresAt        time.Time
}

type CallbackResult struct {
	MerchantReference string
	GatewayReference  string
	Status            PaymentStatus
	Amount            int64
	PaidAt            *time.Time
	EventType         string
}

// Gateway adalah kontrak tunggal yang diimplementasikan tiap adapter.
type Gateway interface {
	Name() string
	CreateCharge(ctx context.Context, in CreateChargeInput) (CreateChargeResult, error)
	VerifySignature(headers http.Header, rawBody []byte) bool
	ParseCallback(rawBody []byte) (CallbackResult, error)
	QueryStatus(ctx context.Context, gatewayReference string) (CallbackResult, error)
}
```

- [ ] **Step 2: Tulis test registry**

Create `services/api/internal/modules/payments/gateway/registry_test.go`:
```go
package gateway

import "testing"

type stubGW struct{ name string }

func (s stubGW) Name() string { return s.name }
func (s stubGW) CreateCharge(ctx any, in any) {}

func TestRegistry_GetAndHas(t *testing.T) {
	r := NewRegistry()
	r.Register(fakeGateway{name: "duitku"})

	if !r.Has("duitku") {
		t.Fatal("expected duitku registered")
	}
	if r.Has("xendit") {
		t.Fatal("xendit should not be registered")
	}
	gw, ok := r.Get("duitku")
	if !ok || gw.Name() != "duitku" {
		t.Fatalf("Get(duitku) = %v, %v", gw, ok)
	}
	if _, ok := r.Get("midtrans"); ok {
		t.Fatal("Get(midtrans) should be false")
	}
}
```

Tambah fake yang memenuhi interface penuh (di file test yang sama):
```go
import (
	"context"
	"net/http"
)

type fakeGateway struct{ name string }

func (f fakeGateway) Name() string { return f.name }
func (f fakeGateway) CreateCharge(ctx context.Context, in CreateChargeInput) (CreateChargeResult, error) {
	return CreateChargeResult{GatewayReference: "ref"}, nil
}
func (f fakeGateway) VerifySignature(h http.Header, b []byte) bool { return true }
func (f fakeGateway) ParseCallback(b []byte) (CallbackResult, error) {
	return CallbackResult{MerchantReference: "PAY-X", Status: StatusPaid}, nil
}
func (f fakeGateway) QueryStatus(ctx context.Context, ref string) (CallbackResult, error) {
	return CallbackResult{Status: StatusPaid}, nil
}
```
Hapus `stubGW` placeholder di atas (ganti dengan `fakeGateway`).

- [ ] **Step 3: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/payments/gateway/ -v; cd ../..
```
Expected: FAIL — `undefined: NewRegistry`.

- [ ] **Step 4: Implement registry**

Create `services/api/internal/modules/payments/gateway/registry.go`:
```go
package gateway

// Registry menyimpan gateway aktif berdasarkan nama.
type Registry struct {
	gws map[string]Gateway
}

func NewRegistry() *Registry {
	return &Registry{gws: make(map[string]Gateway)}
}

func (r *Registry) Register(g Gateway) {
	r.gws[g.Name()] = g
}

func (r *Registry) Get(name string) (Gateway, bool) {
	g, ok := r.gws[name]
	return g, ok
}

func (r *Registry) Has(name string) bool {
	_, ok := r.gws[name]
	return ok
}

func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.gws))
	for n := range r.gws {
		out = append(out, n)
	}
	return out
}
```

- [ ] **Step 5: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/modules/payments/gateway/ -v; cd ../..
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/modules/payments/gateway/
git commit -m "feat(payments): add gateway interface, types, and registry"
```

---

## Task 5: Duitku adapter

**Files:**
- Create: `services/api/internal/modules/payments/gateway/duitku/duitku.go`
- Test: `services/api/internal/modules/payments/gateway/duitku/duitku_test.go`

Duitku callback signature (sandbox V2): `MD5(merchantcode + amount + merchantOrderId + apiKey)`. Status code: `00`=success, `01`=pending, `02`=cancelled/failed. (Verifikasi terhadap dok Duitku terbaru saat implementasi; mapping di satu fungsi agar mudah dikoreksi.)

- [ ] **Step 1: Tulis test signature + status mapping**

Create `services/api/internal/modules/payments/gateway/duitku/duitku_test.go`:
```go
package duitku

import (
	"crypto/md5"
	"encoding/hex"
	"net/url"
	"strings"
	"testing"

	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

func TestVerifySignature_Valid(t *testing.T) {
	a := New(Config{MerchantCode: "MC", APIKey: "KEY"})
	// signature = md5(merchantcode + amount + merchantOrderId + apiKey)
	raw := "MC" + "100000" + "PAY-1" + "KEY"
	sum := md5.Sum([]byte(raw))
	sig := hex.EncodeToString(sum[:])

	form := url.Values{}
	form.Set("merchantCode", "MC")
	form.Set("amount", "100000")
	form.Set("merchantOrderId", "PAY-1")
	form.Set("resultCode", "00")
	form.Set("signature", sig)
	body := []byte(form.Encode())

	if !a.VerifySignature(nil, body) {
		t.Fatal("expected valid signature")
	}
}

func TestVerifySignature_Invalid(t *testing.T) {
	a := New(Config{MerchantCode: "MC", APIKey: "KEY"})
	body := []byte("merchantCode=MC&amount=100000&merchantOrderId=PAY-1&signature=deadbeef")
	if a.VerifySignature(nil, body) {
		t.Fatal("expected invalid signature")
	}
}

func TestParseCallback_StatusMapping(t *testing.T) {
	a := New(Config{MerchantCode: "MC", APIKey: "KEY"})
	cases := map[string]gw.PaymentStatus{
		"00": gw.StatusPaid,
		"01": gw.StatusPending,
		"02": gw.StatusFailed,
	}
	for code, want := range cases {
		body := []byte("merchantOrderId=PAY-1&amount=100000&resultCode=" + code + "&reference=DTREF")
		res, err := a.ParseCallback(body)
		if err != nil {
			t.Fatalf("code %s: %v", code, err)
		}
		if res.Status != want {
			t.Errorf("code %s: status = %s, want %s", code, res.Status, want)
		}
		if res.MerchantReference != "PAY-1" {
			t.Errorf("code %s: ref = %s", code, res.MerchantReference)
		}
		if !strings.EqualFold(res.GatewayReference, "DTREF") {
			t.Errorf("code %s: gwref = %s", code, res.GatewayReference)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/payments/gateway/duitku/ -v; cd ../..
```
Expected: FAIL — `undefined: New`.

- [ ] **Step 3: Implement adapter**

Create `services/api/internal/modules/payments/gateway/duitku/duitku.go`:
```go
package duitku

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

type Config struct {
	MerchantCode string
	APIKey       string
	Env          string // sandbox|production
	HTTPClient   *http.Client
}

type Adapter struct {
	cfg    Config
	client *http.Client
}

func New(cfg Config) *Adapter {
	c := cfg.HTTPClient
	if c == nil {
		c = http.DefaultClient
	}
	return &Adapter{cfg: cfg, client: c}
}

func (a *Adapter) Name() string { return "duitku" }

func (a *Adapter) VerifySignature(_ http.Header, rawBody []byte) bool {
	form, err := url.ParseQuery(string(rawBody))
	if err != nil {
		return false
	}
	merchantCode := form.Get("merchantCode")
	amount := form.Get("amount")
	orderID := form.Get("merchantOrderId")
	got := form.Get("signature")
	if got == "" {
		return false
	}
	raw := merchantCode + amount + orderID + a.cfg.APIKey
	sum := md5.Sum([]byte(raw))
	want := hex.EncodeToString(sum[:])
	return hmacishEqual(want, got)
}

func (a *Adapter) ParseCallback(rawBody []byte) (gw.CallbackResult, error) {
	form, err := url.ParseQuery(string(rawBody))
	if err != nil {
		return gw.CallbackResult{}, fmt.Errorf("duitku: parse callback: %w", err)
	}
	amount, _ := strconv.ParseInt(form.Get("amount"), 10, 64)
	return gw.CallbackResult{
		MerchantReference: form.Get("merchantOrderId"),
		GatewayReference:  form.Get("reference"),
		Status:            mapStatus(form.Get("resultCode")),
		Amount:            amount,
		EventType:         "duitku.callback",
	}, nil
}

func (a *Adapter) CreateCharge(ctx context.Context, in gw.CreateChargeInput) (gw.CreateChargeResult, error) {
	// Implementasi nyata: POST ke endpoint inquiry Duitku (sandbox/production) dengan
	// signature MD5(merchantCode + merchantOrderId + amount + apiKey). Untuk plan ini,
	// fokus pada kontrak; isi HTTP call + mapping response saat implementasi, dengan
	// a.client agar bisa di-fake di test.
	return gw.CreateChargeResult{}, fmt.Errorf("duitku: CreateCharge not yet implemented")
}

func (a *Adapter) QueryStatus(ctx context.Context, gatewayReference string) (gw.CallbackResult, error) {
	return gw.CallbackResult{}, fmt.Errorf("duitku: QueryStatus not yet implemented")
}

func mapStatus(resultCode string) gw.PaymentStatus {
	switch resultCode {
	case "00":
		return gw.StatusPaid
	case "01":
		return gw.StatusPending
	default: // "02" dan lainnya
		return gw.StatusFailed
	}
}

// hmacishEqual membandingkan dua hex string secara aman dari segi panjang.
func hmacishEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
```

Catatan: `CreateCharge`/`QueryStatus` sengaja dibiarkan eksplisit "not yet implemented" sebagai kontrak — DIISI di sub-task tersendiri saat menyiapkan kredensial sandbox. Signature & ParseCallback (jalur callback yang kritikal) diuji penuh sekarang.

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/modules/payments/gateway/duitku/ -v; cd ../..
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/payments/gateway/duitku/
git commit -m "feat(payments): add duitku adapter (signature, parse, status mapping)"
```

---

## Task 6: Xendit adapter

**Files:**
- Create: `services/api/internal/modules/payments/gateway/xendit/xendit.go`
- Test: `services/api/internal/modules/payments/gateway/xendit/xendit_test.go`

Xendit callback diverifikasi via header `x-callback-token` dibandingkan dengan `XENDIT_CALLBACK_TOKEN`. Payload JSON; status field (mis. `status: "PAID"/"PENDING"/"EXPIRED"`).

- [ ] **Step 1: Tulis test token + parse JSON**

Create `services/api/internal/modules/payments/gateway/xendit/xendit_test.go`:
```go
package xendit

import (
	"net/http"
	"testing"

	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

func TestVerifySignature_Token(t *testing.T) {
	a := New(Config{SecretKey: "sk", CallbackToken: "tok123"})
	h := http.Header{}
	h.Set("x-callback-token", "tok123")
	if !a.VerifySignature(h, nil) {
		t.Fatal("expected valid token")
	}
	h.Set("x-callback-token", "wrong")
	if a.VerifySignature(h, nil) {
		t.Fatal("expected invalid token")
	}
}

func TestParseCallback_JSON(t *testing.T) {
	a := New(Config{SecretKey: "sk", CallbackToken: "tok"})
	body := []byte(`{"external_id":"PAY-1","id":"xnd-123","status":"PAID","amount":100000}`)
	res, err := a.ParseCallback(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if res.MerchantReference != "PAY-1" || res.GatewayReference != "xnd-123" {
		t.Errorf("refs: %+v", res)
	}
	if res.Status != gw.StatusPaid {
		t.Errorf("status = %s, want PAID", res.Status)
	}
	if res.Amount != 100000 {
		t.Errorf("amount = %d", res.Amount)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/payments/gateway/xendit/ -v; cd ../..
```
Expected: FAIL — `undefined: New`.

- [ ] **Step 3: Implement adapter**

Create `services/api/internal/modules/payments/gateway/xendit/xendit.go`:
```go
package xendit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

type Config struct {
	SecretKey     string
	CallbackToken string
	Env           string
	HTTPClient    *http.Client
}

type Adapter struct {
	cfg    Config
	client *http.Client
}

func New(cfg Config) *Adapter {
	c := cfg.HTTPClient
	if c == nil {
		c = http.DefaultClient
	}
	return &Adapter{cfg: cfg, client: c}
}

func (a *Adapter) Name() string { return "xendit" }

func (a *Adapter) VerifySignature(headers http.Header, _ []byte) bool {
	if headers == nil {
		return false
	}
	got := headers.Get("x-callback-token")
	return got != "" && constantEqual(got, a.cfg.CallbackToken)
}

type callbackPayload struct {
	ExternalID string `json:"external_id"`
	ID         string `json:"id"`
	Status     string `json:"status"`
	Amount     int64  `json:"amount"`
}

func (a *Adapter) ParseCallback(rawBody []byte) (gw.CallbackResult, error) {
	var p callbackPayload
	if err := json.Unmarshal(rawBody, &p); err != nil {
		return gw.CallbackResult{}, fmt.Errorf("xendit: parse callback: %w", err)
	}
	return gw.CallbackResult{
		MerchantReference: p.ExternalID,
		GatewayReference:  p.ID,
		Status:            mapStatus(p.Status),
		Amount:            p.Amount,
		EventType:         "xendit.callback",
	}, nil
}

func (a *Adapter) CreateCharge(ctx context.Context, in gw.CreateChargeInput) (gw.CreateChargeResult, error) {
	return gw.CreateChargeResult{}, fmt.Errorf("xendit: CreateCharge not yet implemented")
}

func (a *Adapter) QueryStatus(ctx context.Context, gatewayReference string) (gw.CallbackResult, error) {
	return gw.CallbackResult{}, fmt.Errorf("xendit: QueryStatus not yet implemented")
}

func mapStatus(s string) gw.PaymentStatus {
	switch strings.ToUpper(s) {
	case "PAID", "SETTLED", "SUCCEEDED", "COMPLETED":
		return gw.StatusPaid
	case "PENDING", "ACTIVE":
		return gw.StatusPending
	case "EXPIRED":
		return gw.StatusExpired
	default:
		return gw.StatusFailed
	}
}

func constantEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/modules/payments/gateway/xendit/ -v; cd ../..
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/payments/gateway/xendit/
git commit -m "feat(payments): add xendit adapter (token verify, parse, status mapping)"
```

---

## Task 7: Payments errors, model, dto, merchant-ref generator

**Files:**
- Create: `services/api/internal/modules/payments/errors.go`
- Create: `services/api/internal/modules/payments/model.go`
- Create: `services/api/internal/modules/payments/dto.go`
- Create: `services/api/internal/modules/payments/merchantref.go`
- Test: `services/api/internal/modules/payments/merchantref_test.go`

- [ ] **Step 1: Tulis errors.go**

Create `services/api/internal/modules/payments/errors.go`:
```go
package payments

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrPaymentNotFound   = apperr.New(http.StatusNotFound, "PAYMENT_NOT_FOUND", "payment not found")
	ErrOrderNotPayable   = apperr.New(http.StatusConflict, "ORDER_NOT_PAYABLE", "order is not pending payment")
	ErrPaymentActive     = apperr.New(http.StatusConflict, "PAYMENT_ALREADY_ACTIVE", "order already has an active payment")
	ErrGatewayNotAvail   = apperr.New(http.StatusBadRequest, "GATEWAY_NOT_AVAILABLE", "gateway or method not available")
	ErrUnsupportedMethod = apperr.New(http.StatusBadRequest, "UNSUPPORTED_METHOD", "payment method not supported")
	ErrGatewayError      = apperr.New(http.StatusBadGateway, "GATEWAY_ERROR", "payment gateway error")
	ErrAmountMismatch    = apperr.New(http.StatusBadRequest, "PAYMENT_AMOUNT_MISMATCH", "callback amount does not match")
	ErrInvalidSignature  = apperr.New(http.StatusUnauthorized, "INVALID_SIGNATURE", "invalid callback signature")
	ErrReconcileFailed   = apperr.New(http.StatusBadGateway, "RECONCILE_FAILED", "reconcile failed")
	ErrMerchantRefGen    = apperr.New(http.StatusInternalServerError, "MERCHANT_REF_GENERATION_FAILED", "could not generate merchant reference")
)
```

- [ ] **Step 2: Tulis model.go**

Create `services/api/internal/modules/payments/model.go`:
```go
package payments

import gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"

// Status payment di DB (string).
const (
	StatusPending = "PENDING"
	StatusPaid    = "PAID"
	StatusExpired = "EXPIRED"
	StatusFailed  = "FAILED"
)

// Status webhook processing.
const (
	WebhookReceived  = "RECEIVED"
	WebhookProcessed = "PROCESSED"
	WebhookRejected  = "REJECTED"
	WebhookDuplicate = "DUPLICATE"
	WebhookFailed    = "FAILED"
)

// Reservation status saat payment sukses (Phase 5 enum).
const ReservationCompleted = "COMPLETED"

// Order status Phase 5 yang dipakai Phase 6.
const (
	OrderPendingPayment = "PENDING_PAYMENT"
	OrderPaid           = "PAID"
)

// dbStatusFromGateway memetakan status gateway ternormalisasi → string status DB.
func dbStatusFromGateway(s gw.PaymentStatus) string {
	switch s {
	case gw.StatusPaid:
		return StatusPaid
	case gw.StatusExpired:
		return StatusExpired
	case gw.StatusFailed:
		return StatusFailed
	default:
		return StatusPending
	}
}
```

Catatan: verifikasi konstanta order/reservation Phase 5 (`StatusPendingPayment`, `StatusPaid`, `"COMPLETED"`) cocok dengan yang ada di module `orders`/`inventory`; sesuaikan bila beda.

- [ ] **Step 3: Tulis dto.go**

Create `services/api/internal/modules/payments/dto.go`:
```go
package payments

import (
	"time"

	"github.com/google/uuid"
)

type CreatePaymentRequest struct {
	Gateway string `json:"gateway"`
	Method  string `json:"method"`
	Channel string `json:"channel,omitempty"`
}

type PaymentResponse struct {
	ID                uuid.UUID  `json:"id"`
	OrderID           uuid.UUID  `json:"orderId"`
	Gateway           string     `json:"gateway"`
	Method            string     `json:"method"`
	Channel           string     `json:"channel,omitempty"`
	Status            string     `json:"status"`
	Amount            int64      `json:"amount"`
	Currency          string     `json:"currency"`
	MerchantReference string     `json:"merchantReference"`
	GatewayReference  string     `json:"gatewayReference,omitempty"`
	PayURL            string     `json:"payUrl,omitempty"`
	QRString          string     `json:"qrString,omitempty"`
	VANumber          string     `json:"vaNumber,omitempty"`
	ExpiresAt         *time.Time `json:"expiresAt,omitempty"`
	PaidAt            *time.Time `json:"paidAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
}
```

- [ ] **Step 4: Tulis test generator merchant ref**

Create `services/api/internal/modules/payments/merchantref_test.go`:
```go
package payments

import (
	"regexp"
	"testing"
	"time"
)

func TestGenerateMerchantReference_Format(t *testing.T) {
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	ref, err := generateMerchantReference(now)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	re := regexp.MustCompile(`^PAY-20260607-[0-9A-Z]{6}$`)
	if !re.MatchString(ref) {
		t.Errorf("ref %q does not match expected format", ref)
	}
}

func TestGenerateMerchantReference_Unique(t *testing.T) {
	now := time.Now()
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		ref, err := generateMerchantReference(now)
		if err != nil {
			t.Fatal(err)
		}
		if seen[ref] {
			t.Fatalf("collision on %s", ref)
		}
		seen[ref] = true
	}
}
```

- [ ] **Step 5: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/payments/ -run TestGenerateMerchantReference -v; cd ../..
```
Expected: FAIL — `undefined: generateMerchantReference`.

- [ ] **Step 6: Implement merchantref.go**

Create `services/api/internal/modules/payments/merchantref.go`:
```go
package payments

import (
	"crypto/rand"
	"fmt"
	"time"
)

const refAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// generateMerchantReference: PAY-YYYYMMDD-XXXXXX (6 char alfanumerik uppercase acak).
func generateMerchantReference(now time.Time) (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	suffix := make([]byte, 6)
	for i := range b {
		suffix[i] = refAlphabet[int(b[i])%len(refAlphabet)]
	}
	return fmt.Sprintf("PAY-%s-%s", now.Format("20060102"), string(suffix)), nil
}
```

- [ ] **Step 7: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/modules/payments/ -run TestGenerateMerchantReference -v; cd ../..
```
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add services/api/internal/modules/payments/errors.go services/api/internal/modules/payments/model.go services/api/internal/modules/payments/dto.go services/api/internal/modules/payments/merchantref.go services/api/internal/modules/payments/merchantref_test.go
git commit -m "feat(payments): add errors, model, dto, merchant-ref generator"
```

---

## Task 8: Repository — payments + webhooks + ExecTx + order/reservation access

**Files:**
- Create: `services/api/internal/modules/payments/repository.go`

Repo membungkus sqlc queries dan menyediakan `ExecTx` (pola persis Phase 5 orders). Dalam tx,
processor mengakses payment, order, dan reservation lewat repo yang berbagi `db.Queries` ber-tx.

- [ ] **Step 1: Implement repository**

Create `services/api/internal/modules/payments/repository.go`:
```go
package payments

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

// ErrDuplicateDedupe ditandai saat dedupe_key bentrok (callback duplikat).
var ErrDuplicateDedupe = errors.New("duplicate dedupe key")

type Repository interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error

	CreatePayment(ctx context.Context, arg db.CreatePaymentParams) (db.Payment, error)
	GetPaymentByID(ctx context.Context, id uuid.UUID) (db.Payment, error)
	GetPaymentByMerchantRefForUpdate(ctx context.Context, ref string) (db.Payment, error)
	ListPaymentsByOrder(ctx context.Context, orderID uuid.UUID) ([]db.Payment, error)
	ListPaymentsByOrgEvent(ctx context.Context, arg db.ListPaymentsByOrgEventParams) ([]db.Payment, error)
	GetActivePaymentByOrder(ctx context.Context, orderID uuid.UUID) (db.Payment, error)
	MarkPaymentPaid(ctx context.Context, arg db.MarkPaymentPaidParams) (db.Payment, error)
	UpdatePaymentStatus(ctx context.Context, arg db.UpdatePaymentStatusParams) (db.Payment, error)

	CreatePaymentWebhook(ctx context.Context, arg db.CreatePaymentWebhookParams) (db.PaymentWebhook, error)
	ClaimWebhookDedupe(ctx context.Context, id uuid.UUID, dedupeKey string) error
	MarkWebhookProcessed(ctx context.Context, arg db.MarkWebhookProcessedParams) error

	// Order/reservation Phase 5 (dipakai saat transisi PAID dalam tx yang sama).
	GetOrderByIDForUpdate(ctx context.Context, id uuid.UUID) (db.Order, error)
	UpdateOrderStatus(ctx context.Context, arg db.UpdateOrderStatusParams) (db.Order, error)
	CompleteReservationsForOrder(ctx context.Context, orderID uuid.UUID) error
}

type sqlcRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &sqlcRepo{pool: pool, q: db.New(pool)}
}

func (r *sqlcRepo) ExecTx(ctx context.Context, fn func(Repository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	txRepo := &sqlcRepo{pool: r.pool, q: r.q.WithTx(tx)}
	if err := fn(txRepo); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *sqlcRepo) CreatePayment(ctx context.Context, arg db.CreatePaymentParams) (db.Payment, error) {
	return r.q.CreatePayment(ctx, arg)
}
func (r *sqlcRepo) GetPaymentByID(ctx context.Context, id uuid.UUID) (db.Payment, error) {
	return r.q.GetPaymentByID(ctx, id)
}
func (r *sqlcRepo) GetPaymentByMerchantRefForUpdate(ctx context.Context, ref string) (db.Payment, error) {
	return r.q.GetPaymentByMerchantRefForUpdate(ctx, ref)
}
func (r *sqlcRepo) ListPaymentsByOrder(ctx context.Context, orderID uuid.UUID) ([]db.Payment, error) {
	return r.q.ListPaymentsByOrder(ctx, orderID)
}
func (r *sqlcRepo) ListPaymentsByOrgEvent(ctx context.Context, arg db.ListPaymentsByOrgEventParams) ([]db.Payment, error) {
	return r.q.ListPaymentsByOrgEvent(ctx, arg)
}
func (r *sqlcRepo) GetActivePaymentByOrder(ctx context.Context, orderID uuid.UUID) (db.Payment, error) {
	return r.q.GetActivePaymentByOrder(ctx, orderID)
}
func (r *sqlcRepo) MarkPaymentPaid(ctx context.Context, arg db.MarkPaymentPaidParams) (db.Payment, error) {
	return r.q.MarkPaymentPaid(ctx, arg)
}
func (r *sqlcRepo) UpdatePaymentStatus(ctx context.Context, arg db.UpdatePaymentStatusParams) (db.Payment, error) {
	return r.q.UpdatePaymentStatus(ctx, arg)
}
func (r *sqlcRepo) CreatePaymentWebhook(ctx context.Context, arg db.CreatePaymentWebhookParams) (db.PaymentWebhook, error) {
	return r.q.CreatePaymentWebhook(ctx, arg)
}
func (r *sqlcRepo) ClaimWebhookDedupe(ctx context.Context, id uuid.UUID, dedupeKey string) error {
	_, err := r.q.ClaimWebhookDedupe(ctx, db.ClaimWebhookDedupeParams{ID: id, DedupeKey: pgtypeText(dedupeKey)})
	if isUniqueViolation(err) {
		return ErrDuplicateDedupe
	}
	return err
}
func (r *sqlcRepo) MarkWebhookProcessed(ctx context.Context, arg db.MarkWebhookProcessedParams) error {
	return r.q.MarkWebhookProcessed(ctx, arg)
}

// Order/reservation: query sqlc Phase 5 yang sudah ada — verifikasi nama persisnya.
func (r *sqlcRepo) GetOrderByIDForUpdate(ctx context.Context, id uuid.UUID) (db.Order, error) {
	return r.q.GetOrderByIDForUpdate(ctx, id)
}
func (r *sqlcRepo) UpdateOrderStatus(ctx context.Context, arg db.UpdateOrderStatusParams) (db.Order, error) {
	return r.q.UpdateOrderStatus(ctx, arg)
}
func (r *sqlcRepo) CompleteReservationsForOrder(ctx context.Context, orderID uuid.UUID) error {
	return r.q.UpdateReservationStatusByOrder(ctx, db.UpdateReservationStatusByOrderParams{
		OrderID: orderID, Status: "COMPLETED",
	})
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
```

Import tambahan yang diperlukan di file ini: `github.com/jackc/pgx/v5/pgconn` (untuk `PgError`),
`github.com/jackc/pgx/v5/pgtype` (helper `pgtypeText`). Tambahkan helper kecil:
```go
func pgtypeText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}
```
Hapus import `pgx` jika tidak terpakai. Jika query Phase 5 belum punya `GetOrderByIDForUpdate`
atau `UpdateReservationStatusByOrder`, tambahkan ke `database/queries/` Phase 6 (query baru,
tidak mengubah Phase 5) lalu `make sqlc`. Verifikasi nama di Task 3.

- [ ] **Step 2: Tambah query order lock bila belum ada**

Jika `GetOrderByIDForUpdate` belum ada, tambah ke `database/queries/payments.sql`:
```sql
-- name: GetOrderByIDForUpdate :one
SELECT * FROM orders WHERE id = $1 FOR UPDATE;
```
Lalu `make sqlc`.

- [ ] **Step 3: Verify build**

Run:
```bash
cd services/api && go build ./internal/modules/payments/... && cd ../..
```
Expected: build OK (mungkin perlu sesuaikan nama param sqlc).

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/modules/payments/repository.go database/queries/ services/api/internal/db/
git commit -m "feat(payments): add repository with ExecTx and order/reservation access"
```

---

## Task 9: Processor — idempotent ProcessCallback (CRITICAL CORE)

**Files:**
- Create: `services/api/internal/modules/payments/processor.go`
- Test: `services/api/internal/modules/payments/processor_test.go`

Ini inti korektnya: dipanggil webhook (live) & reconcile. Diuji unit dengan fake repo.

- [ ] **Step 1: Tulis test processor (idempotency, signature, amount, race)**

Create `services/api/internal/modules/payments/processor_test.go`:
```go
package payments

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

// fakeProcRepo: in-memory repo yang memenuhi Repository untuk menguji processor.
// (Implementasikan minimal: simpan satu payment + satu order, track jumlah transisi.)
// — lihat catatan di bawah; gunakan map dan flag.

func TestProcessCallback_PaidTransitionsOrderOnce(t *testing.T) {
	repo := newFakeProcRepo()
	pid := uuid.New()
	oid := uuid.New()
	repo.addPayment(db.Payment{ID: pid, OrderID: oid, MerchantReference: "PAY-1", Status: StatusPending, Amount: 100000})
	repo.addOrder(db.Order{ID: oid, Status: OrderPendingPayment})

	p := NewProcessor(repo, nil)
	res := gw.CallbackResult{MerchantReference: "PAY-1", GatewayReference: "ref1", Status: gw.StatusPaid, Amount: 100000}

	// proses dua kali — efek harus sekali
	if err := p.apply(context.Background(), "duitku", res); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if err := p.apply(context.Background(), "duitku", res); err != nil {
		t.Fatalf("second apply: %v", err)
	}

	if repo.orderPaidCount != 1 {
		t.Errorf("order transitioned %d times, want 1", repo.orderPaidCount)
	}
	if repo.reservationCompletedCount != 1 {
		t.Errorf("reservation completed %d times, want 1", repo.reservationCompletedCount)
	}
}

func TestProcessCallback_AmountMismatchRejected(t *testing.T) {
	repo := newFakeProcRepo()
	oid := uuid.New()
	repo.addPayment(db.Payment{ID: uuid.New(), OrderID: oid, MerchantReference: "PAY-1", Status: StatusPending, Amount: 100000})
	repo.addOrder(db.Order{ID: oid, Status: OrderPendingPayment})

	p := NewProcessor(repo, nil)
	res := gw.CallbackResult{MerchantReference: "PAY-1", Status: gw.StatusPaid, Amount: 999}
	err := p.apply(context.Background(), "duitku", res)
	if err != ErrAmountMismatch {
		t.Fatalf("err = %v, want ErrAmountMismatch", err)
	}
	if repo.orderPaidCount != 0 {
		t.Error("order should not transition on amount mismatch")
	}
}

func TestProcessCallback_PaymentNotFound(t *testing.T) {
	repo := newFakeProcRepo()
	p := NewProcessor(repo, nil)
	res := gw.CallbackResult{MerchantReference: "PAY-UNKNOWN", Status: gw.StatusPaid}
	if err := p.apply(context.Background(), "duitku", res); err != ErrPaymentNotFound {
		t.Fatalf("err = %v, want ErrPaymentNotFound", err)
	}
}

func TestProcessCallback_OrderAlreadyExpired(t *testing.T) {
	repo := newFakeProcRepo()
	oid := uuid.New()
	repo.addPayment(db.Payment{ID: uuid.New(), OrderID: oid, MerchantReference: "PAY-1", Status: StatusPending, Amount: 100000})
	repo.addOrder(db.Order{ID: oid, Status: "EXPIRED"}) // order keburu expired

	p := NewProcessor(repo, nil)
	res := gw.CallbackResult{MerchantReference: "PAY-1", Status: gw.StatusPaid, Amount: 100000, PaidAt: ptrTime(time.Now())}
	err := p.apply(context.Background(), "duitku", res)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	// payment ditandai PAID, tapi order TIDAK berubah
	if repo.paymentPaidCount != 1 {
		t.Error("payment should be marked paid")
	}
	if repo.orderPaidCount != 0 {
		t.Error("expired order must not transition to PAID")
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
```

Catatan: `newFakeProcRepo()` adalah fake in-memory yang mengimplementasikan `Repository`.
Implementasikan di file test (atau `processor_fake_test.go`) dengan: map merchantRef→Payment,
map orderID→Order, counter `orderPaidCount`/`reservationCompletedCount`/`paymentPaidCount`,
`ExecTx` yang langsung memanggil fn dengan dirinya sendiri (tanpa tx nyata), dan
`MarkPaymentPaid` yang hanya berhasil bila status saat ini PENDING (mengembalikan baris
PAID; pada panggilan kedua status sudah PAID → kembalikan `pgx.ErrNoRows` agar processor
memperlakukannya sebagai no-op idempotent). Ini menegakkan "efek sekali".

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/payments/ -run TestProcessCallback -v; cd ../..
```
Expected: FAIL — `undefined: NewProcessor`.

- [ ] **Step 3: Implement processor.go**

Create `services/api/internal/modules/payments/processor.go`:
```go
package payments

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/audit"
	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

type AuditRecorder interface {
	Record(ctx context.Context, e audit.Entry)
}

type Processor struct {
	repo  Repository
	audit AuditRecorder
}

func NewProcessor(repo Repository, recorder AuditRecorder) *Processor {
	return &Processor{repo: repo, audit: recorder}
}

// ProcessRaw: store-then-process. Dipakai webhook receiver.
// 1) simpan raw webhook (selalu), 2) verifikasi signature, 3) parse, 4) apply idempotent.
func (p *Processor) ProcessRaw(ctx context.Context, g gw.Gateway, headers map[string][]string, rawBody []byte) error {
	signatureValid := g.VerifySignature(headers, rawBody)

	parsed, parseErr := g.ParseCallback(rawBody)
	_, err := p.repo.CreatePaymentWebhook(ctx, db.CreatePaymentWebhookParams{
		Gateway:           g.Name(),
		EventType:         pgtypeText(parsed.EventType),
		MerchantReference: pgtypeText(parsed.MerchantReference),
		GatewayReference:  pgtypeText(parsed.GatewayReference),
		Signature:         pgtypeText(headerSignature(g.Name(), headers)),
		SignatureValid:    signatureValid,
		Payload:           rawBody,
		ProcessingStatus:  WebhookReceived,
	})
	if err != nil {
		return fmt.Errorf("store webhook: %w", err)
	}

	if !signatureValid {
		p.recordRejected(ctx, g.Name(), "INVALID_SIGNATURE")
		return ErrInvalidSignature
	}
	if parseErr != nil {
		return fmt.Errorf("parse callback: %w", parseErr)
	}
	return p.apply(ctx, g.Name(), parsed)
}

// apply menjalankan transisi idempotent dalam satu transaksi.
func (p *Processor) apply(ctx context.Context, gateway string, res gw.CallbackResult) error {
	dedupe := dedupeKey(gateway, res)

	return p.repo.ExecTx(ctx, func(tx Repository) error {
		payment, err := tx.GetPaymentByMerchantRefForUpdate(ctx, res.MerchantReference)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPaymentNotFound
		} else if err != nil {
			return err
		}
		if res.Amount != 0 && res.Amount != payment.Amount {
			return ErrAmountMismatch
		}

		// payment sudah final → no-op idempotent
		if payment.Status != StatusPending {
			return nil
		}

		switch res.Status {
		case gw.StatusPaid:
			return p.applyPaid(ctx, tx, payment, res, dedupe)
		case gw.StatusExpired, gw.StatusFailed:
			_, err := tx.UpdatePaymentStatus(ctx, db.UpdatePaymentStatusParams{
				ID: payment.ID, Status: dbStatusFromGateway(res.Status),
			})
			return err
		default:
			return nil // PENDING callback: tak ada transisi
		}
	})
}

func (p *Processor) applyPaid(ctx context.Context, tx Repository, payment db.Payment, res gw.CallbackResult, dedupe string) error {
	paidAt := pgtype.Timestamptz{Valid: true}
	if res.PaidAt != nil {
		paidAt.Time = *res.PaidAt
	} else {
		paidAt = pgtype.Timestamptz{Valid: true}
	}
	updated, err := tx.MarkPaymentPaid(ctx, db.MarkPaymentPaidParams{
		ID: payment.ID, PaidAt: paidAt, GatewayReference: pgtypeText(res.GatewayReference),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // sudah PAID oleh proses lain → idempotent no-op
	} else if err != nil {
		return err
	}

	// transisi order Phase 5 — hanya jika masih PENDING_PAYMENT
	order, err := tx.GetOrderByIDForUpdate(ctx, updated.OrderID)
	if err != nil {
		return err
	}
	if order.Status == OrderPendingPayment {
		if _, err := tx.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID: order.ID, Status: OrderPaid, Status_2: OrderPendingPayment,
		}); err != nil {
			return err
		}
		if err := tx.CompleteReservationsForOrder(ctx, order.ID); err != nil {
			return err
		}
		p.recordPaid(ctx, updated, "")
	} else {
		// race: order sudah EXPIRED/CANCELLED — payment PAID, order tak berubah
		p.recordPaid(ctx, updated, "ORDER_ALREADY_"+order.Status)
	}
	return nil
}

func dedupeKey(gateway string, res gw.CallbackResult) string {
	ref := res.GatewayReference
	if ref == "" {
		ref = res.MerchantReference
	}
	return gateway + ":" + ref + ":" + string(res.Status)
}

func headerSignature(gateway string, headers map[string][]string) string {
	if headers == nil {
		return ""
	}
	switch gateway {
	case "xendit":
		if v := headers["X-Callback-Token"]; len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

func (p *Processor) recordPaid(ctx context.Context, payment db.Payment, note string) {
	if p.audit == nil {
		return
	}
	oid := payment.OrganizationID
	uid := payment.ParticipantID
	meta := map[string]any{"merchantReference": payment.MerchantReference}
	if note != "" {
		meta["note"] = note
	}
	p.audit.Record(ctx, audit.Entry{
		OrganizationID: &oid, ActorUserID: &uid, Action: "PAYMENT_PAID",
		TargetType: "payment", TargetID: payment.ID.String(), Metadata: meta,
	})
}

func (p *Processor) recordRejected(ctx context.Context, gateway, reason string) {
	if p.audit == nil {
		return
	}
	p.audit.Record(ctx, audit.Entry{
		Action: "PAYMENT_CALLBACK_REJECTED", TargetType: "webhook",
		Metadata: map[string]any{"gateway": gateway, "reason": reason},
	})
}
```

Catatan: `dedupe` di-claim di webhook receiver (Task 12) lewat `ClaimWebhookDedupe` SEBELUM
`apply`, atau di awal `apply`. Untuk menjaga inti tetap idempotent walau dedupe race lolos,
`MarkPaymentPaid` ber-guard `status='PENDING'` adalah pengaman kedua (efek tetap sekali).
Sesuaikan signature `UpdateOrderStatusParams.Status_2` dengan yang dihasilkan sqlc Phase 5.

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/modules/payments/ -run TestProcessCallback -v; cd ../..
```
Expected: PASS (semua kasus: paid-once, amount mismatch, not found, order-expired race).

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/payments/processor.go services/api/internal/modules/payments/processor_test.go
git commit -m "feat(payments): add idempotent callback processor (core)"
```

---

## Task 10: Service — create charge, get/list, eligibility & ownership

**Files:**
- Create: `services/api/internal/modules/payments/service.go`
- Test: `services/api/internal/modules/payments/service_test.go`

- [ ] **Step 1: Tulis test service (create charge happy + guards)**

Create `services/api/internal/modules/payments/service_test.go` dengan kasus:
- create payment sukses → payment PENDING, charge gateway dipanggil, merchant ref ter-set.
- order bukan PENDING_PAYMENT → `ErrOrderNotPayable`.
- order milik user lain → `ErrPaymentNotFound` (ownership; 404).
- gateway/method tak aktif (tidak di registry) → `ErrGatewayNotAvail`.
- sudah ada payment aktif untuk order → `ErrPaymentActive`.

Gunakan fake `Repository` + `gateway.Registry` berisi `fakeGateway` yang mengembalikan
`CreateChargeResult` valid. (Pola fake sama seperti Task 4/9.)

```go
package payments

import (
	"context"
	"testing"

	"github.com/google/uuid"

	gwpkg "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

func TestCreatePayment_OrderNotPayable(t *testing.T) {
	repo := newFakeProcRepo()
	oid := uuid.New()
	uid := uuid.New()
	repo.addOrder(db.Order{ID: oid, ParticipantID: uid, Status: "PAID", Total: 100000})

	reg := gwpkg.NewRegistry()
	reg.Register(fakeGateway{name: "duitku"})
	svc := NewService(repo, reg, nil, 15*60)

	_, err := svc.CreatePayment(context.Background(), uid, oid, CreatePaymentRequest{Gateway: "duitku", Method: "qris"})
	if err != ErrOrderNotPayable {
		t.Fatalf("err = %v, want ErrOrderNotPayable", err)
	}
}

func TestCreatePayment_GatewayNotAvailable(t *testing.T) {
	repo := newFakeProcRepo()
	oid := uuid.New()
	uid := uuid.New()
	repo.addOrder(db.Order{ID: oid, ParticipantID: uid, Status: OrderPendingPayment, Total: 100000})

	reg := gwpkg.NewRegistry() // kosong
	svc := NewService(repo, reg, nil, 900)

	_, err := svc.CreatePayment(context.Background(), uid, oid, CreatePaymentRequest{Gateway: "duitku", Method: "qris"})
	if err != ErrGatewayNotAvail {
		t.Fatalf("err = %v, want ErrGatewayNotAvail", err)
	}
}
```

(Tambahkan `addOrder` ke fake bila belum; sertakan field `ParticipantID`, `Total`,
`OrganizationID`, `EventID`.)

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/payments/ -run TestCreatePayment -v; cd ../..
```
Expected: FAIL — `undefined: NewService`.

- [ ] **Step 3: Implement service.go**

Create `services/api/internal/modules/payments/service.go`:
```go
package payments

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

var validMethods = map[string]bool{"qris": true, "va": true, "ewallet": true}

type Service struct {
	repo     Repository
	registry *gw.Registry
	audit    AuditRecorder
	expiry   time.Duration
}

func NewService(repo Repository, registry *gw.Registry, recorder AuditRecorder, expiry time.Duration) *Service {
	return &Service{repo: repo, registry: registry, audit: recorder, expiry: expiry}
}

func (s *Service) CreatePayment(ctx context.Context, participantID, orderID uuid.UUID, req CreatePaymentRequest) (PaymentResponse, error) {
	if !validMethods[req.Method] {
		return PaymentResponse{}, ErrUnsupportedMethod
	}
	g, ok := s.registry.Get(req.Gateway)
	if !ok {
		return PaymentResponse{}, ErrGatewayNotAvail
	}

	order, err := s.repo.GetOrderByIDForUpdate(ctx, orderID) // non-tx read ok di luar; lihat catatan
	if errors.Is(err, pgx.ErrNoRows) {
		return PaymentResponse{}, ErrPaymentNotFound
	} else if err != nil {
		return PaymentResponse{}, err
	}
	if order.ParticipantID != participantID {
		return PaymentResponse{}, ErrPaymentNotFound // ownership → 404
	}
	if order.Status != OrderPendingPayment {
		return PaymentResponse{}, ErrOrderNotPayable
	}
	if _, err := s.repo.GetActivePaymentByOrder(ctx, orderID); err == nil {
		return PaymentResponse{}, ErrPaymentActive
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return PaymentResponse{}, err
	}

	now := time.Now()
	ref, err := generateMerchantReference(now)
	if err != nil {
		return PaymentResponse{}, ErrMerchantRefGen
	}
	expiresAt := now.Add(s.expiry)
	if order.ExpiredAt.Valid && order.ExpiredAt.Time.Before(expiresAt) {
		expiresAt = order.ExpiredAt.Time // clamp ke order expiry
	}

	charge, err := g.CreateCharge(ctx, gw.CreateChargeInput{
		MerchantReference: ref, Amount: order.Total, Method: req.Method, Channel: req.Channel,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return PaymentResponse{}, ErrGatewayError
	}

	payment, err := s.repo.CreatePayment(ctx, db.CreatePaymentParams{
		OrganizationID: order.OrganizationID, EventID: order.EventID, OrderID: order.ID,
		ParticipantID: participantID, Gateway: req.Gateway, Method: req.Method,
		Channel: pgtypeText(req.Channel), Status: StatusPending, Amount: order.Total,
		Currency: "IDR", GatewayReference: pgtypeText(charge.GatewayReference),
		MerchantReference: ref, PayUrl: pgtypeText(charge.PayURL),
		QrString: pgtypeText(charge.QRString), VaNumber: pgtypeText(charge.VANumber),
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return PaymentResponse{}, err
	}

	s.recordCreated(ctx, payment)
	return toResponse(payment), nil
}

func (s *Service) GetForParticipant(ctx context.Context, participantID, paymentID uuid.UUID) (PaymentResponse, error) {
	pay, err := s.repo.GetPaymentByID(ctx, paymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return PaymentResponse{}, ErrPaymentNotFound
	} else if err != nil {
		return PaymentResponse{}, err
	}
	if pay.ParticipantID != participantID {
		return PaymentResponse{}, ErrPaymentNotFound
	}
	return toResponse(pay), nil
}

func (s *Service) ListByOrder(ctx context.Context, participantID, orderID uuid.UUID) ([]PaymentResponse, error) {
	rows, err := s.repo.ListPaymentsByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	out := make([]PaymentResponse, 0, len(rows))
	for _, p := range rows {
		if p.ParticipantID == participantID {
			out = append(out, toResponse(p))
		}
	}
	return out, nil
}

func (s *Service) ListForOrgEvent(ctx context.Context, orgID, eventID uuid.UUID) ([]PaymentResponse, error) {
	rows, err := s.repo.ListPaymentsByOrgEvent(ctx, db.ListPaymentsByOrgEventParams{
		OrganizationID: orgID, EventID: eventID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]PaymentResponse, 0, len(rows))
	for _, p := range rows {
		out = append(out, toResponse(p))
	}
	return out, nil
}

func (s *Service) recordCreated(ctx context.Context, p db.Payment) {
	if s.audit == nil {
		return
	}
	oid := p.OrganizationID
	uid := p.ParticipantID
	s.audit.Record(ctx, audit.Entry{
		OrganizationID: &oid, ActorUserID: &uid, Action: "PAYMENT_CREATED",
		TargetType: "payment", TargetID: p.ID.String(),
	})
}
```

Tambahkan `toResponse(db.Payment) PaymentResponse` di `dto.go` atau file mapping; petakan
field nullable (`pay_url`, `qr_string`, `va_number`, `expires_at`, `paid_at`). Import `audit`
di service. Catatan: pemakaian `GetOrderByIDForUpdate` di luar tx hanya membaca baris;
jika ingin guard ketat terhadap balapan create-payment ganda, bungkus seluruh CreatePayment
dalam `ExecTx` (disarankan) — partial unique index `uq_payments_order_active` adalah
pengaman terakhir terhadap dua payment aktif.

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/modules/payments/ -run TestCreatePayment -v; cd ../..
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/payments/service.go services/api/internal/modules/payments/service_test.go services/api/internal/modules/payments/dto.go
git commit -m "feat(payments): add service (create charge, get/list, guards)"
```

---

## Task 11: Reconcile

**Files:**
- Create: `services/api/internal/modules/payments/reconcile.go`
- Test: `services/api/internal/modules/payments/reconcile_test.go`

- [ ] **Step 1: Tulis test reconcile**

Reconcile: ambil payment, `gateway.QueryStatus(gatewayReference)`, lalu jalankan jalur
`processor.apply` yang sama (idempotent dengan callback). Test: payment PENDING + gateway
melaporkan PAID → order jadi PAID (sekali).

```go
package payments

import (
	"context"
	"testing"

	"github.com/google/uuid"

	gwpkg "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

func TestReconcile_PendingToPaid(t *testing.T) {
	repo := newFakeProcRepo()
	oid := uuid.New()
	pid := uuid.New()
	repo.addPayment(db.Payment{ID: pid, OrderID: oid, MerchantReference: "PAY-1", GatewayReference: "ref1", Status: StatusPending, Amount: 100000, Gateway: "duitku"})
	repo.addOrder(db.Order{ID: oid, Status: OrderPendingPayment})

	reg := gwpkg.NewRegistry()
	reg.Register(fakeGateway{name: "duitku"}) // QueryStatus → PAID
	proc := NewProcessor(repo, nil)
	rec := NewReconciler(repo, reg, proc)

	if err := rec.Reconcile(context.Background(), pid); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if repo.orderPaidCount != 1 {
		t.Errorf("order paid %d times, want 1", repo.orderPaidCount)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/payments/ -run TestReconcile -v; cd ../..
```
Expected: FAIL — `undefined: NewReconciler`.

- [ ] **Step 3: Implement reconcile.go**

Create `services/api/internal/modules/payments/reconcile.go`:
```go
package payments

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

type Reconciler struct {
	repo      Repository
	registry  *gw.Registry
	processor *Processor
}

func NewReconciler(repo Repository, registry *gw.Registry, processor *Processor) *Reconciler {
	return &Reconciler{repo: repo, registry: registry, processor: processor}
}

func (r *Reconciler) Reconcile(ctx context.Context, paymentID uuid.UUID) error {
	pay, err := r.repo.GetPaymentByID(ctx, paymentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPaymentNotFound
	} else if err != nil {
		return err
	}
	g, ok := r.registry.Get(pay.Gateway)
	if !ok {
		return ErrGatewayNotAvail
	}
	res, err := g.QueryStatus(ctx, textValue(pay.GatewayReference))
	if err != nil {
		return ErrReconcileFailed
	}
	res.MerchantReference = pay.MerchantReference
	if res.Amount == 0 {
		res.Amount = pay.Amount
	}
	return r.processor.apply(ctx, pay.Gateway, res)
}
```

Tambahkan helper `textValue(pgtype.Text) string` di file mapping bila belum ada.

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/modules/payments/ -run TestReconcile -v; cd ../..
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/payments/reconcile.go services/api/internal/modules/payments/reconcile_test.go
git commit -m "feat(payments): add manual reconcile via gateway query"
```

---

## Task 12: Handler, routes, webhook binary, wiring

**Files:**
- Create: `services/api/internal/modules/payments/handler.go`
- Create: `services/api/internal/modules/payments/routes.go`
- Create: `services/api/internal/modules/payments/webhook/http/server.go`
- Create: `services/api/internal/modules/payments/webhook/http/duitku_handler.go`
- Create: `services/api/internal/modules/payments/webhook/http/xendit_handler.go`
- Create: `services/api/internal/modules/payments/webhook/http/server_test.go`
- Create: `services/api/cmd/webhook/main.go`
- Modify: `services/api/internal/app/server.go`

- [ ] **Step 1: Implement handler.go**

Pola persis handler Phase 5 orders (ambil identity dari `authctx`, parse URL param, panggil
service, `apperr.WriteJSON/WriteError`). Endpoint: `CreatePayment` (POST order payments),
`ListByOrder`, `GetMine`, `ListByOrgEvent`, `Reconcile`.

```go
package payments

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/authctx"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct {
	svc *Service
	rec *Reconciler
}

func NewHandler(svc *Service, rec *Reconciler) *Handler { return &Handler{svc: svc, rec: rec} }

func participant(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := authctx.FromContext(r.Context())
	if !ok {
		apperr.WriteError(w, r, apperr.New(http.StatusUnauthorized, "UNAUTHENTICATED", "identity required"))
		return uuid.Nil, false
	}
	return id.UserID, true
}

func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	uid, ok := participant(w, r)
	if !ok {
		return
	}
	orderID, err := uuid.Parse(chi.URLParam(r, "orderId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORDER_ID", "invalid order id"))
		return
	}
	var req CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "invalid request body"))
		return
	}
	resp, err := h.svc.CreatePayment(r.Context(), uid, orderID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, resp)
}

// ListByOrder, GetMine, ListByOrgEvent, Reconcile — pola serupa (lihat orders handler Phase 5).
```

Lengkapi `ListByOrder`, `GetMine`, `ListByOrgEvent`, `Reconcile` mengikuti pola yang sama.

- [ ] **Step 2: Implement routes.go**

```go
package payments

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes: endpoint peserta (authn level).
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/orders/{orderId}/payments", func(r chi.Router) {
		r.Post("/", h.CreatePayment)
		r.Get("/", h.ListByOrder)
	})
	r.Get("/payments/{paymentId}", h.GetMine)
}

// RegisterOrgRoutes: endpoint organizer (dalam scope /organizations/{orgId}).
func (h *Handler) RegisterOrgRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.With(middleware.RequirePermission(loader, "payment.view")).
		Get("/events/{eventId}/payments", h.ListByOrgEvent)
	r.With(middleware.RequirePermission(loader, "payment.manage")).
		Post("/payments/{paymentId}/reconcile", h.Reconcile)
}
```

Verifikasi nama permission (`payment.view`) sesuai katalog Phase 2.

- [ ] **Step 3: Implement webhook receiver + binary**

`webhook/http/server.go`: chi router dengan request-id + recover, route `/healthz`,
`/webhooks/duitku`, `/webhooks/xendit`. Handler: claim dedupe lalu `processor.ProcessRaw`.

```go
package http

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/modules/payments"
	gw "github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway"
)

type Server struct {
	processor *payments.Processor
	registry  *gw.Registry
}

func NewServer(processor *payments.Processor, registry *gw.Registry) *Server {
	return &Server{processor: processor, registry: registry}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Post("/webhooks/duitku", s.handle("duitku"))
	r.Post("/webhooks/xendit", s.handle("xendit"))
	return r
}

func (s *Server) handle(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g, ok := s.registry.Get(name)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := s.processor.ProcessRaw(r.Context(), g, r.Header, body); err != nil {
			// signature invalid → 4xx; lain → 200 agar gateway tak retry badai saat error internal
			if err == payments.ErrInvalidSignature {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		w.WriteHeader(http.StatusOK) // sebagian besar gateway cukup 200
	}
}
```

(Pisah `duitku_handler.go`/`xendit_handler.go` opsional; boleh satu fungsi `handle` seperti di atas.)

`cmd/webhook/main.go`: load config, connect Postgres, build registry dari config, build
processor (`payments.NewProcessor(payments.NewRepository(pool), auditLogger)`), serve di
`WEBHOOK_PORT`. Pola main mengikuti `cmd/api` & `cmd/worker`.

- [ ] **Step 4: Tulis test webhook receiver**

`server_test.go`: POST `/webhooks/duitku` dengan body valid (signature benar via fake gateway)
→ 200, processor terpanggil. Signature invalid → 401. Gunakan fake gateway + fake repo.

- [ ] **Step 5: Wire payments handler ke server.go (API utama)**

Tambah build di `NewRouter`/server assembly (pola Phase 5):
```go
paymentsRepo := payments.NewRepository(pool)
paymentsRegistry := gateway.BuildRegistry(cfg) // helper baru dari config (Task 13)
paymentsProc := payments.NewProcessor(paymentsRepo, auditLogger)
paymentsSvc := payments.NewService(paymentsRepo, paymentsRegistry, auditLogger, cfg.PaymentDefaultExpiry)
paymentsReconciler := payments.NewReconciler(paymentsRepo, paymentsRegistry, paymentsProc)
paymentsHandler := payments.NewHandler(paymentsSvc, paymentsReconciler)
```
Mount: `paymentsHandler.RegisterRoutes(r)` di level authn; `paymentsHandler.RegisterOrgRoutes(r, loader)`
di dalam `r.Route("/organizations/{orgId}", ...)`. JANGAN ubah wiring Phase 1-5 yang ada.

- [ ] **Step 6: Verify build + tests**

Run:
```bash
cd services/api && go build ./... && go test ./internal/modules/payments/... && cd ../..
```
Expected: build OK; tests PASS.

- [ ] **Step 7: Commit**

```bash
git add services/api/internal/modules/payments/handler.go services/api/internal/modules/payments/routes.go services/api/internal/modules/payments/webhook/ services/api/cmd/webhook/ services/api/internal/app/server.go
git commit -m "feat(payments): add handler, routes, webhook receiver binary, wiring"
```

---

## Task 13: Gateway registry builder from config + Makefile target

**Files:**
- Create: `services/api/internal/modules/payments/gateway/build.go`
- Modify: `Makefile`

- [ ] **Step 1: Implement BuildRegistry**

Create `services/api/internal/modules/payments/gateway/build.go`:
```go
package gateway

import (
	"github.com/varin/ivyticketing/services/api/internal/app"
	"github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway/duitku"
	"github.com/varin/ivyticketing/services/api/internal/modules/payments/gateway/xendit"
)

// BuildRegistry membangun registry dari config; hanya gateway ENABLED + kredensial lengkap.
func BuildRegistry(cfg app.Config) *Registry {
	r := NewRegistry()
	if cfg.DuitkuEnabled {
		r.Register(duitku.New(duitku.Config{
			MerchantCode: cfg.DuitkuMerchantCode, APIKey: cfg.DuitkuAPIKey, Env: cfg.DuitkuEnv,
		}))
	}
	if cfg.XenditEnabled {
		r.Register(xendit.New(xendit.Config{
			SecretKey: cfg.XenditSecretKey, CallbackToken: cfg.XenditCallbackToken, Env: cfg.XenditEnv,
		}))
	}
	return r
}
```

Catatan: jika import `app` dari package `gateway` menimbulkan import cycle, pindahkan
`BuildRegistry` ke package `payments` atau `app` (mana yang tidak menimbulkan siklus).
Cek dengan `go build`; sesuaikan lokasi.

- [ ] **Step 2: Tambah Makefile target webhook**

Tambah ke `Makefile`:
```make
webhook:
	cd services/api && go run ./cmd/webhook
```

- [ ] **Step 3: Verify build**

Run:
```bash
cd services/api && go build ./... && cd ../..
```
Expected: build OK (atau pindahkan BuildRegistry bila ada cycle).

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/modules/payments/gateway/build.go Makefile
git commit -m "feat(payments): build gateway registry from config + makefile webhook target"
```

---

## Task 14: Integration + concurrency tests, docs, DoD verification

**Files:**
- Create: `services/api/tests/integration/payments_test.go`
- Create: `services/api/tests/integration/payment_concurrency_test.go`
- Create: docs (lihat File Structure)
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Integration — happy path + duplicate + invalid signature**

Pakai test DB `ivyticketing_test` + helper integration Phase 5. Skenario:
- register → login → buat org → event(+publish) → kategori → checkout (PENDING_PAYMENT) →
  create payment → simulasi callback PAID (panggil `processor.ProcessRaw` dengan body bertanda
  tangan valid via gateway sandbox-fake) → assert order PAID, reservasi COMPLETED, payment PAID,
  webhook PROCESSED.
- duplicate callback (2x) → order PAID sekali, webhook ke-2 DUPLICATE.
- invalid signature → REJECTED, order tetap PENDING_PAYMENT.
- expired-then-paid: expire order via worker Phase 5, lalu callback PAID → payment PAID,
  order tetap EXPIRED, webhook PROCESSED + error_detail.
- ownership: payment user A → 404 untuk user B.
- organizer list/reconcile butuh `payment.view`/`payment.manage`.

- [ ] **Step 2: Concurrency — satu payment, banyak callback identik**

Create `payment_concurrency_test.go`: 50 goroutine memanggil `processor.apply` dengan
`CallbackResult` PAID yang sama untuk satu payment → assert: order PAID tepat sekali,
reservasi COMPLETED sekali (query DB), tidak ada error fatal. Jalankan dengan `-race`.

Run:
```bash
cd services/api && go test ./tests/integration/ -run TestPayment -race -v; cd ../..
```
Expected: PASS, tidak ada data race.

- [ ] **Step 3: Tulis dokumentasi**

Buat 5 file utama + 3 file `docs/payment/` (lihat spec §Documentation). Tiap file: sequence
diagram teks, state machine, failure/recovery. Minimal lengkap, tanpa placeholder.

- [ ] **Step 4: Update CHANGELOG**

Tambah entri Phase 6 di `CHANGELOG.md` (payments module, gateway Duitku+Xendit, webhook binary,
idempotency, reconcile).

- [ ] **Step 5: Full verification (DoD)**

Run:
```bash
cd services/api && go vet ./... && go test ./... && cd ../..
make migrate-down && make migrate-up
make sqlc
```
Expected: vet bersih; semua test (unit + integration + concurrency `-race`) PASS; migrasi
roundtrip OK; sqlc generate bersih (no diff tak terduga).

Checklist DoD (spec §Definition of Done) — verifikasi tiap poin 1-16 terpenuhi.

- [ ] **Step 6: Commit**

```bash
git add services/api/tests/integration/ docs/ CHANGELOG.md
git commit -m "test(payments): integration + concurrency tests; docs; phase6 DoD"
```

---

## Self-Review Notes (untuk implementer)

- **Verifikasi nama sqlc Phase 5** sebelum Task 8/9: `UpdateOrderStatusParams.Status_2`,
  `UpdateReservationStatusByOrderParams`, `GetOrderByIDForUpdate`. Sesuaikan bila beda — JANGAN
  ubah query Phase 5, tambahkan query baru bila perlu.
- **Idempotency berlapis**: dedupe_key (cegah proses ulang) + `MarkPaymentPaid` guard
  `status='PENDING'` (cegah double-transition walau dedupe lolos). Keduanya wajib.
- **Signature/secret tidak boleh ter-log** — hanya metadata (gateway, ref, status).
- **CreateCharge/QueryStatus adapter** sengaja "not yet implemented" di plan; isi HTTP call
  nyata + uji dengan `http.Client` fake saat kredensial sandbox tersedia. Jalur callback
  (signature + parse + processor) — yang paling kritikal — diuji penuh sekarang.
- **Import cycle**: `BuildRegistry` mungkin perlu pindah package (app vs payments). Putuskan
  saat `go build`.
