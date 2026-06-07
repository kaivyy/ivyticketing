# Phase 4 Plan — Part 3: Forms Module (Tasks 6-8)

> Part of the Phase 4 implementation plan. Index: [2026-06-07-phase4-form-builder.md](2026-06-07-phase4-form-builder.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **Depends on:** Parts 1-2 (`formschema` complete, migrations + sqlc queries done). Verify generated `db.FormSchema`/`db.FormField` + `*Params` types before writing repository/service.

---

## Task 6: Forms errors, DTOs, repository

**Files:**
- Create: `services/api/internal/modules/forms/errors.go`
- Create: `services/api/internal/modules/forms/dto.go`
- Create: `services/api/internal/modules/forms/repository.go`

- [ ] **Step 1: Typed errors**

Create `services/api/internal/modules/forms/errors.go`:
```go
package forms

import (
	"net/http"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

var (
	ErrEventNotFound    = apperr.New(http.StatusNotFound, "EVENT_NOT_FOUND", "event not found")
	ErrFormNotFound     = apperr.New(http.StatusNotFound, "FORM_NOT_FOUND", "form not found")
	ErrFieldNotFound    = apperr.New(http.StatusNotFound, "FIELD_NOT_FOUND", "field not found")
	ErrFieldReferenced  = apperr.New(http.StatusConflict, "FIELD_REFERENCED", "field is referenced by another field's conditional")
	ErrInvalidReorder   = apperr.New(http.StatusBadRequest, "INVALID_REORDER_SET", "reorder set must match exactly the form's fields")
	ErrCategoryNotInEvt = apperr.New(http.StatusBadRequest, "CATEGORY_NOT_IN_EVENT", "category does not belong to this event")
)
```
Note: definition-validation failures from `formschema` carry their own `Code`/`Message` (a `*formschema.ValidationError`). The service maps those to `apperr` with the same code (see Task 7).

- [ ] **Step 2: DTOs**

Create `services/api/internal/modules/forms/dto.go`:
```go
package forms

import (
	"time"

	"github.com/google/uuid"

	"github.com/varin/ivyticketing/services/api/internal/platform/formschema"
)

type FormResponse struct {
	ID        uuid.UUID       `json:"id"`
	EventID   uuid.UUID       `json:"eventId"`
	Name      string          `json:"name"`
	Fields    []FieldResponse `json:"fields"`
	CreatedAt time.Time       `json:"createdAt"`
}

type FieldResponse struct {
	ID            uuid.UUID               `json:"id"`
	FieldType     string                  `json:"fieldType"`
	Label         string                  `json:"label"`
	FieldKey      string                  `json:"fieldKey"`
	HelpText      string                  `json:"helpText"`
	IsRequired    bool                    `json:"isRequired"`
	DisplayOrder  int                     `json:"displayOrder"`
	Options       []string                `json:"options,omitempty"`
	Validation    *formschema.Validation  `json:"validation,omitempty"`
	Conditional   *formschema.Condition   `json:"conditional,omitempty"`
	CategoryScope []string                `json:"categoryScope,omitempty"`
}

type UpdateFormRequest struct {
	Name string `json:"name"`
}

type FieldRequest struct {
	FieldType     string                  `json:"fieldType"`
	Label         string                  `json:"label"`
	FieldKey      string                  `json:"fieldKey"`
	HelpText      string                  `json:"helpText"`
	IsRequired    bool                    `json:"isRequired"`
	Options       []string                `json:"options"`
	Validation    *formschema.Validation  `json:"validation"`
	Conditional   *formschema.Condition   `json:"conditional"`
	CategoryScope []string                `json:"categoryScope"`
}

type ReorderRequest struct {
	FieldIDs []uuid.UUID `json:"fieldIds"`
}

type PreviewValidateRequest struct {
	Answers map[string]any `json:"answers"`
}

type PreviewValidateResponse struct {
	Valid         bool                    `json:"valid"`
	Errors        []formschema.FieldError `json:"errors"`
	VisibleFields []string                `json:"visibleFields"`
}
```

- [ ] **Step 3: Repository interface + adapter**

Create `services/api/internal/modules/forms/repository.go`:
```go
package forms

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type Repository interface {
	ExecTx(ctx context.Context, fn func(Repository) error) error
	GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error)
	GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error)
	GetFormSchemaByEvent(ctx context.Context, eventID uuid.UUID) (db.FormSchema, error)
	CreateFormSchema(ctx context.Context, arg db.CreateFormSchemaParams) (db.FormSchema, error)
	UpdateFormSchemaName(ctx context.Context, arg db.UpdateFormSchemaNameParams) (db.FormSchema, error)
	ListFieldsBySchema(ctx context.Context, schemaID uuid.UUID) ([]db.FormField, error)
	GetFieldByID(ctx context.Context, id uuid.UUID) (db.FormField, error)
	CreateField(ctx context.Context, arg db.CreateFieldParams) (db.FormField, error)
	UpdateField(ctx context.Context, arg db.UpdateFieldParams) (db.FormField, error)
	UpdateFieldOrder(ctx context.Context, arg db.UpdateFieldOrderParams) error
	DeleteField(ctx context.Context, arg db.DeleteFieldParams) error
	MaxFieldOrder(ctx context.Context, schemaID uuid.UUID) (int32, error)
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
	if err := fn(&sqlcRepo{pool: r.pool, q: db.New(tx)}); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (r *sqlcRepo) GetEventByID(ctx context.Context, id uuid.UUID) (db.Event, error) {
	return r.q.GetEventByID(ctx, id)
}
func (r *sqlcRepo) GetCategoryByID(ctx context.Context, id uuid.UUID) (db.EventCategory, error) {
	return r.q.GetCategoryByID(ctx, id)
}
func (r *sqlcRepo) GetFormSchemaByEvent(ctx context.Context, eventID uuid.UUID) (db.FormSchema, error) {
	return r.q.GetFormSchemaByEvent(ctx, eventID)
}
func (r *sqlcRepo) CreateFormSchema(ctx context.Context, arg db.CreateFormSchemaParams) (db.FormSchema, error) {
	return r.q.CreateFormSchema(ctx, arg)
}
func (r *sqlcRepo) UpdateFormSchemaName(ctx context.Context, arg db.UpdateFormSchemaNameParams) (db.FormSchema, error) {
	return r.q.UpdateFormSchemaName(ctx, arg)
}
func (r *sqlcRepo) ListFieldsBySchema(ctx context.Context, schemaID uuid.UUID) ([]db.FormField, error) {
	return r.q.ListFieldsBySchema(ctx, schemaID)
}
func (r *sqlcRepo) GetFieldByID(ctx context.Context, id uuid.UUID) (db.FormField, error) {
	return r.q.GetFieldByID(ctx, id)
}
func (r *sqlcRepo) CreateField(ctx context.Context, arg db.CreateFieldParams) (db.FormField, error) {
	return r.q.CreateField(ctx, arg)
}
func (r *sqlcRepo) UpdateField(ctx context.Context, arg db.UpdateFieldParams) (db.FormField, error) {
	return r.q.UpdateField(ctx, arg)
}
func (r *sqlcRepo) UpdateFieldOrder(ctx context.Context, arg db.UpdateFieldOrderParams) error {
	return r.q.UpdateFieldOrder(ctx, arg)
}
func (r *sqlcRepo) DeleteField(ctx context.Context, arg db.DeleteFieldParams) error {
	return r.q.DeleteField(ctx, arg)
}
func (r *sqlcRepo) MaxFieldOrder(ctx context.Context, schemaID uuid.UUID) (int32, error) {
	return r.q.MaxFieldOrder(ctx, schemaID)
}
```
Note: `MaxFieldOrder` return type — sqlc generates the `::int` cast result. Confirm whether it's `int32` or `interface{}`/`int`. If sqlc returns `interface{}`, change the query to `COALESCE(MAX(display_order), 0)::integer` and the signature accordingly. Verify against `forms.sql.go`.

- [ ] **Step 4: Commit**

```bash
git add services/api/internal/modules/forms/errors.go services/api/internal/modules/forms/dto.go services/api/internal/modules/forms/repository.go
git commit -m "feat(forms): add errors, dtos, and repository"
```

---

## Task 7: Forms service (upsert, field CRUD, reorder, preview)

**Files:**
- Create: `services/api/internal/modules/forms/mapping.go`
- Create: `services/api/internal/modules/forms/service.go`
- Test: `services/api/internal/modules/forms/service_test.go`

The service converts between `db.FormField` (jsonb as `[]byte`) and `formschema.Field`,
runs `formschema.ValidateFields` on the whole set before commit, and enforces tenant
ownership (event belongs to org; mismatch → 404).

- [ ] **Step 1: Mapping helpers (db row ↔ formschema/dto)**

Create `services/api/internal/modules/forms/mapping.go`:
```go
package forms

import (
	"encoding/json"

	"github.com/varin/ivyticketing/services/api/internal/db"
	"github.com/varin/ivyticketing/services/api/internal/platform/formschema"
)

// toSchemaField converts a db row to the pure formschema.Field (for validation).
func toSchemaField(row db.FormField) (formschema.Field, error) {
	f := formschema.Field{
		Key:          row.FieldKey,
		Type:         formschema.FieldType(row.FieldType),
		Label:        row.Label,
		Required:     row.IsRequired,
		DisplayOrder: int(row.DisplayOrder),
	}
	if len(row.Options) > 0 {
		if err := json.Unmarshal(row.Options, &f.Options); err != nil {
			return f, err
		}
	}
	if len(row.Validation) > 0 {
		if err := json.Unmarshal(row.Validation, &f.Validation); err != nil {
			return f, err
		}
	}
	if len(row.Conditional) > 0 {
		if err := json.Unmarshal(row.Conditional, &f.Conditional); err != nil {
			return f, err
		}
	}
	if len(row.CategoryScope) > 0 {
		if err := json.Unmarshal(row.CategoryScope, &f.CategoryScope); err != nil {
			return f, err
		}
	}
	return f, nil
}

func toFieldResponse(row db.FormField) (FieldResponse, error) {
	sf, err := toSchemaField(row)
	if err != nil {
		return FieldResponse{}, err
	}
	return FieldResponse{
		ID:            row.ID,
		FieldType:     row.FieldType,
		Label:         row.Label,
		FieldKey:      row.FieldKey,
		HelpText:      row.HelpText.String,
		IsRequired:    row.IsRequired,
		DisplayOrder:  int(row.DisplayOrder),
		Options:       sf.Options,
		Validation:    sf.Validation,
		Conditional:   sf.Conditional,
		CategoryScope: sf.CategoryScope,
	}, nil
}

// marshalJSONB marshals a value to []byte for a jsonb column, or nil if empty.
func marshalOptions(opts []string) ([]byte, error) {
	if len(opts) == 0 {
		return nil, nil
	}
	return json.Marshal(opts)
}

func marshalValidation(v *formschema.Validation) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func marshalConditional(c *formschema.Condition) ([]byte, error) {
	if c == nil {
		return nil, nil
	}
	return json.Marshal(c)
}

func marshalScope(s []string) ([]byte, error) {
	if len(s) == 0 {
		return nil, nil
	}
	return json.Marshal(s)
}
```
Note: confirm `db.FormField` field names against generated `models.go` (`Options []byte`, `Validation []byte`, `Conditional []byte`, `CategoryScope []byte`, `HelpText pgtype.Text`, `IsRequired bool`, `DisplayOrder int32`). Adjust if different.

- [ ] **Step 2: Write the failing service tests**

Create `services/api/internal/modules/forms/service_test.go`:
```go
package forms

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
)

type fakeRepo struct {
	events  map[uuid.UUID]db.Event
	schemas map[uuid.UUID]db.FormSchema // by event_id
	fields  map[uuid.UUID]db.FormField  // by field id
	nextOrd int32
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		events:  map[uuid.UUID]db.Event{},
		schemas: map[uuid.UUID]db.FormSchema{},
		fields:  map[uuid.UUID]db.FormField{},
	}
}

func (f *fakeRepo) seedEvent(orgID uuid.UUID) db.Event {
	e := db.Event{ID: uuid.New(), OrganizationID: orgID, Name: "E", Slug: "e", Status: "draft"}
	f.events[e.ID] = e
	return e
}

func (f *fakeRepo) ExecTx(ctx context.Context, fn func(Repository) error) error { return fn(f) }
func (f *fakeRepo) GetEventByID(_ context.Context, id uuid.UUID) (db.Event, error) {
	e, ok := f.events[id]
	if !ok {
		return db.Event{}, pgx.ErrNoRows
	}
	return e, nil
}
func (f *fakeRepo) GetCategoryByID(_ context.Context, id uuid.UUID) (db.EventCategory, error) {
	return db.EventCategory{}, pgx.ErrNoRows
}
func (f *fakeRepo) GetFormSchemaByEvent(_ context.Context, eventID uuid.UUID) (db.FormSchema, error) {
	s, ok := f.schemas[eventID]
	if !ok {
		return db.FormSchema{}, pgx.ErrNoRows
	}
	return s, nil
}
func (f *fakeRepo) CreateFormSchema(_ context.Context, arg db.CreateFormSchemaParams) (db.FormSchema, error) {
	s := db.FormSchema{ID: uuid.New(), OrganizationID: arg.OrganizationID, EventID: arg.EventID, Name: arg.Name}
	f.schemas[arg.EventID] = s
	return s, nil
}
func (f *fakeRepo) UpdateFormSchemaName(_ context.Context, arg db.UpdateFormSchemaNameParams) (db.FormSchema, error) {
	for eid, s := range f.schemas {
		if s.ID == arg.ID {
			s.Name = arg.Name
			f.schemas[eid] = s
			return s, nil
		}
	}
	return db.FormSchema{}, pgx.ErrNoRows
}
func (f *fakeRepo) ListFieldsBySchema(_ context.Context, schemaID uuid.UUID) ([]db.FormField, error) {
	var out []db.FormField
	for _, fld := range f.fields {
		if fld.FormSchemaID == schemaID {
			out = append(out, fld)
		}
	}
	// sort by display order
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].DisplayOrder < out[i].DisplayOrder {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}
func (f *fakeRepo) GetFieldByID(_ context.Context, id uuid.UUID) (db.FormField, error) {
	fld, ok := f.fields[id]
	if !ok {
		return db.FormField{}, pgx.ErrNoRows
	}
	return fld, nil
}
func (f *fakeRepo) CreateField(_ context.Context, arg db.CreateFieldParams) (db.FormField, error) {
	fld := db.FormField{
		ID: uuid.New(), OrganizationID: arg.OrganizationID, FormSchemaID: arg.FormSchemaID,
		FieldType: arg.FieldType, Label: arg.Label, FieldKey: arg.FieldKey,
		HelpText: arg.HelpText, IsRequired: arg.IsRequired, DisplayOrder: arg.DisplayOrder,
		Options: arg.Options, Validation: arg.Validation, Conditional: arg.Conditional, CategoryScope: arg.CategoryScope,
	}
	f.fields[fld.ID] = fld
	return fld, nil
}
func (f *fakeRepo) UpdateField(_ context.Context, arg db.UpdateFieldParams) (db.FormField, error) {
	fld := f.fields[arg.ID]
	fld.FieldType = arg.FieldType
	fld.Label = arg.Label
	fld.FieldKey = arg.FieldKey
	fld.HelpText = arg.HelpText
	fld.IsRequired = arg.IsRequired
	fld.Options = arg.Options
	fld.Validation = arg.Validation
	fld.Conditional = arg.Conditional
	fld.CategoryScope = arg.CategoryScope
	f.fields[arg.ID] = fld
	return fld, nil
}
func (f *fakeRepo) UpdateFieldOrder(_ context.Context, arg db.UpdateFieldOrderParams) error {
	fld := f.fields[arg.ID]
	fld.DisplayOrder = arg.DisplayOrder
	f.fields[arg.ID] = fld
	return nil
}
func (f *fakeRepo) DeleteField(_ context.Context, arg db.DeleteFieldParams) error {
	delete(f.fields, arg.ID)
	return nil
}
func (f *fakeRepo) MaxFieldOrder(_ context.Context, schemaID uuid.UUID) (int32, error) {
	var max int32
	for _, fld := range f.fields {
		if fld.FormSchemaID == schemaID && fld.DisplayOrder > max {
			max = fld.DisplayOrder
		}
	}
	return max, nil
}

func TestGetForm_AutoCreatesEmpty(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	ev := repo.seedEvent(orgID)

	form, err := svc.GetForm(context.Background(), orgID, ev.ID)
	if err != nil {
		t.Fatalf("get form: %v", err)
	}
	if form.EventID != ev.ID || len(form.Fields) != 0 {
		t.Fatalf("expected empty auto-created form, got %+v", form)
	}
}

func TestGetForm_TenantMismatch(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ev := repo.seedEvent(uuid.New())
	if _, err := svc.GetForm(context.Background(), uuid.New(), ev.ID); err != ErrEventNotFound {
		t.Fatalf("err = %v, want ErrEventNotFound", err)
	}
}

func TestAddField_ValidatesAndPersists(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	ev := repo.seedEvent(orgID)

	got, err := svc.AddField(context.Background(), orgID, ev.ID, FieldRequest{
		FieldType: "text", Label: "Nama", FieldKey: "nama", IsRequired: true,
	})
	if err != nil {
		t.Fatalf("add field: %v", err)
	}
	if got.FieldKey != "nama" {
		t.Errorf("fieldKey = %q", got.FieldKey)
	}
}

func TestAddField_RejectsDuplicateKey(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	ev := repo.seedEvent(orgID)
	if _, err := svc.AddField(context.Background(), orgID, ev.ID, FieldRequest{FieldType: "text", Label: "N", FieldKey: "nama"}); err != nil {
		t.Fatalf("first add: %v", err)
	}
	_, err := svc.AddField(context.Background(), orgID, ev.ID, FieldRequest{FieldType: "email", Label: "E", FieldKey: "nama"})
	if err == nil {
		t.Fatal("expected duplicate key error")
	}
	ae := apperrCode(err)
	if ae != "DUPLICATE_FIELD_KEY" {
		t.Fatalf("code = %q, want DUPLICATE_FIELD_KEY", ae)
	}
}

func TestAddField_RejectsCyclicConditional(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	ev := repo.seedEvent(orgID)
	// field referencing a non-existent earlier field
	cond := json.RawMessage(`{"field":"ghost","op":"equals","value":"x"}`)
	var c struct{}
	_ = json.Unmarshal(cond, &c)
	_, err := svc.AddField(context.Background(), orgID, ev.ID, FieldRequest{
		FieldType: "text", Label: "P", FieldKey: "passport",
		Conditional: parseCondForTest(`{"field":"ghost","op":"equals","value":"x"}`),
	})
	if err == nil {
		t.Fatal("expected conditional unknown-field error")
	}
}

func TestReorder_RejectsMismatchedSet(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	ev := repo.seedEvent(orgID)
	svc.AddField(context.Background(), orgID, ev.ID, FieldRequest{FieldType: "text", Label: "A", FieldKey: "a"})
	// reorder with an unknown id
	if err := svc.Reorder(context.Background(), orgID, ev.ID, []uuid.UUID{uuid.New()}); err != ErrInvalidReorder {
		t.Fatalf("err = %v, want ErrInvalidReorder", err)
	}
}

func TestDeleteField_RejectsReferenced(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	ev := repo.seedEvent(orgID)
	a, _ := svc.AddField(context.Background(), orgID, ev.ID, FieldRequest{FieldType: "dropdown", Label: "WNA", FieldKey: "wna", Options: []string{"Ya", "Tidak"}})
	_, _ = svc.AddField(context.Background(), orgID, ev.ID, FieldRequest{
		FieldType: "text", Label: "Passport", FieldKey: "passport",
		Conditional: parseCondForTest(`{"field":"wna","op":"equals","value":"Ya"}`),
	})
	if err := svc.DeleteField(context.Background(), orgID, ev.ID, a.ID); err != ErrFieldReferenced {
		t.Fatalf("err = %v, want ErrFieldReferenced", err)
	}
}

// helpers
var _ = pgtype.Text{}
```
Note: this test references two helpers — `apperrCode(err) string` (extracts the `Code` from a `*apperr.APIError`) and `parseCondForTest(s string) *formschema.Condition` (unmarshal a JSON string to a Condition). Add them at the bottom of `service_test.go`:
```go
func apperrCode(err error) string {
	type coder interface{ Error() string }
	var ae *apperr.APIError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

func parseCondForTest(s string) *formschema.Condition {
	var c formschema.Condition
	_ = json.Unmarshal([]byte(s), &c)
	return &c
}
```
with imports `"errors"`, `apperr ".../platform/errors"`, `".../platform/formschema"`. Confirm `apperr.APIError` has an exported `Code` field (it does — from Phase 2).

- [ ] **Step 3: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/forms/ -v; cd ../..
```
Expected: FAIL — `undefined: NewService`.

- [ ] **Step 4: Implement the service**

Create `services/api/internal/modules/forms/service.go`:
```go
package forms

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
	"github.com/varin/ivyticketing/services/api/internal/platform/formschema"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

// ensureForm loads or lazily creates the form schema for an org-owned event.
func (s *Service) ensureForm(ctx context.Context, orgID, eventID uuid.UUID) (db.FormSchema, error) {
	ev, err := s.repo.GetEventByID(ctx, eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.FormSchema{}, ErrEventNotFound
	} else if err != nil {
		return db.FormSchema{}, err
	}
	if ev.OrganizationID != orgID {
		return db.FormSchema{}, ErrEventNotFound
	}
	form, err := s.repo.GetFormSchemaByEvent(ctx, eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.repo.CreateFormSchema(ctx, db.CreateFormSchemaParams{
			OrganizationID: orgID, EventID: eventID, Name: "",
		})
	}
	return form, err
}

func (s *Service) GetForm(ctx context.Context, orgID, eventID uuid.UUID) (FormResponse, error) {
	form, err := s.ensureForm(ctx, orgID, eventID)
	if err != nil {
		return FormResponse{}, err
	}
	return s.buildFormResponse(ctx, form)
}

func (s *Service) UpdateForm(ctx context.Context, orgID, eventID uuid.UUID, req UpdateFormRequest) (FormResponse, error) {
	form, err := s.ensureForm(ctx, orgID, eventID)
	if err != nil {
		return FormResponse{}, err
	}
	form, err = s.repo.UpdateFormSchemaName(ctx, db.UpdateFormSchemaNameParams{
		ID: form.ID, Name: req.Name, OrganizationID: orgID,
	})
	if err != nil {
		return FormResponse{}, err
	}
	return s.buildFormResponse(ctx, form)
}

func (s *Service) AddField(ctx context.Context, orgID, eventID uuid.UUID, req FieldRequest) (FieldResponse, error) {
	form, err := s.ensureForm(ctx, orgID, eventID)
	if err != nil {
		return FieldResponse{}, err
	}
	if err := s.validateCategoryScope(ctx, eventID, req.CategoryScope); err != nil {
		return FieldResponse{}, err
	}

	rows, err := s.repo.ListFieldsBySchema(ctx, form.ID)
	if err != nil {
		return FieldResponse{}, err
	}
	maxOrd, err := s.repo.MaxFieldOrder(ctx, form.ID)
	if err != nil {
		return FieldResponse{}, err
	}
	newOrder := int(maxOrd) + 1

	// Build the prospective full set and validate.
	candidate := req.toSchemaField(newOrder)
	existing, err := toSchemaFields(rows)
	if err != nil {
		return FieldResponse{}, err
	}
	if err := validateSet(append(existing, candidate)); err != nil {
		return FieldResponse{}, err
	}

	created, err := s.persistNewField(ctx, orgID, form.ID, req, int32(newOrder))
	if err != nil {
		return FieldResponse{}, err
	}
	return toFieldResponse(created)
}

func (s *Service) UpdateField(ctx context.Context, orgID, eventID, fieldID uuid.UUID, req FieldRequest) (FieldResponse, error) {
	form, err := s.ensureForm(ctx, orgID, eventID)
	if err != nil {
		return FieldResponse{}, err
	}
	cur, err := s.loadField(ctx, form.ID, fieldID)
	if err != nil {
		return FieldResponse{}, err
	}
	if err := s.validateCategoryScope(ctx, eventID, req.CategoryScope); err != nil {
		return FieldResponse{}, err
	}

	rows, err := s.repo.ListFieldsBySchema(ctx, form.ID)
	if err != nil {
		return FieldResponse{}, err
	}
	// Build set with this field replaced.
	var set []formschema.Field
	for _, r := range rows {
		if r.ID == fieldID {
			set = append(set, req.toSchemaField(int(cur.DisplayOrder)))
			continue
		}
		sf, err := toSchemaField(r)
		if err != nil {
			return FieldResponse{}, err
		}
		set = append(set, sf)
	}
	if err := validateSet(set); err != nil {
		return FieldResponse{}, err
	}

	opts, _ := marshalOptions(req.Options)
	val, _ := marshalValidation(req.Validation)
	cond, _ := marshalConditional(req.Conditional)
	scope, _ := marshalScope(req.CategoryScope)
	updated, err := s.repo.UpdateField(ctx, db.UpdateFieldParams{
		ID: fieldID, FieldType: req.FieldType, Label: req.Label, FieldKey: req.FieldKey,
		HelpText: pgText(req.HelpText), IsRequired: req.IsRequired,
		Options: opts, Validation: val, Conditional: cond, CategoryScope: scope,
		FormSchemaID: form.ID,
	})
	if err != nil {
		return FieldResponse{}, err
	}
	return toFieldResponse(updated)
}

func (s *Service) DeleteField(ctx context.Context, orgID, eventID, fieldID uuid.UUID) error {
	form, err := s.ensureForm(ctx, orgID, eventID)
	if err != nil {
		return err
	}
	target, err := s.loadField(ctx, form.ID, fieldID)
	if err != nil {
		return err
	}
	rows, err := s.repo.ListFieldsBySchema(ctx, form.ID)
	if err != nil {
		return err
	}
	// reject if another field's conditional references this field's key
	for _, r := range rows {
		if r.ID == fieldID {
			continue
		}
		sf, err := toSchemaField(r)
		if err != nil {
			return err
		}
		if sf.Conditional != nil && conditionRefs(sf.Conditional, target.FieldKey) {
			return ErrFieldReferenced
		}
	}
	return s.repo.DeleteField(ctx, db.DeleteFieldParams{ID: fieldID, FormSchemaID: form.ID})
}

func (s *Service) Reorder(ctx context.Context, orgID, eventID uuid.UUID, ids []uuid.UUID) error {
	form, err := s.ensureForm(ctx, orgID, eventID)
	if err != nil {
		return err
	}
	rows, err := s.repo.ListFieldsBySchema(ctx, form.ID)
	if err != nil {
		return err
	}
	if len(ids) != len(rows) {
		return ErrInvalidReorder
	}
	existing := make(map[uuid.UUID]bool, len(rows))
	for _, r := range rows {
		existing[r.ID] = true
	}
	seen := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		if !existing[id] || seen[id] {
			return ErrInvalidReorder
		}
		seen[id] = true
	}
	return s.repo.ExecTx(ctx, func(r Repository) error {
		for i, id := range ids {
			if err := r.UpdateFieldOrder(ctx, db.UpdateFieldOrderParams{
				ID: id, DisplayOrder: int32(i + 1), FormSchemaID: form.ID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) Preview(ctx context.Context, orgID, eventID uuid.UUID, categoryID *uuid.UUID) ([]FieldResponse, error) {
	form, err := s.ensureForm(ctx, orgID, eventID)
	if err != nil {
		return nil, err
	}
	if err := s.assertCategory(ctx, eventID, categoryID); err != nil {
		return nil, err
	}
	rows, err := s.repo.ListFieldsBySchema(ctx, form.ID)
	if err != nil {
		return nil, err
	}
	fields, err := toSchemaFields(rows)
	if err != nil {
		return nil, err
	}
	var catStr *string
	if categoryID != nil {
		s := categoryID.String()
		catStr = &s
	}
	visible := formschema.VisibleFields(fields, map[string]any{}, catStr)
	// map visible keys back to FieldResponse (preserve full definition)
	byKey := make(map[string]db.FormField, len(rows))
	for _, r := range rows {
		byKey[r.FieldKey] = r
	}
	out := make([]FieldResponse, 0, len(visible))
	for _, vf := range visible {
		fr, err := toFieldResponse(byKey[vf.Key])
		if err != nil {
			return nil, err
		}
		out = append(out, fr)
	}
	return out, nil
}

func (s *Service) PreviewValidate(ctx context.Context, orgID, eventID uuid.UUID, categoryID *uuid.UUID, answers map[string]any) (PreviewValidateResponse, error) {
	form, err := s.ensureForm(ctx, orgID, eventID)
	if err != nil {
		return PreviewValidateResponse{}, err
	}
	if err := s.assertCategory(ctx, eventID, categoryID); err != nil {
		return PreviewValidateResponse{}, err
	}
	rows, err := s.repo.ListFieldsBySchema(ctx, form.ID)
	if err != nil {
		return PreviewValidateResponse{}, err
	}
	fields, err := toSchemaFields(rows)
	if err != nil {
		return PreviewValidateResponse{}, err
	}
	var catStr *string
	if categoryID != nil {
		v := categoryID.String()
		catStr = &v
	}
	errs := formschema.ValidateAnswers(fields, answers, catStr)
	visible := formschema.VisibleFields(fields, answers, catStr)
	keys := make([]string, 0, len(visible))
	for _, vf := range visible {
		keys = append(keys, vf.Key)
	}
	if errs == nil {
		errs = []formschema.FieldError{}
	}
	return PreviewValidateResponse{Valid: len(errs) == 0, Errors: errs, VisibleFields: keys}, nil
}

// --- helpers ---

func (s *Service) loadField(ctx context.Context, schemaID, fieldID uuid.UUID) (db.FormField, error) {
	fld, err := s.repo.GetFieldByID(ctx, fieldID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.FormField{}, ErrFieldNotFound
	} else if err != nil {
		return db.FormField{}, err
	}
	if fld.FormSchemaID != schemaID {
		return db.FormField{}, ErrFieldNotFound
	}
	return fld, nil
}

func (s *Service) persistNewField(ctx context.Context, orgID, schemaID uuid.UUID, req FieldRequest, order int32) (db.FormField, error) {
	opts, _ := marshalOptions(req.Options)
	val, _ := marshalValidation(req.Validation)
	cond, _ := marshalConditional(req.Conditional)
	scope, _ := marshalScope(req.CategoryScope)
	return s.repo.CreateField(ctx, db.CreateFieldParams{
		OrganizationID: orgID, FormSchemaID: schemaID,
		FieldType: req.FieldType, Label: req.Label, FieldKey: req.FieldKey,
		HelpText: pgText(req.HelpText), IsRequired: req.IsRequired, DisplayOrder: order,
		Options: opts, Validation: val, Conditional: cond, CategoryScope: scope,
	})
}

func (s *Service) buildFormResponse(ctx context.Context, form db.FormSchema) (FormResponse, error) {
	rows, err := s.repo.ListFieldsBySchema(ctx, form.ID)
	if err != nil {
		return FormResponse{}, err
	}
	fr := FormResponse{ID: form.ID, EventID: form.EventID, Name: form.Name, CreatedAt: form.CreatedAt.Time, Fields: []FieldResponse{}}
	for _, r := range rows {
		f, err := toFieldResponse(r)
		if err != nil {
			return FormResponse{}, err
		}
		fr.Fields = append(fr.Fields, f)
	}
	return fr, nil
}

func (s *Service) validateCategoryScope(ctx context.Context, eventID uuid.UUID, scope []string) error {
	for _, idStr := range scope {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return ErrCategoryNotInEvt
		}
		cat, err := s.repo.GetCategoryByID(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrCategoryNotInEvt
		} else if err != nil {
			return err
		}
		if cat.EventID != eventID {
			return ErrCategoryNotInEvt
		}
	}
	return nil
}

func (s *Service) assertCategory(ctx context.Context, eventID uuid.UUID, categoryID *uuid.UUID) error {
	if categoryID == nil {
		return nil
	}
	cat, err := s.repo.GetCategoryByID(ctx, *categoryID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCategoryNotInEvt
	} else if err != nil {
		return err
	}
	if cat.EventID != eventID {
		return ErrCategoryNotInEvt
	}
	return nil
}

// validateSet runs formschema.ValidateFields and maps its error to an apperr.
func validateSet(fields []formschema.Field) error {
	if err := formschema.ValidateFields(fields); err != nil {
		var ve *formschema.ValidationError
		if errors.As(err, &ve) {
			return apperr.New(statusForCode(ve.Code), ve.Code, ve.Message)
		}
		return err
	}
	return nil
}

func statusForCode(code string) int {
	switch code {
	case "CONDITIONAL_UNKNOWN_FIELD", "CONDITIONAL_CYCLE", "INVALID_CONDITIONAL",
		"INVALID_FIELD_KEY", "INVALID_FIELD_TYPE", "OPTIONS_REQUIRED", "OPTIONS_NOT_ALLOWED",
		"INVALID_VALIDATION_RULE":
		return 400
	case "DUPLICATE_FIELD_KEY":
		return 409
	}
	return 400
}

func toSchemaFields(rows []db.FormField) ([]formschema.Field, error) {
	out := make([]formschema.Field, 0, len(rows))
	for _, r := range rows {
		sf, err := toSchemaField(r)
		if err != nil {
			return nil, err
		}
		out = append(out, sf)
	}
	return out, nil
}

func (req FieldRequest) toSchemaField(order int) formschema.Field {
	return formschema.Field{
		Key: req.FieldKey, Type: formschema.FieldType(req.FieldType), Label: req.Label,
		Required: req.IsRequired, DisplayOrder: order, Options: req.Options,
		Validation: req.Validation, Conditional: req.Conditional, CategoryScope: req.CategoryScope,
	}
}

func conditionRefs(c *formschema.Condition, key string) bool {
	if c == nil {
		return false
	}
	if c.Field == key {
		return true
	}
	for i := range c.Rules {
		if conditionRefs(&c.Rules[i], key) {
			return true
		}
	}
	return false
}

func pgText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}
```
Note: `DUPLICATE_FIELD_KEY` returns 409 in `statusForCode` but `formschema` raises it during `ValidateFields`. The test `TestAddField_RejectsDuplicateKey` checks the code is `DUPLICATE_FIELD_KEY` — the status mapping is independent. Confirm `form.CreatedAt` is `pgtype.Timestamptz` (`.Time`).

- [ ] **Step 5: Run tests to verify they pass**

Run:
```bash
cd services/api && go test ./internal/modules/forms/ -v; cd ../..
```
Expected: PASS (all forms service tests).

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/modules/forms/mapping.go services/api/internal/modules/forms/service.go services/api/internal/modules/forms/service_test.go
git commit -m "feat(forms): add service with upsert, field CRUD, reorder, and preview"
```

---

## Task 8: Forms handler, routes, and wiring

**Files:**
- Create: `services/api/internal/modules/forms/handler.go`
- Create: `services/api/internal/modules/forms/routes.go`
- Modify: `services/api/internal/app/server.go`

- [ ] **Step 1: Handler**

Create `services/api/internal/modules/forms/handler.go`:
```go
package forms

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) ids(w http.ResponseWriter, r *http.Request) (orgID, eventID uuid.UUID, ok bool) {
	var err error
	orgID, err = uuid.Parse(chi.URLParam(r, "orgId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_ORG_ID", "invalid organization id"))
		return uuid.Nil, uuid.Nil, false
	}
	eventID, err = uuid.Parse(chi.URLParam(r, "eventId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_EVENT_ID", "invalid event id"))
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, eventID, true
}

func (h *Handler) categoryParam(r *http.Request) (*uuid.UUID, error) {
	q := r.URL.Query().Get("categoryId")
	if q == "" {
		return nil, nil
	}
	id, err := uuid.Parse(q)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (h *Handler) GetForm(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	form, err := h.svc.GetForm(r.Context(), orgID, eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, form)
}

func (h *Handler) UpdateForm(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	var req UpdateFormRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "malformed body"))
		return
	}
	form, err := h.svc.UpdateForm(r.Context(), orgID, eventID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, form)
}

func (h *Handler) ListFields(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	form, err := h.svc.GetForm(r.Context(), orgID, eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, form.Fields)
}

func (h *Handler) AddField(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	var req FieldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FieldKey == "" || req.FieldType == "" || req.Label == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "fieldType, label, and fieldKey are required"))
		return
	}
	field, err := h.svc.AddField(r.Context(), orgID, eventID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusCreated, field)
}

func (h *Handler) UpdateField(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	fieldID, err := uuid.Parse(chi.URLParam(r, "fieldId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_FIELD_ID", "invalid field id"))
		return
	}
	var req FieldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FieldKey == "" || req.FieldType == "" || req.Label == "" {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "fieldType, label, and fieldKey are required"))
		return
	}
	field, err := h.svc.UpdateField(r.Context(), orgID, eventID, fieldID, req)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, field)
}

func (h *Handler) DeleteField(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	fieldID, err := uuid.Parse(chi.URLParam(r, "fieldId"))
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_FIELD_ID", "invalid field id"))
		return
	}
	if err := h.svc.DeleteField(r.Context(), orgID, eventID, fieldID); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Reorder(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	var req ReorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "malformed body"))
		return
	}
	if err := h.svc.Reorder(r.Context(), orgID, eventID, req.FieldIDs); err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	form, err := h.svc.GetForm(r.Context(), orgID, eventID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, form)
}

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	catID, err := h.categoryParam(r)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CATEGORY_ID", "invalid category id"))
		return
	}
	fields, err := h.svc.Preview(r.Context(), orgID, eventID, catID)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, fields)
}

func (h *Handler) PreviewValidate(w http.ResponseWriter, r *http.Request) {
	orgID, eventID, ok := h.ids(w, r)
	if !ok {
		return
	}
	catID, err := h.categoryParam(r)
	if err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_CATEGORY_ID", "invalid category id"))
		return
	}
	var req PreviewValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperr.WriteError(w, r, apperr.New(http.StatusBadRequest, "INVALID_BODY", "malformed body"))
		return
	}
	res, err := h.svc.PreviewValidate(r.Context(), orgID, eventID, catID, req.Answers)
	if err != nil {
		apperr.WriteError(w, r, err)
		return
	}
	apperr.WriteJSON(w, http.StatusOK, res)
}
```

- [ ] **Step 2: Routes**

Create `services/api/internal/modules/forms/routes.go`:
```go
package forms

import (
	"github.com/go-chi/chi/v5"

	"github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

// RegisterRoutes mounts form-builder endpoints under an existing
// /organizations/{orgId}/events/{eventId} router (already behind authn).
// All routes require form.manage.
func (h *Handler) RegisterRoutes(r chi.Router, loader middleware.PermissionLoader) {
	r.Route("/form", func(r chi.Router) {
		r.Use(middleware.RequirePermission(loader, "form.manage"))
		r.Get("/", h.GetForm)
		r.Put("/", h.UpdateForm)
		r.Get("/fields", h.ListFields)
		r.Post("/fields", h.AddField)
		r.Put("/fields/reorder", h.Reorder)
		r.Put("/fields/{fieldId}", h.UpdateField)
		r.Delete("/fields/{fieldId}", h.DeleteField)
		r.Get("/preview", h.Preview)
		r.Post("/preview/validate", h.PreviewValidate)
	})
}
```
Note: register `/fields/reorder` BEFORE `/fields/{fieldId}` so chi matches the literal path first. chi actually prioritizes static over param segments, but ordering it explicitly avoids ambiguity.

- [ ] **Step 3: Wire into server.go**

In `services/api/internal/app/server.go`, add import `formsmod "github.com/varin/ivyticketing/services/api/internal/modules/forms"`, build the handler near the others:
```go
	formHandler := formsmod.NewHandler(formsmod.NewService(formsmod.NewRepository(pool)))
```
Mount it via the events `mountSubRoutes` callback, alongside categories:
```go
				eventHandler.RegisterRoutes(r, loader, func(r chi.Router) {
					categoryHandler.RegisterRoutes(r, loader)
					formHandler.RegisterRoutes(r, loader)
				})
```

- [ ] **Step 4: Build and full test**

Run:
```bash
cd services/api && go build ./... && go vet ./... && go test ./...; cd ../..
```
Expected: build OK; vet clean; all unit tests PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/modules/forms/handler.go services/api/internal/modules/forms/routes.go services/api/internal/app/server.go
git commit -m "feat(forms): add handler, routes, and wire into router"
```

---
