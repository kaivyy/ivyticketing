# Phase 4 Plan — Part 1: Foundation (Tasks 1-3)

> Part of the Phase 4 implementation plan. Index: [2026-06-07-phase4-form-builder.md](2026-06-07-phase4-form-builder.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

---

## Task 1: Database migrations

**Files:**
- Create: `database/migrations/00010_create_form_schemas.sql`
- Create: `database/migrations/00011_create_form_fields.sql`

- [ ] **Step 1: form_schemas migration**

Create `database/migrations/00010_create_form_schemas.sql`:
```sql
-- +goose Up
CREATE TABLE form_schemas (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_id        uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name            text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (event_id)
);
CREATE INDEX idx_form_schemas_event ON form_schemas(event_id);

-- +goose Down
DROP TABLE form_schemas;
```

- [ ] **Step 2: form_fields migration**

Create `database/migrations/00011_create_form_fields.sql`:
```sql
-- +goose Up
CREATE TABLE form_fields (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    form_schema_id  uuid NOT NULL REFERENCES form_schemas(id) ON DELETE CASCADE,
    field_type      text NOT NULL,
    label           text NOT NULL,
    field_key       text NOT NULL,
    help_text       text,
    is_required     boolean NOT NULL DEFAULT false,
    display_order   integer NOT NULL,
    options         jsonb,
    validation      jsonb,
    conditional     jsonb,
    category_scope  jsonb,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (form_schema_id, field_key)
);
CREATE INDEX idx_form_fields_schema ON form_fields(form_schema_id);
CREATE INDEX idx_form_fields_org ON form_fields(organization_id);

-- +goose Down
DROP TABLE form_fields;
```

- [ ] **Step 3: Apply and verify roundtrip**

Run:
```bash
make migrate-up
make migrate-down && make migrate-up
```
Expected: both migrations apply, roll back cleanly, and re-apply with no errors.

- [ ] **Step 4: Commit**

```bash
git add database/migrations
git commit -m "feat(db): add form_schemas and form_fields migrations"
```

---

## Task 2: sqlc queries + regenerate

**Files:**
- Create: `database/queries/forms.sql`
- Regenerate: `services/api/internal/db/*`

- [ ] **Step 1: Form queries**

Create `database/queries/forms.sql`:
```sql
-- name: GetFormSchemaByEvent :one
SELECT * FROM form_schemas WHERE event_id = $1;

-- name: CreateFormSchema :one
INSERT INTO form_schemas (organization_id, event_id, name)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateFormSchemaName :one
UPDATE form_schemas SET name = $2, updated_at = now()
WHERE id = $1 AND organization_id = $3
RETURNING *;

-- name: ListFieldsBySchema :many
SELECT * FROM form_fields WHERE form_schema_id = $1 ORDER BY display_order;

-- name: GetFieldByID :one
SELECT * FROM form_fields WHERE id = $1;

-- name: CreateField :one
INSERT INTO form_fields (organization_id, form_schema_id, field_type, label, field_key,
    help_text, is_required, display_order, options, validation, conditional, category_scope)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: UpdateField :one
UPDATE form_fields SET
    field_type = $2, label = $3, field_key = $4, help_text = $5, is_required = $6,
    options = $7, validation = $8, conditional = $9, category_scope = $10, updated_at = now()
WHERE id = $1 AND form_schema_id = $11
RETURNING *;

-- name: UpdateFieldOrder :exec
UPDATE form_fields SET display_order = $2, updated_at = now()
WHERE id = $1 AND form_schema_id = $3;

-- name: DeleteField :exec
DELETE FROM form_fields WHERE id = $1 AND form_schema_id = $2;

-- name: MaxFieldOrder :one
SELECT COALESCE(MAX(display_order), 0)::int FROM form_fields WHERE form_schema_id = $1;
```

- [ ] **Step 2: Regenerate and verify build**

Run:
```bash
make sqlc
cd services/api && go build ./...; cd ../..
```
Expected: `sqlc generate` succeeds; new `forms.sql.go` and `FormSchema`/`FormField` structs in `models.go`. Build passes.

- [ ] **Step 3: Inspect generated types**

Run:
```bash
sed -n '/type FormSchema struct/,/^}/p;/type FormField struct/,/^}/p' services/api/internal/db/models.go
grep -A 16 "type CreateFieldParams" services/api/internal/db/forms.sql.go
```
Note the field types: jsonb columns (`Options`, `Validation`, `Conditional`, `CategoryScope`) should be `[]byte`; `HelpText` is `pgtype.Text`; `IsRequired` is `bool`; `DisplayOrder` is `int32`. Part 3 references these — adjust mapping if sqlc differs.

- [ ] **Step 4: Commit**

```bash
git add database/queries services/api/internal/db
git commit -m "feat(db): add forms sqlc queries and regenerate"
```

---

## Task 3: formschema field types + definition validation

**Files:**
- Create: `services/api/internal/platform/formschema/field.go`
- Create: `services/api/internal/platform/formschema/validate.go`
- Test: `services/api/internal/platform/formschema/validate_test.go`

This package is pure (no DB, no HTTP). It defines the canonical `Field` type used across
the service and the validation rules for field definitions. Conditional logic lives in
Part 2; `validate.go` calls into it (`validateConditional`), which Part 2 implements — so
in this task, stub `validateConditional` to accept any non-nil tree as valid, and Part 2
replaces it with the real validator. **To keep Task 3 self-contained and compiling, include
a minimal `conditional.go` with just the types + a permissive `validateConditional`**, then
Part 2 Task 4 fully implements parsing/validation/evaluation.

- [ ] **Step 1: Field types**

Create `services/api/internal/platform/formschema/field.go`:
```go
package formschema

// FieldType is the kind of a form field.
type FieldType string

const (
	TypeText     FieldType = "text"
	TypeEmail    FieldType = "email"
	TypePhone    FieldType = "phone"
	TypeNumber   FieldType = "number"
	TypeDate     FieldType = "date"
	TypeDropdown FieldType = "dropdown"
	TypeRadio    FieldType = "radio"
	TypeCheckbox FieldType = "checkbox"
	TypeTextarea FieldType = "textarea"
	TypeFile     FieldType = "file"
)

var validTypes = map[FieldType]bool{
	TypeText: true, TypeEmail: true, TypePhone: true, TypeNumber: true, TypeDate: true,
	TypeDropdown: true, TypeRadio: true, TypeCheckbox: true, TypeTextarea: true, TypeFile: true,
}

func (t FieldType) needsOptions() bool {
	return t == TypeDropdown || t == TypeRadio || t == TypeCheckbox
}

func (t FieldType) isNumeric() bool {
	return t == TypeNumber || t == TypeDate
}

func (t FieldType) isText() bool {
	return t == TypeText || t == TypeTextarea || t == TypeEmail || t == TypePhone
}

// Validation holds per-field validation rules. Nil pointers mean "not set".
type Validation struct {
	MinLength *int    `json:"minLength,omitempty"`
	MaxLength *int    `json:"maxLength,omitempty"`
	Pattern   *string `json:"pattern,omitempty"`
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
}

// Field is the canonical in-memory representation of a form field definition.
// The service builds these from DB rows (unmarshalling jsonb) before validating.
type Field struct {
	Key          string       `json:"fieldKey"`
	Type         FieldType    `json:"fieldType"`
	Label        string       `json:"label"`
	Required     bool         `json:"isRequired"`
	DisplayOrder int          `json:"displayOrder"`
	Options      []string     `json:"options,omitempty"`
	Validation   *Validation  `json:"validation,omitempty"`
	Conditional  *Condition   `json:"conditional,omitempty"`
	CategoryScope []string    `json:"categoryScope,omitempty"`
}
```
Note: `Condition` is defined in `conditional.go` (Step 3 below — minimal version here, full in Part 2).

- [ ] **Step 2: Write failing tests for definition validation**

Create `services/api/internal/platform/formschema/validate_test.go`:
```go
package formschema

import "testing"

func f(key string, t FieldType, order int) Field {
	return Field{Key: key, Type: t, Label: key, DisplayOrder: order}
}

func TestValidateFields_OK(t *testing.T) {
	fields := []Field{
		f("nama", TypeText, 1),
		func() Field { x := f("gender", TypeDropdown, 2); x.Options = []string{"Pria", "Wanita"}; return x }(),
	}
	if err := ValidateFields(fields); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestValidateFields_RejectsBadKey(t *testing.T) {
	fields := []Field{f("Nama Lengkap", TypeText, 1)} // spaces, uppercase
	if err := ValidateFields(fields); err == nil {
		t.Fatal("expected invalid key error")
	}
}

func TestValidateFields_RejectsDuplicateKey(t *testing.T) {
	fields := []Field{f("nama", TypeText, 1), f("nama", TypeEmail, 2)}
	if err := ValidateFields(fields); err == nil {
		t.Fatal("expected duplicate key error")
	}
}

func TestValidateFields_RejectsUnknownType(t *testing.T) {
	fields := []Field{f("x", FieldType("rocket"), 1)}
	if err := ValidateFields(fields); err == nil {
		t.Fatal("expected unknown type error")
	}
}

func TestValidateFields_OptionsRequiredForDropdown(t *testing.T) {
	fields := []Field{f("g", TypeDropdown, 1)} // no options
	if err := ValidateFields(fields); err == nil {
		t.Fatal("expected options-required error")
	}
}

func TestValidateFields_OptionsNotAllowedForText(t *testing.T) {
	x := f("nama", TypeText, 1)
	x.Options = []string{"a"}
	if err := ValidateFields([]Field{x}); err == nil {
		t.Fatal("expected options-not-allowed error")
	}
}

func TestValidateFields_ValidationRuleMustMatchType(t *testing.T) {
	x := f("umur", TypeNumber, 1)
	ml := 3
	x.Validation = &Validation{MinLength: &ml} // minLength on number → invalid
	if err := ValidateFields([]Field{x}); err == nil {
		t.Fatal("expected invalid validation rule error")
	}
}
```

- [ ] **Step 3: Minimal conditional types (full impl in Part 2)**

Create `services/api/internal/platform/formschema/conditional.go`:
```go
package formschema

// Condition is a node in the AND/OR conditional tree. A node is either a group
// (Op is "and"/"or" with Rules) or a leaf (Field+Op+Value). Full parsing,
// validation, and evaluation are implemented in Part 2.
type Condition struct {
	Op    string      `json:"op"`
	Rules []Condition `json:"rules,omitempty"`
	Field string      `json:"field,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

// validateConditional is replaced with the full implementation in Part 2.
// For now it accepts any tree (permissive) so Task 3 compiles and tests pass.
func validateConditional(_ *Condition, _ map[string]Field, _ int) error {
	return nil
}
```
Note: Part 2 replaces this `validateConditional` body and adds `Evaluate`. Keep the signature stable: `validateConditional(cond *Condition, byKey map[string]Field, selfOrder int) error`.

- [ ] **Step 4: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/platform/formschema/ -v; cd ../..
```
Expected: FAIL — `undefined: ValidateFields`.

- [ ] **Step 5: Implement definition validation**

Create `services/api/internal/platform/formschema/validate.go`:
```go
package formschema

import (
	"fmt"
	"regexp"
)

var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ValidationError carries a machine-readable code and a human message.
type ValidationError struct {
	Code    string
	Message string
}

func (e *ValidationError) Error() string { return e.Code + ": " + e.Message }

func errf(code, format string, args ...any) *ValidationError {
	return &ValidationError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// ValidateFields validates the entire set of fields for a form. It checks key
// uniqueness/format, type allowlist, options presence rules, validation-rule
// type compatibility, and conditional tree validity (acyclic, known refs).
func ValidateFields(fields []Field) error {
	byKey := make(map[string]Field, len(fields))
	for _, fld := range fields {
		if !keyPattern.MatchString(fld.Key) {
			return errf("INVALID_FIELD_KEY", "field key %q must be snake_case", fld.Key)
		}
		if _, dup := byKey[fld.Key]; dup {
			return errf("DUPLICATE_FIELD_KEY", "duplicate field key %q", fld.Key)
		}
		byKey[fld.Key] = fld
	}

	for _, fld := range fields {
		if !validTypes[fld.Type] {
			return errf("INVALID_FIELD_TYPE", "unknown field type %q", fld.Type)
		}
		if fld.Type.needsOptions() {
			if len(fld.Options) == 0 {
				return errf("OPTIONS_REQUIRED", "field %q requires options", fld.Key)
			}
		} else if len(fld.Options) > 0 {
			return errf("OPTIONS_NOT_ALLOWED", "field %q must not have options", fld.Key)
		}
		if err := validateRules(fld); err != nil {
			return err
		}
		if fld.Conditional != nil {
			if err := validateConditional(fld.Conditional, byKey, fld.DisplayOrder); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateRules(fld Field) error {
	v := fld.Validation
	if v == nil {
		return nil
	}
	hasText := v.MinLength != nil || v.MaxLength != nil || v.Pattern != nil
	hasNum := v.Min != nil || v.Max != nil
	if hasText && !fld.Type.isText() {
		return errf("INVALID_VALIDATION_RULE", "field %q: text rules not allowed for type %s", fld.Key, fld.Type)
	}
	if hasNum && !fld.Type.isNumeric() {
		return errf("INVALID_VALIDATION_RULE", "field %q: min/max not allowed for type %s", fld.Key, fld.Type)
	}
	if v.Pattern != nil {
		if _, err := regexp.Compile(*v.Pattern); err != nil {
			return errf("INVALID_VALIDATION_RULE", "field %q: invalid pattern: %v", fld.Key, err)
		}
	}
	return nil
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run:
```bash
cd services/api && go test ./internal/platform/formschema/ -v; cd ../..
```
Expected: PASS (all validate tests).

- [ ] **Step 7: Commit**

```bash
git add services/api/internal/platform/formschema
git commit -m "feat(formschema): add field types and definition validation"
```

---
