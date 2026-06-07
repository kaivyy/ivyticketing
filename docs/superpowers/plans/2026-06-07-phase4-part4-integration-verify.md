# Phase 4 Plan — Part 4: Integration, Verification, Docs (Tasks 9-11)

> Part of the Phase 4 implementation plan. Index: [2026-06-07-phase4-form-builder.md](2026-06-07-phase4-form-builder.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **Depends on:** Parts 1-3 (forms module wired). Phase 2/3 integration harness exists at `services/api/tests/integration/`.

---

## Task 9: Integration tests (real Postgres)

Reuse the existing harness (`testPool`, `truncate`, `newTestServer`, `postJSON`,
`loginCreateOrg`). Extend `truncate` for the new tables.

**Files:**
- Modify: `services/api/tests/integration/helpers_test.go`
- Create: `services/api/tests/integration/form_flow_test.go`
- Create: `services/api/tests/integration/form_conditional_test.go`
- Create: `services/api/tests/integration/form_tenant_test.go`

- [ ] **Step 1: Extend truncate**

Read the current `truncate` in `helpers_test.go`. Add `DELETE FROM form_fields;` and
`DELETE FROM form_schemas;` BEFORE the `events`/`organizations` deletes (they FK to events
& organizations). Keep all existing deletes. Correct order (children first):
```
form_fields → form_schemas → event_categories → events → member_roles →
organization_members → audit_logs → refresh_tokens →
role_permissions (org-owned) → roles (org-owned) → organizations → users
```

- [ ] **Step 2: Ensure test DB migrated**

Run:
```bash
make test-db-setup
```
Expected: migrations 00010/00011 apply to `ivyticketing_test`.

- [ ] **Step 3: Helper to create an event (reuse across form tests)**

Add to `helpers_test.go` (only if not already present from Phase 3 — check first):
```go
// createEvent makes a draft event and returns its ID. Requires event.create.
func createEvent(t *testing.T, client *http.Client, baseURL, token, orgID, name string) string {
	t.Helper()
	resp := postJSON(t, client, baseURL+"/api/v1/organizations/"+orgID+"/events",
		map[string]any{"name": name, "eventType": "marathon"}, token)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create event = %d, want 201", resp.StatusCode)
	}
	var ev struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&ev)
	return ev.ID
}
```
Note: if `helpers_test.go` doesn't already import `encoding/json`/`net/http`/`testing`, they're certainly there from Phase 2/3. Don't duplicate.

- [ ] **Step 4: Full form-builder flow test**

Create `services/api/tests/integration/form_flow_test.go`:
```go
//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestFormFlow_AutoCreateAddFieldsReorderPreview(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	token, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner@x.com", "Form Org")
	eventID := createEvent(t, client, srv.URL, token, orgID, "Marathon")

	base := srv.URL + "/api/v1/organizations/" + orgID + "/events/" + eventID + "/form"

	// GET form auto-creates empty.
	req, _ := http.NewRequest(http.MethodGet, base, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := client.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get form = %d, want 200", resp.StatusCode)
	}
	var form struct {
		ID     string `json:"id"`
		Fields []any  `json:"fields"`
	}
	json.NewDecoder(resp.Body).Decode(&form)
	resp.Body.Close()
	if form.ID == "" || len(form.Fields) != 0 {
		t.Fatalf("expected empty auto-created form, got %+v", form)
	}

	// Add text field.
	resp = postJSON(t, client, base+"/fields",
		map[string]any{"fieldType": "text", "label": "Nama", "fieldKey": "nama", "isRequired": true}, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add text field = %d, want 201", resp.StatusCode)
	}
	var f1 struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&f1)
	resp.Body.Close()

	// Add dropdown field with options.
	resp = postJSON(t, client, base+"/fields",
		map[string]any{"fieldType": "dropdown", "label": "Gender", "fieldKey": "gender", "options": []string{"Pria", "Wanita"}}, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add dropdown = %d, want 201", resp.StatusCode)
	}
	var f2 struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&f2)
	resp.Body.Close()

	// Reject dropdown without options.
	resp = postJSON(t, client, base+"/fields",
		map[string]any{"fieldType": "radio", "label": "Bad", "fieldKey": "bad"}, token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("dropdown-no-options = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()

	// Reject duplicate key.
	resp = postJSON(t, client, base+"/fields",
		map[string]any{"fieldType": "email", "label": "Dup", "fieldKey": "nama"}, token)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("dup-key = %d, want 409", resp.StatusCode)
	}
	resp.Body.Close()

	// Reorder: gender first, then nama.
	body, _ := json.Marshal(map[string]any{"fieldIds": []string{f2.ID, f1.ID}})
	req, _ = http.NewRequest(http.MethodPut, base+"/fields/reorder", bytesReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, _ = client.Do(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reorder = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Preview returns visible fields (no category filter → both).
	req, _ = http.NewRequest(http.MethodGet, base+"/preview", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ = client.Do(req)
	var preview []struct{ FieldKey string `json:"fieldKey"` }
	json.NewDecoder(resp.Body).Decode(&preview)
	resp.Body.Close()
	if len(preview) != 2 {
		t.Fatalf("preview fields = %d, want 2", len(preview))
	}
}
```
Note: `bytesReader` — add a tiny helper in `helpers_test.go` if not present: `func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }` (import `bytes`). Or just use `bytes.NewReader(body)` inline and import `bytes` in this test file.

- [ ] **Step 5: Conditional + category scope test**

Create `services/api/tests/integration/form_conditional_test.go`:
```go
//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestFormConditional_ShowHideAndCategoryScope(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	token, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner@x.com", "Cond Org")
	eventID := createEvent(t, client, srv.URL, token, orgID, "Marathon")

	// Add a category so category_scope can reference it.
	resp := postJSON(t, client, srv.URL+"/api/v1/organizations/"+orgID+"/events/"+eventID+"/categories",
		map[string]any{
			"name": "42K", "price": 100000, "capacity": 100,
			"registrationOpensAt":  time.Now().Format(time.RFC3339),
			"registrationClosesAt": time.Now().Add(240 * time.Hour).Format(time.RFC3339),
			"maxOrderPerUser":      1,
		}, token)
	var cat struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&cat)
	resp.Body.Close()

	base := srv.URL + "/api/v1/organizations/" + orgID + "/events/" + eventID + "/form"

	// Auto-create form.
	req, _ := http.NewRequest(http.MethodGet, base, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	client.Do(req)

	// Field A: dropdown "wna".
	resp = postJSON(t, client, base+"/fields",
		map[string]any{"fieldType": "dropdown", "label": "WNA", "fieldKey": "wna", "options": []string{"Ya", "Tidak"}}, token)
	resp.Body.Close()

	// Field B: passport, showIf wna == Ya, required.
	resp = postJSON(t, client, base+"/fields", map[string]any{
		"fieldType": "text", "label": "Passport", "fieldKey": "passport", "isRequired": true,
		"conditional": map[string]any{"field": "wna", "op": "equals", "value": "Ya"},
	}, token)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add conditional field = %d, want 201", resp.StatusCode)
	}
	resp.Body.Close()

	// Validate with wna=Ya → passport visible & required & missing → invalid.
	res := previewValidate(t, client, base, token, map[string]any{"wna": "Ya"})
	if res.Valid {
		t.Fatalf("expected invalid (passport required), got valid")
	}
	if !containsKey(res.VisibleFields, "passport") {
		t.Errorf("passport should be visible when wna=Ya")
	}

	// Validate with wna=Tidak → passport hidden → valid.
	res = previewValidate(t, client, base, token, map[string]any{"wna": "Tidak"})
	if !res.Valid {
		t.Fatalf("expected valid (passport hidden), got errors %+v", res.Errors)
	}
	if containsKey(res.VisibleFields, "passport") {
		t.Errorf("passport should be hidden when wna=Tidak")
	}

	// Add a category-scoped field (only for 42K).
	resp = postJSON(t, client, base+"/fields", map[string]any{
		"fieldType": "text", "label": "Jersey", "fieldKey": "jersey",
		"categoryScope": []string{cat.ID},
	}, token)
	resp.Body.Close()

	// Preview for 42K includes jersey; preview without category excludes it.
	if !containsKey(previewKeys(t, client, base, token, "?categoryId="+cat.ID), "jersey") {
		t.Error("jersey should appear in 42K preview")
	}
	if containsKey(previewKeys(t, client, base, token, ""), "jersey") {
		t.Error("jersey should NOT appear in no-category preview")
	}
}

type pvResp struct {
	Valid         bool `json:"valid"`
	Errors        []struct {
		FieldKey string `json:"fieldKey"`
	} `json:"errors"`
	VisibleFields []string `json:"visibleFields"`
}

func previewValidate(t *testing.T, client *http.Client, base, token string, answers map[string]any) pvResp {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"answers": answers})
	req, _ := http.NewRequest(http.MethodPost, base+"/preview/validate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, _ := client.Do(req)
	defer resp.Body.Close()
	var r pvResp
	json.NewDecoder(resp.Body).Decode(&r)
	return r
}

func previewKeys(t *testing.T, client *http.Client, base, token, query string) []string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, base+"/preview"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := client.Do(req)
	defer resp.Body.Close()
	var fields []struct {
		FieldKey string `json:"fieldKey"`
	}
	json.NewDecoder(resp.Body).Decode(&fields)
	keys := make([]string, 0, len(fields))
	for _, f := range fields {
		keys = append(keys, f.FieldKey)
	}
	return keys
}

func containsKey(keys []string, want string) bool {
	for _, k := range keys {
		if k == want {
			return true
		}
	}
	return false
}
```

- [ ] **Step 6: Tenant isolation + cyclic conditional test**

Create `services/api/tests/integration/form_tenant_test.go`:
```go
//go:build integration

package integration

import (
	"net/http"
	"testing"
)

func TestFormTenantIsolation(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	tokenA, orgA, _ := loginCreateOrg(t, client, srv.URL, "a@x.com", "Org A")
	tokenB, orgB, _ := loginCreateOrg(t, client, srv.URL, "b@x.com", "Org B")
	eventA := createEvent(t, client, srv.URL, tokenA, orgA, "A Event")

	// B reads A's form via B's org path → 404 (event not in org B).
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgB+"/events/"+eventA+"/form", nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	resp, _ := client.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-tenant form via own org = %d, want 404", resp.StatusCode)
	}

	// B reads A's form via A's org path → 403 (not a member of org A).
	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/api/v1/organizations/"+orgA+"/events/"+eventA+"/form", nil)
	req.Header.Set("Authorization", "Bearer "+tokenB)
	resp, _ = client.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-org form = %d, want 403", resp.StatusCode)
	}
}

func TestFormConditional_CyclicRejected(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	srv := newTestServer(t, pool)
	client := &http.Client{}

	token, orgID, _ := loginCreateOrg(t, client, srv.URL, "owner@x.com", "Cycle Org")
	eventID := createEvent(t, client, srv.URL, token, orgID, "Marathon")
	base := srv.URL + "/api/v1/organizations/" + orgID + "/events/" + eventID + "/form"

	req, _ := http.NewRequest(http.MethodGet, base, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	client.Do(req)

	// Field referencing a field that doesn't exist yet (forward/unknown ref) → rejected.
	resp := postJSON(t, client, base+"/fields", map[string]any{
		"fieldType": "text", "label": "P", "fieldKey": "passport",
		"conditional": map[string]any{"field": "wna", "op": "equals", "value": "Ya"},
	}, token)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown-ref conditional = %d, want 400", resp.StatusCode)
	}
}
```

- [ ] **Step 7: Run integration tests**

Run:
```bash
make test-integration
```
Expected: all Phase 2/3/4 integration tests PASS. If the test DB is missing seeded RBAC templates (wiped by a prior truncate cascade), re-seed: `goose -dir database/migrations postgres "postgres://localhost:5432/ivyticketing_test?sslmode=disable" down-to 7 && goose ... up` — but the truncate helper preserves template roles, so this should not be needed.

- [ ] **Step 8: Commit**

```bash
git add services/api/tests/integration
git commit -m "test(api): add phase 4 form-builder integration tests"
```

---

## Task 10: Full Definition-of-Done verification

- [ ] **Step 1: Run the complete gate**

Run:
```bash
# DoD #1 migrations roundtrip
make migrate-down && make migrate-up
# DoD #9/#10 unit tests + sqlc clean + build + vet
make sqlc && cd services/api && go build ./... && go vet ./... && go test ./... && cd ../..
# DoD #2-#8 integration
make test-db-setup && make test-integration
# DoD #11 no hardcoded secrets (none expected for forms; sanity check)
grep -rn "SECRET" services/api/internal/modules/forms services/api/internal/platform/formschema || echo "no secrets in forms"
```
Expected: every command succeeds. Mapping to spec DoD:
- #1 migrations up/down → roundtrip
- #2 field CRUD + reorder + upsert → unit (forms/service_test) + integration (form_flow)
- #3 formschema package (validate, conditional, evaluate, validateAnswers) → unit (formschema tests)
- #4 conditional logic via preview/validate → integration (form_conditional)
- #5 category_scope via preview → integration (form_conditional)
- #6 preview shows effective fields → integration (form_flow + form_conditional)
- #7 tenant isolation → integration (form_tenant)
- #8 definition validation rejects bad input → unit + integration (dup key, cyclic/unknown ref, options)
- #9 `go test ./...` green → final gate
- #10 sqlc clean → `make sqlc`
- #11 no hardcoded secrets → grep

- [ ] **Step 2: Confirm everything green**

If any step fails, fix the root cause — do not weaken a test.

---

## Task 11: README + CHANGELOG

**Files:**
- Modify: `README.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Document Phase 4 in README**

Add a "Phase 4 — Custom Registration Form Builder" section to `README.md` (newest-first, above Phase 3), covering: the form-builder endpoints, conditional logic (AND/OR), category scoping, and the preview/validate dry-run. Include a short curl smoke test:
```markdown
## Phase 4 — Custom Registration Form Builder

Per-event form builder: fields (text/email/dropdown/etc), per-field validation,
AND/OR conditional logic, per-category field scoping, and a preview/dry-run validator.
Builder only — participant submission arrives in Phase 5.

### Smoke test

```bash
# auto-create form + add a field (needs form.manage; use an Owner token)
curl -s localhost:8080/api/v1/organizations/<orgId>/events/<eventId>/form \
  -H "authorization: Bearer <accessToken>"

curl -s -X POST localhost:8080/api/v1/organizations/<orgId>/events/<eventId>/form/fields \
  -H "authorization: Bearer <accessToken>" -H 'content-type: application/json' \
  -d '{"fieldType":"dropdown","label":"WNA","fieldKey":"wna","options":["Ya","Tidak"]}'

# dry-run validate
curl -s -X POST "localhost:8080/api/v1/organizations/<orgId>/events/<eventId>/form/preview/validate" \
  -H "authorization: Bearer <accessToken>" -H 'content-type: application/json' \
  -d '{"answers":{"wna":"Ya"}}'
```
```

- [ ] **Step 2: Add CHANGELOG entry**

Prepend a Phase 4 section to `CHANGELOG.md` (newest-first, above Phase 3):
```markdown
## [Phase 4] — 2026-06-07

Custom registration form builder. Backend-only (builder; submission deferred to Phase 5).

### Added

**Form builder**
- One form per event (auto-created on first `GET /form`)
- Field CRUD: `POST/PUT/DELETE .../events/:eventId/form/fields[/:fieldId]`
- Reorder: `PUT .../form/fields/reorder { fieldIds }`
- Field types: text, email, phone, number, date, dropdown, radio, checkbox, textarea, file
- Per-field validation rules (minLength/maxLength/pattern for text; min/max for number/date)

**Conditional logic**
- Multi-condition AND/OR tree (`{op:"and"|"or", rules:[...]}` + leaves `{field, op, value}`)
- Operators: equals, notEquals, in, notIn, gt, gte, lt, lte
- Acyclic (refs earlier fields only), depth ≤ 3, ≤ 20 leaves/field

**Per-category scoping**
- `categoryScope` limits a field to specific categories (null = all)

**Preview / dry-run**
- `GET .../form/preview?categoryId=` — effective visible fields for a category
- `POST .../form/preview/validate?categoryId=` — runs conditional + validation over sample answers

**Pure logic package** `formschema`
- `ValidateFields` (definition validation), `Evaluate` (conditional), `ValidateAnswers` (preview) — no DB, fully unit-tested

**Database** (goose migrations 00010–00011)
- Tables: `form_schemas`, `form_fields`

**Tests**
- Unit: formschema (validate, conditional AND/OR, answers), forms service (upsert, CRUD, reorder, tenant guard, referenced-field delete)
- Integration: full form flow, conditional show/hide, category scope, tenant isolation (404/403)
```

- [ ] **Step 3: Commit**

```bash
git add README.md CHANGELOG.md
git commit -m "docs: document phase 4 form builder and update changelog"
```

---

## Done

Phase 4 complete. All 11 tasks across 4 parts: migrations + queries + formschema field/validation (Part 1), conditional evaluator + answer validation (Part 2), forms module (Part 3), integration + verification + docs (Part 4).
