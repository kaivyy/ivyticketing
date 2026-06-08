# Phase 11 Part 4: Corporate Module — Accounts, Bulk Upload, Invoice, Approval

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the full corporate registration flow: create/approve corporate accounts, bulk member upload (CSV), access code issuance per member, invoice JSON generation, and the organizer corporate management frontend.

**Architecture:** `CorporateService` (already in Part 1) gains HTTP endpoints via `access.Handler`. A corporate account owns a CORPORATE `AccessPool`. Bulk upload is transactional — all members inserted or none. Each member gets an `access_code` issued to their email. Invoice is JSON (PDF deferred). Approval flow: PENDING → ACTIVE via organizer action.

**Tech Stack:** Go 1.25, Chi v5, pgx v5, encoding/csv. Module: `github.com/varin/ivyticketing/services/api`.

---

### Task 1: Corporate HTTP Endpoints

**Files:**
- Modify: `services/api/internal/modules/access/handler.go`
- Modify: `services/api/internal/modules/access/routes.go`
- Modify: `services/api/internal/modules/access/dto.go`

- [ ] **Step 1: Add corporate DTOs to dto.go**

```go
type CreateCorporateAccountRequest struct {
	Name            string `json:"name"`
	BillingEmail    string `json:"billingEmail"`
	InvoiceRequired bool   `json:"invoiceRequired"`
}

type CorporateAccountDTO struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	BillingEmail string  `json:"billingEmail"`
	Status       string  `json:"status"`
	ApprovedAt   *string `json:"approvedAt,omitempty"`
}

type BulkUploadResultDTO struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
}

type MemberDTO struct {
	ID           string  `json:"id"`
	Email        string  `json:"email"`
	MemberStatus string  `json:"memberStatus"`
	RegisteredAt *string `json:"registeredAt,omitempty"`
}
```

- [ ] **Step 2: Add corporate handlers to handler.go**

```go
func (h *Handler) CreateCorporateAccount(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context())
	if !ok { apperr.WriteError(w, r, apperr.New(401, "UNAUTHENTICATED", "not authenticated")); return }
	orgID, _ := uuid.Parse(chi.URLParam(r, "orgId"))
	var req CreateCorporateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(400, "INVALID_BODY", "invalid request")); return
	}
	account, err := h.corporate.Create(r.Context(), orgID, req.Name, req.BillingEmail, req.InvoiceRequired, actor.UserID)
	if err != nil { apperr.WriteError(w, r, err); return }
	apperr.WriteJSON(w, http.StatusCreated, CorporateAccountDTO{
		ID: account.ID.String(), Name: account.Name,
		BillingEmail: account.BillingEmail, Status: account.Status,
	})
}

func (h *Handler) ListCorporateAccounts(w http.ResponseWriter, r *http.Request) {
	orgID, _ := uuid.Parse(chi.URLParam(r, "orgId"))
	limit := int32(50)
	accounts, err := h.corporate.repo.ListCorporateAccounts(r.Context(), db.ListCorporateAccountsParams{
		OrganizationID: orgID, Limit: limit, Offset: 0,
	})
	if err != nil { apperr.WriteError(w, r, err); return }
	apperr.WriteJSON(w, http.StatusOK, accounts)
}

func (h *Handler) ApproveCorporateAccount(w http.ResponseWriter, r *http.Request) {
	actor, ok := authctx.FromContext(r.Context())
	if !ok { apperr.WriteError(w, r, apperr.New(401, "UNAUTHENTICATED", "not authenticated")); return }
	accountID, _ := uuid.Parse(chi.URLParam(r, "accountId"))
	if err := h.corporate.Approve(r.Context(), accountID, actor.UserID); err != nil {
		apperr.WriteError(w, r, err); return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) BulkUploadMembers(w http.ResponseWriter, r *http.Request) {
	poolID, _ := uuid.Parse(chi.URLParam(r, "poolId"))
	actor, ok := authctx.FromContext(r.Context())
	if !ok { apperr.WriteError(w, r, apperr.New(401, "UNAUTHENTICATED", "not authenticated")); return }
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		apperr.WriteError(w, r, apperr.New(400, "INVALID_MULTIPART", "expected multipart form")); return
	}
	file, _, err := r.FormFile("file")
	if err != nil { apperr.WriteError(w, r, apperr.New(400, "MISSING_FILE", "file field required")); return }
	defer file.Close()
	result, err := h.corporate.BulkUploadMembers(r.Context(), poolID, actor.UserID, file)
	if err != nil { apperr.WriteError(w, r, err); return }
	apperr.WriteJSON(w, http.StatusOK, BulkUploadResultDTO{Imported: result.Imported, Skipped: result.Skipped})
}

func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	poolID, _ := uuid.Parse(chi.URLParam(r, "poolId"))
	limit := int32(100)
	members, err := h.corporate.repo.ListPoolMembers(r.Context(), db.ListPoolMembersParams{
		PoolID: poolID, Limit: limit, Offset: 0,
	})
	if err != nil { apperr.WriteError(w, r, err); return }
	apperr.WriteJSON(w, http.StatusOK, members)
}

func (h *Handler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	accountID, _ := uuid.Parse(chi.URLParam(r, "accountId"))
	eventID, _ := uuid.Parse(r.URL.Query().Get("eventId"))
	unitPrice := int64(150000) // default — read from query param or config
	if v := r.URL.Query().Get("unitPrice"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil { unitPrice = n }
	}
	invoice, err := h.corporate.GenerateInvoice(r.Context(), accountID, eventID, unitPrice)
	if err != nil { apperr.WriteError(w, r, err); return }
	apperr.WriteJSON(w, http.StatusOK, invoice)
}
```

- [ ] **Step 3: Add corporate routes to routes.go**

```go
// In RegisterOrganizerRoutes, inside r.Route("/org/{orgId}", ...):
r.Post("/access/corporate", h.CreateCorporateAccount)
r.Get("/access/corporate", h.ListCorporateAccounts)
r.Post("/access/corporate/{accountId}/approve", h.ApproveCorporateAccount)
r.Post("/access/pools/{poolId}/members", h.BulkUploadMembers)
r.Get("/access/pools/{poolId}/members", h.ListMembers)
r.Get("/access/corporate/{accountId}/invoice", h.GetInvoice)
```

- [ ] **Step 4: Build**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go build ./internal/modules/access/... 2>&1
```

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/access/dto.go \
        services/api/internal/modules/access/handler.go \
        services/api/internal/modules/access/routes.go
git commit -m "feat(phase11): corporate HTTP endpoints (create, approve, bulk upload, invoice)"
```

---

### Task 2: Corporate Bulk Upload — Integration Test

**Files:**
- Create: `services/api/internal/modules/access/tests/corporate_integration_test.go`

- [ ] **Step 1: Write integration test (uses build tag `integration`)**

```go
//go:build integration
// +build integration

// services/api/internal/modules/access/tests/corporate_integration_test.go
package access_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/modules/access"
	"github.com/varin/ivyticketing/services/api/testutil"
)

func TestCorporateBulkUpload_10kRows_NoPartialWrite(t *testing.T) {
	pool := testutil.NewTestPool(t)
	repo := access.NewRepository(pool)
	svc := access.NewCorporateService(repo)

	// Create a pool with 10001 slots
	poolID := testutil.CreateTestPool(t, pool, "CORPORATE", 10001)

	// Build 10k row CSV
	var sb strings.Builder
	sb.WriteString("email\n")
	for i := 0; i < 10000; i++ {
		sb.WriteString("user" + strconv.Itoa(i) + "@corp.example.com\n")
	}

	result, err := svc.BulkUploadMembers(context.Background(), poolID, uuid.New(), strings.NewReader(sb.String()))
	if err != nil { t.Fatalf("10k upload should succeed: %v", err) }
	if result.Imported != 10000 { t.Fatalf("want 10000 imported, got %d", result.Imported) }

	// Verify count in DB
	members, _ := repo.ListPoolMembers(context.Background(), db.ListPoolMembersParams{PoolID: poolID, Limit: 10001})
	if len(members) != 10000 { t.Fatalf("DB has %d members, want 10000", len(members)) }
}

func TestCorporateBulkUpload_ExceedsQuota_RejectsAll(t *testing.T) {
	pool := testutil.NewTestPool(t)
	repo := access.NewRepository(pool)
	svc := access.NewCorporateService(repo)

	poolID := testutil.CreateTestPool(t, pool, "CORPORATE", 2) // only 2 slots

	csv := "email\na@x.com\nb@x.com\nc@x.com\n" // 3 rows > 2 slots
	_, err := svc.BulkUploadMembers(context.Background(), poolID, uuid.New(), strings.NewReader(csv))
	if err == nil { t.Fatal("should reject upload exceeding quota") }

	// Verify no members were inserted (transactional reject)
	members, _ := repo.ListPoolMembers(context.Background(), db.ListPoolMembersParams{PoolID: poolID, Limit: 10})
	if len(members) != 0 { t.Fatalf("no members should be inserted on rejection, got %d", len(members)) }
}
```

- [ ] **Step 2: Run integration test**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api
go test ./internal/modules/access/tests/ -tags integration -run TestCorporateBulk -v 2>&1
```

- [ ] **Step 3: Commit**

```bash
git add services/api/internal/modules/access/tests/corporate_integration_test.go
git commit -m "test(phase11): corporate bulk upload integration tests (10k CSV, quota rejection)"
```

---

### Task 3: Frontend — Corporate Management Tab

**Files:**
- Create: `apps/web/src/pages/org/[orgId]/events/[eventId]/corporate.astro`
- Create: `apps/web/src/lib/corporate.ts`

- [ ] **Step 1: Write corporate.ts**

```typescript
const API_URL = import.meta.env.PUBLIC_API_URL ?? "http://localhost:8080"

export async function createCorporateAccount(
  orgId: string,
  data: { name: string; billingEmail: string; invoiceRequired: boolean },
  token: string
) {
  const res = await fetch(`${API_URL}/api/v1/org/${orgId}/access/corporate`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function approveCorporateAccount(orgId: string, accountId: string, token: string) {
  const res = await fetch(`${API_URL}/api/v1/org/${orgId}/access/corporate/${accountId}/approve`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!res.ok) throw new Error(await res.text())
}

export async function bulkUploadMembers(orgId: string, poolId: string, file: File, token: string) {
  const form = new FormData()
  form.append("file", file)
  const res = await fetch(`${API_URL}/api/v1/org/${orgId}/access/pools/${poolId}/members`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: form,
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json() as Promise<{ imported: number; skipped: number }>
}

export async function getInvoice(orgId: string, accountId: string, eventId: string, token: string) {
  const res = await fetch(
    `${API_URL}/api/v1/org/${orgId}/access/corporate/${accountId}/invoice?eventId=${eventId}`,
    { headers: { Authorization: `Bearer ${token}` } }
  )
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}
```

- [ ] **Step 2: Write corporate.astro page**

```astro
---
// apps/web/src/pages/org/[orgId]/events/[eventId]/corporate.astro
const { orgId, eventId } = Astro.params
---

<html lang="en">
<head><title>Corporate Registration</title></head>
<body class="p-6 max-w-2xl mx-auto">
  <h1 class="text-2xl font-bold mb-6">Corporate Registration</h1>

  <section class="mb-8">
    <h2 class="text-lg font-semibold mb-3">Create Corporate Account</h2>
    <form id="create-corp-form" class="space-y-3">
      <input id="corp-name" placeholder="Company name" class="w-full border rounded px-3 py-2" />
      <input id="corp-email" type="email" placeholder="Billing email" class="w-full border rounded px-3 py-2" />
      <label class="flex items-center gap-2">
        <input id="corp-invoice" type="checkbox" />
        <span>Invoice required</span>
      </label>
      <button type="submit" class="bg-blue-600 text-white px-4 py-2 rounded">Create Account</button>
    </form>
    <p id="create-corp-result" class="mt-2 text-sm text-gray-600"></p>
  </section>

  <section>
    <h2 class="text-lg font-semibold mb-3">Bulk Upload Members</h2>
    <p class="text-sm text-gray-500 mb-2">CSV format: email (required), name (optional)</p>
    <input id="pool-id-input" placeholder="Pool ID" class="w-full border rounded px-3 py-2 mb-2" />
    <input id="csv-file" type="file" accept=".csv" class="w-full mb-2" />
    <button id="upload-btn" class="bg-green-600 text-white px-4 py-2 rounded">Upload Members</button>
    <p id="upload-result" class="mt-2 text-sm text-gray-600"></p>
  </section>
</body>
</html>

<script define:vars={{ orgId, eventId }}>
  import { createCorporateAccount, bulkUploadMembers } from "/src/lib/corporate.ts"

  const token = localStorage.getItem("auth_token") ?? ""

  document.getElementById("create-corp-form")?.addEventListener("submit", async (e) => {
    e.preventDefault()
    try {
      const account = await createCorporateAccount(orgId, {
        name: document.getElementById("corp-name").value,
        billingEmail: document.getElementById("corp-email").value,
        invoiceRequired: document.getElementById("corp-invoice").checked,
      }, token)
      document.getElementById("create-corp-result").textContent =
        `Created: ${account.name} (${account.status})`
    } catch (e) {
      document.getElementById("create-corp-result").textContent = `Error: ${e.message}`
    }
  })

  document.getElementById("upload-btn")?.addEventListener("click", async () => {
    const poolId = document.getElementById("pool-id-input").value
    const file = document.getElementById("csv-file").files?.[0]
    if (!file || !poolId) { alert("Pool ID and CSV file required"); return }
    try {
      const result = await bulkUploadMembers(orgId, poolId, file, token)
      document.getElementById("upload-result").textContent =
        `Imported: ${result.imported}, Skipped: ${result.skipped}`
    } catch (e) {
      document.getElementById("upload-result").textContent = `Error: ${e.message}`
    }
  })
</script>
```

- [ ] **Step 3: Build frontend**

```bash
cd /Users/kaivy/Coding/ivyticketing/apps/web && npm run build 2>&1
```

- [ ] **Step 4: Commit**

```bash
git add apps/web/src/pages/org/ apps/web/src/lib/corporate.ts
git commit -m "feat(phase11): corporate management page (create account, bulk upload, invoice)"
```

---

### Task 4: Admin Endpoints — List Codes, Emergency Quota Adjust

**Files:**
- Modify: `services/api/internal/modules/access/handler.go`
- Modify: `services/api/internal/modules/access/routes.go`

- [ ] **Step 1: Add admin handlers**

```go
func (h *Handler) AdminListCodes(w http.ResponseWriter, r *http.Request) {
	limit := int32(100)
	offset := int32(0)
	if v := r.URL.Query().Get("limit"); v != "" { if n, _ := strconv.Atoi(v); n > 0 { limit = int32(n) } }
	if v := r.URL.Query().Get("offset"); v != "" { if n, _ := strconv.Atoi(v); n >= 0 { offset = int32(n) } }
	eventID, _ := uuid.Parse(r.URL.Query().Get("eventId"))
	codes, err := h.codes.repo.ListAccessCodesByEvent(r.Context(),
		db.ListAccessCodesByEventParams{EventID: eventID, Limit: limit, Offset: offset})
	if err != nil { apperr.WriteError(w, r, err); return }
	apperr.WriteJSON(w, http.StatusOK, codes)
}

func (h *Handler) AdminAdjustQuota(w http.ResponseWriter, r *http.Request) {
	poolID, _ := uuid.Parse(chi.URLParam(r, "poolId"))
	var body struct { Delta int `json:"delta"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apperr.WriteError(w, r, apperr.New(400, "INVALID_BODY", "invalid request")); return
	}
	if err := h.pools.AdjustTotalSlots(r.Context(), poolID, body.Delta); err != nil {
		apperr.WriteError(w, r, err); return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: Add admin routes**

```go
func (h *Handler) RegisterAdminRoutes(r chi.Router) {
	r.Route("/admin/access", func(r chi.Router) {
		r.Get("/codes", h.AdminListCodes)
		r.Post("/pools/{poolId}/adjust", h.AdminAdjustQuota)
	})
}
```

Admin routes require `middleware.RequirePlatformAdmin()` (from Phase 9).

- [ ] **Step 3: Wire admin routes in server.go**

```go
r.Group(func(r chi.Router) {
    r.Use(middleware.RequirePlatformAdmin())
    accessHandler.RegisterAdminRoutes(r)
})
```

- [ ] **Step 4: Build + test**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api && go build ./... 2>&1
go test ./... -race 2>&1 | grep -E "^(ok|FAIL)"
```

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/access/handler.go \
        services/api/internal/modules/access/routes.go \
        services/api/internal/app/server.go
git commit -m "feat(phase11): admin endpoints (list codes, emergency quota adjust)"
```

---

### Task 5: Part 4 Full Verification

- [ ] **Step 1**

```bash
cd /Users/kaivy/Coding/ivyticketing/services/api
go build ./... 2>&1
go vet ./... 2>&1
go test ./... -race 2>&1 | grep -E "^(ok|FAIL)"
cd /Users/kaivy/Coding/ivyticketing/apps/web && npm run build 2>&1
```

- [ ] **Step 2: Commit**

```bash
git commit -m "feat(phase11): part 4 complete — corporate module, admin endpoints, corporate frontend"
```
