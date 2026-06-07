# Phase 3 Plan — Part 4: Integration, Verification, Docs (Tasks 12-14)

> Part of the Phase 3 implementation plan. Index: [2026-06-07-phase3-event-category-management.md](2026-06-07-phase3-event-category-management.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **Depends on:** Parts 1-3 (all modules wired). The Phase 2 integration helpers exist at `services/api/tests/integration/helpers_test.go`.

---

## Task 12: Integration tests (real Postgres)

These reuse the Phase 2 integration harness (`testPool`, `truncate`, `newTestServer`, `postJSON`). The `truncate` helper must be extended to clear the new tables.

**Files:**
- Modify: `services/api/tests/integration/helpers_test.go`
- Create: `services/api/tests/integration/event_flow_test.go`
- Create: `services/api/tests/integration/event_tenant_test.go`
- Create: `services/api/tests/integration/media_upload_test.go`

- [ ] **Step 1: Extend truncate for new tables**

In `services/api/tests/integration/helpers_test.go`, update the `truncate` SQL to also clear events/categories (add BEFORE the `organizations`/`users` deletes, since they FK to organizations):
```go
	_, err := pool.Exec(context.Background(), `
		DELETE FROM event_categories;
		DELETE FROM events;
		DELETE FROM member_roles;
		DELETE FROM organization_members;
		DELETE FROM audit_logs;
		DELETE FROM refresh_tokens;
		DELETE FROM role_permissions WHERE role_id IN (SELECT id FROM roles WHERE organization_id IS NOT NULL);
		DELETE FROM roles WHERE organization_id IS NOT NULL;
		DELETE FROM organizations;
		DELETE FROM users;
	`)
```
Note: confirm the exact current content of `truncate` from Phase 2 and merge — keep the existing deletes, prepend the two event ones. Order matters: `event_categories` → `events` → ... → `organizations`.

- [ ] **Step 2: Verify test DB has the new migrations**

Run:
```bash
make test-db-setup
```
Expected: migrations 00008/00009 apply to `ivyticketing_test` (goose reports up-to-date or applies them).

- [ ] **Step 3: Full event flow test**

Create `services/api/tests/integration/event_flow_test.go`:
```go
//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// helper from Phase 2: postJSON(t, client, url, body, bearer) *http.Response
// helper from this file below: loginCreateOrg returns (token, orgID, orgSlug)

func loginCreateOrg(t *testing.T, client *http.Client, baseURL, email, orgName string) (token, orgID, orgSlug string) {
	t.Helper()
	postJSON(t, client, baseURL+"/api/v1/auth/register",
		map[string]string{"email": email, "password": "pw123456", "fullName": email}, "").Body.Close()
	resp := postJSON(t, client, baseURL+"/api/v1/auth/login",
		map[string]string{"email": email, "password": "pw123456"}, "")
	var login struct{ AccessToken string `json:"accessToken"` }
	json.NewDecoder(resp.Body).Decode(&login)
	resp.Body.Close()

	resp = postJSON(t, client, baseURL+"/api/v1/organizations",
		map[string]string{"name": orgName}, login.AccessToken)
	var org struct{ ID, Slug string }
	json.NewDecoder(resp.Body).Decode(&org)
	resp.Body.Close()
	return login.AccessToken, org.ID, org.Slug
}

func TestEventFlow_CreateCategoryPublishPublic(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	token, orgID, orgSlug := loginCreateOrg(t, client, srv.URL, "owner@x.com", "Jakarta Marathon Org")

	// Create event.
	resp := postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events",
		map[string]any{"name": "Jakarta Marathon 2026", "eventType": "marathon"}, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create event = %d, want 201", resp.StatusCode)
	}
	var ev struct{ ID, Slug, Status string }
	json.NewDecoder(resp.Body).Decode(&ev)
	resp.Body.Close()
	if ev.Status != "draft" {
		t.Errorf("status = %q, want draft", ev.Status)
	}

	// Publish without categories → 409.
	resp = postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+ev.ID+"/publish", nil, token)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("publish-no-cats = %d, want 409", resp.StatusCode)
	}
	resp.Body.Close()

	// Add a category.
	resp = postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+ev.ID+"/categories",
		map[string]any{
			"name": "42K", "price": 350000, "capacity": 2000,
			"registrationOpensAt":  time.Now().Format(time.RFC3339),
			"registrationClosesAt": time.Now().Add(720 * time.Hour).Format(time.RFC3339),
			"maxOrderPerUser":      1,
		}, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create category = %d, want 201", resp.StatusCode)
	}
	resp.Body.Close()

	// Publish now succeeds.
	resp = postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+ev.ID+"/publish", nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("publish = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Public list shows it.
	resp, _ = client.Get(srv.URL + "/api/v1/public/organizations/" + orgSlug + "/events")
	var list []struct{ Slug string }
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list) != 1 || list[0].Slug != ev.Slug {
		t.Fatalf("public list = %+v, want 1 event %q", list, ev.Slug)
	}

	// Public detail includes the category.
	resp, _ = client.Get(srv.URL + "/api/v1/public/organizations/" + orgSlug + "/events/" + ev.Slug)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("public detail = %d, want 200", resp.StatusCode)
	}
	var detail struct {
		Categories []struct{ Name string } `json:"categories"`
	}
	json.NewDecoder(resp.Body).Decode(&detail)
	resp.Body.Close()
	if len(detail.Categories) != 1 || detail.Categories[0].Name != "42K" {
		t.Errorf("public detail categories = %+v", detail.Categories)
	}

	// Unpublish → public detail 404.
	resp = postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+ev.ID+"/unpublish", nil, token)
	resp.Body.Close()
	resp, _ = client.Get(srv.URL + "/api/v1/public/organizations/" + orgSlug + "/events/" + ev.Slug)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("public after unpublish = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
}
```

- [ ] **Step 4: Tenant isolation test**

Create `services/api/tests/integration/event_tenant_test.go`:
```go
//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestEventTenantIsolation(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	tokenA, orgA, _ := loginCreateOrg(t, client, srv.URL, "a@x.com", "Org A")
	tokenB, orgB, _ := loginCreateOrg(t, client, srv.URL, "b@x.com", "Org B")

	// A creates an event.
	resp := postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgA+"/events",
		map[string]any{"name": "A Event", "eventType": "marathon"}, tokenA)
	var ev struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&ev)
	resp.Body.Close()

	// B tries to read A's event via B's org path → 404 (event not in org B).
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgB+"/events/"+ev.ID, nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-tenant event read = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()

	// B tries via A's org path → 403 (not a member of org A).
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgA+"/events/"+ev.ID, nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-org access = %d, want 403", resp.StatusCode)
	}
	resp.Body.Close()
}
```

- [ ] **Step 5: Local media upload test**

Create `services/api/tests/integration/media_upload_test.go`:
```go
//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"testing"
)

func TestMediaUpload_LocalDirectFlow(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	token, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner@x.com", "Media Org")

	resp := postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events",
		map[string]any{"name": "Media Event", "eventType": "marathon"}, token)
	var ev struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&ev)
	resp.Body.Close()

	// 1. Request ticket for banner.
	resp = postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+ev.ID+"/media/banner",
		map[string]string{"contentType": "image/png", "fileName": "b.png"}, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ticket = %d, want 200", resp.StatusCode)
	}
	var ticket struct {
		Mode      string `json:"mode"`
		ObjectKey string `json:"objectKey"`
		UploadURL string `json:"uploadUrl"`
	}
	json.NewDecoder(resp.Body).Decode(&ticket)
	resp.Body.Close()
	if ticket.Mode != "direct" {
		t.Fatalf("mode = %q, want direct (local)", ticket.Mode)
	}

	// 2. Multipart upload to the local sink.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "b.png")
	fw.Write([]byte("\x89PNG\r\n\x1a\nfakeimage"))
	mw.Close()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+ticket.UploadURL, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("upload = %d, want 204", resp.StatusCode)
	}
	resp.Body.Close()

	// 3. Confirm.
	body, _ := json.Marshal(map[string]string{"objectKey": ticket.ObjectKey})
	req, _ = http.NewRequest(http.MethodPut, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+ev.ID+"/media/banner/confirm", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("confirm = %d, want 200", resp.StatusCode)
	}
	var confirmed struct{ BannerURL string `json:"bannerUrl"` }
	json.NewDecoder(resp.Body).Decode(&confirmed)
	resp.Body.Close()
	if confirmed.BannerURL == "" {
		t.Fatal("bannerUrl should be set after confirm")
	}

	// 4. The file is served at /media/{key}.
	resp, _ = client.Get(srv.URL + "/media/" + ticket.ObjectKey)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("serve media = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// 5. Confirm rejects a tampered key (different event prefix).
	body, _ = json.Marshal(map[string]string{"objectKey": "org/" + orgID + "/event/00000000-0000-0000-0000-000000000000/banner/x.png"})
	req, _ = http.NewRequest(http.MethodPut, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+ev.ID+"/media/banner/confirm", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("tampered confirm = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}
```
Note: `newTestServer` (from Phase 2 helpers) builds `app.Config` — ensure it sets `StorageDriver: "local"`, `StorageLocalPath: t.TempDir()`, `StoragePublicBaseURL: srv.URL` (or a fixed localhost), and `StorageUploadMaxBytes: 5242880`. **You must update `newTestServer` in `helpers_test.go` to populate these new Config fields**, otherwise `storage.New` may build with an empty local path. Set `StorageLocalPath` to a temp dir created per-server; since the media-serve route reads from `cfg.StorageLocalPath`, the same path must be used. Use a package-level temp dir or set it in the helper and return it if needed.

- [ ] **Step 6: Run integration tests**

Run:
```bash
make test-integration
```
Expected: all Phase 2 + Phase 3 integration tests PASS. If `newTestServer` needs the storage fields, update it first (Step 5 note). If the test DB seed/templates were wiped by truncate in a prior run, re-seed: `goose ... down-to 6 && goose ... up` against `ivyticketing_test` (note: with new migrations, re-seed RBAC by `down-to 7 && up` — adjust so migration 00007 seed re-runs; simplest is `down-to 6 && up`).

- [ ] **Step 7: Commit**

```bash
git add services/api/tests/integration
git commit -m "test(api): add phase 3 integration tests (event flow, tenant, media)"
```

---

## Task 13: Full Definition-of-Done verification

- [ ] **Step 1: Run the complete gate**

Run:
```bash
# DoD #1 migrations roundtrip
make migrate-down && make migrate-up
# DoD #10/#11 unit tests + sqlc clean + build
make sqlc && cd services/api && go build ./... && go vet ./... && go test ./... && cd ../..
# DoD #2-#9 integration
make test-db-setup && make test-integration
# DoD #12 no hardcoded secrets
grep -rn "STORAGE_SECRET_KEY\|STORAGE_ACCESS_KEY" services/api/internal | grep -v "os.Getenv\|cfg.Storage" || echo "no hardcoded storage secrets"
```
Expected: every command succeeds. Mapping to spec DoD:
- #1 migrations up/down → `make migrate-down/up`
- #2 event CRUD + lifecycle → unit (events/service_test) + integration (event_flow)
- #3 category CRUD + validation → unit (categories/service_test)
- #4 storage interface + local driver; S3 stub → unit (storage/local_test) + `s3.go` contract
- #5 media upload end-to-end → integration (media_upload)
- #6 public shows only published → integration (event_flow public list/detail, unpublish→404)
- #7 tenant isolation → integration (event_tenant)
- #8 publish without categories rejected → integration (event_flow 409)
- #9 audit on sensitive actions → events service `record()` wiring
- #10 `go test ./...` green → final gate
- #11 sqlc clean → `make sqlc`
- #12 no hardcoded secrets → grep gate

- [ ] **Step 2: Confirm everything green**

If any step fails, fix the root cause before proceeding. Do not weaken a test to make it pass.

---

## Task 14: README + CHANGELOG

**Files:**
- Modify: `README.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Document Phase 3 in README**

Add a "Phase 3 — Event & Category Management" section to `README.md` (above the Phase 2 section, newest-first), covering: new storage env vars, event/category endpoints, the media upload flow (request ticket → upload/presign → confirm), and the public catalog endpoints. Include a short curl smoke test:
```markdown
## Phase 3 — Event & Category Management

Event CRUD with draft/published/archived lifecycle, categories, pluggable storage
(local now; R2/Tencent via presigned URL later), and a public read-only catalog.

### New env

```bash
STORAGE_DRIVER=local                       # local | r2 | tencent | s3
STORAGE_LOCAL_PATH=./var/media
STORAGE_PUBLIC_BASE_URL=http://localhost:8080
STORAGE_UPLOAD_MAX_BYTES=5242880
# cloud (when used): STORAGE_BUCKET, STORAGE_ENDPOINT, STORAGE_ACCESS_KEY, STORAGE_SECRET_KEY, STORAGE_REGION
```

### Smoke test

```bash
# create event (needs event.create; use an Owner access token from Phase 2 login)
curl -s -X POST localhost:8080/api/v1/organizations/<orgId>/events \
  -H "authorization: Bearer <accessToken>" -H 'content-type: application/json' \
  -d '{"name":"Jakarta Marathon 2026","eventType":"marathon"}'

# add a category
curl -s -X POST localhost:8080/api/v1/organizations/<orgId>/events/<eventId>/categories \
  -H "authorization: Bearer <accessToken>" -H 'content-type: application/json' \
  -d '{"name":"42K","price":350000,"capacity":2000,"registrationOpensAt":"2026-07-01T00:00:00Z","registrationClosesAt":"2026-08-01T00:00:00Z","maxOrderPerUser":1}'

# publish, then view publicly
curl -s -X POST localhost:8080/api/v1/organizations/<orgId>/events/<eventId>/publish \
  -H "authorization: Bearer <accessToken>"
curl -s localhost:8080/api/v1/public/organizations/<orgSlug>/events
```
```

- [ ] **Step 2: Add CHANGELOG entry**

Prepend a Phase 3 section to `CHANGELOG.md` (newest-first, above Phase 2):
```markdown
## [Phase 3] — 2026-06-07

Event & category management. Backend-only.

### Added

**Events**
- CRUD: `POST/GET/PUT/DELETE /api/v1/organizations/:orgId/events[/:eventId]`
- Lifecycle: `publish` (rejects if no categories), `unpublish`, `archive`
- Status: draft → published → archived
- Auto slug from name (unique per org)
- Audit logging on publish/unpublish/archive/delete

**Categories**
- CRUD: `.../events/:eventId/categories[/:categoryId]`
- Fields: price (minor units), capacity, registration window, bib prefix, min age, max order per user
- Validation: price ≥ 0, capacity > 0, opens < closes, max order ≥ 1
- No inventory/stock logic yet (Phase 5) — capacity is a stored number

**Media**
- Pluggable `Storage` interface: full `local` disk driver; S3-compatible (R2/Tencent) stub with presigned-upload contract
- Upload flow: request ticket → (cloud: presigned PUT direct-to-storage; local: multipart to API) → confirm
- Object keys namespaced per tenant (`org/{orgId}/event/{eventId}/{kind}/`), confirm validates prefix (anti-tamper)
- Local media served at `/media/{key}`

**Public catalog** (no auth)
- `GET /api/v1/public/organizations/:orgSlug/events` — published only
- `GET /api/v1/public/organizations/:orgSlug/events/:eventSlug` — detail + categories

**Database** (goose migrations 00008–00009)
- Tables: `events`, `event_categories`

**Config**
- `STORAGE_DRIVER`, `STORAGE_LOCAL_PATH`, `STORAGE_PUBLIC_BASE_URL`, `STORAGE_UPLOAD_MAX_BYTES`, and cloud credential vars

**Tests**
- Unit: events service (lifecycle, tenant guard), categories service (validation), storage local driver, media key validation
- Integration: full event→category→publish→public flow, tenant isolation (404/403), local media upload end-to-end
```

- [ ] **Step 3: Commit**

```bash
git add README.md CHANGELOG.md
git commit -m "docs: document phase 3 event/category management and update changelog"
```

---

## Done

Phase 3 complete. All 14 tasks across 4 parts: storage config + migrations + storage drivers + queries (Part 1), events module (Part 2), categories + media + public catalog (Part 3), integration + verification + docs (Part 4).
