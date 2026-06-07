# Phase 4 Plan — Part 2: formschema Logic (Tasks 4-5)

> Part of the Phase 4 implementation plan. Index: [2026-06-07-phase4-form-builder.md](2026-06-07-phase4-form-builder.md)
> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> **Depends on:** Part 1 (`formschema` package with `Field`, `Condition` types, permissive `validateConditional` stub).

---

## Task 4: Conditional AND/OR validation + evaluation

This replaces the permissive `validateConditional` stub from Part 1 with the full
implementation, and adds `Evaluate`. The `Condition` type already exists in
`conditional.go` (Part 1) — extend that file.

**Files:**
- Modify: `services/api/internal/platform/formschema/conditional.go`
- Test: `services/api/internal/platform/formschema/conditional_test.go`

- [ ] **Step 1: Write the failing tests**

Create `services/api/internal/platform/formschema/conditional_test.go`:
```go
package formschema

import "testing"

// build a field map for validation tests
func fieldMap(fields ...Field) map[string]Field {
	m := make(map[string]Field, len(fields))
	for _, f := range fields {
		m[f.Key] = f
	}
	return m
}

func leaf(field, op string, value any) Condition {
	return Condition{Field: field, Op: op, Value: value}
}

func group(op string, rules ...Condition) Condition {
	return Condition{Op: op, Rules: rules}
}

func TestValidateConditional_OK(t *testing.T) {
	byKey := fieldMap(
		Field{Key: "wna", Type: TypeDropdown, DisplayOrder: 1},
		Field{Key: "umur", Type: TypeNumber, DisplayOrder: 2},
	)
	cond := group("and",
		leaf("wna", "equals", "Ya"),
		leaf("umur", "gte", float64(17)),
	)
	if err := validateConditional(&cond, byKey, 3); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestValidateConditional_UnknownField(t *testing.T) {
	byKey := fieldMap(Field{Key: "wna", Type: TypeText, DisplayOrder: 1})
	cond := leaf("ghost", "equals", "x")
	if err := validateConditional(&cond, byKey, 2); err == nil {
		t.Fatal("expected unknown-field error")
	}
}

func TestValidateConditional_ForwardReference(t *testing.T) {
	// referenced field has a LATER display order → cycle/forward ref → reject
	byKey := fieldMap(Field{Key: "later", Type: TypeText, DisplayOrder: 5})
	cond := leaf("later", "equals", "x")
	if err := validateConditional(&cond, byKey, 2); err == nil {
		t.Fatal("expected forward-reference error")
	}
}

func TestValidateConditional_NumericOpOnTextField(t *testing.T) {
	byKey := fieldMap(Field{Key: "name", Type: TypeText, DisplayOrder: 1})
	cond := leaf("name", "gte", float64(3))
	if err := validateConditional(&cond, byKey, 2); err == nil {
		t.Fatal("expected numeric-op-on-text error")
	}
}

func TestValidateConditional_InRequiresArray(t *testing.T) {
	byKey := fieldMap(Field{Key: "g", Type: TypeDropdown, DisplayOrder: 1})
	cond := leaf("g", "in", "not-an-array")
	if err := validateConditional(&cond, byKey, 2); err == nil {
		t.Fatal("expected in-requires-array error")
	}
}

func TestValidateConditional_DepthLimit(t *testing.T) {
	byKey := fieldMap(Field{Key: "a", Type: TypeText, DisplayOrder: 1})
	// nest 4 levels deep (limit is 3)
	deep := group("and", group("and", group("and", group("and", leaf("a", "equals", "x")))))
	if err := validateConditional(&deep, byKey, 2); err == nil {
		t.Fatal("expected depth-limit error")
	}
}

func TestValidateConditional_EmptyGroup(t *testing.T) {
	byKey := fieldMap(Field{Key: "a", Type: TypeText, DisplayOrder: 1})
	cond := Condition{Op: "and"} // no rules
	if err := validateConditional(&cond, byKey, 2); err == nil {
		t.Fatal("expected empty-group error")
	}
}

func TestEvaluate_AndOr(t *testing.T) {
	cond := group("and",
		leaf("wna", "equals", "Ya"),
		group("or",
			leaf("umur", "gte", float64(17)),
			leaf("wali", "equals", "Ya"),
		),
	)
	cases := []struct {
		name    string
		answers map[string]any
		want    bool
	}{
		{"all true via umur", map[string]any{"wna": "Ya", "umur": float64(20)}, true},
		{"true via wali", map[string]any{"wna": "Ya", "umur": float64(10), "wali": "Ya"}, true},
		{"false wna", map[string]any{"wna": "Tidak", "umur": float64(20)}, false},
		{"false both branches", map[string]any{"wna": "Ya", "umur": float64(10)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Evaluate(&cond, tc.answers); got != tc.want {
				t.Errorf("Evaluate = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEvaluate_Operators(t *testing.T) {
	answers := map[string]any{"n": float64(5), "s": "hello"}
	checks := []struct {
		cond Condition
		want bool
	}{
		{leaf("n", "gt", float64(4)), true},
		{leaf("n", "lt", float64(4)), false},
		{leaf("n", "lte", float64(5)), true},
		{leaf("s", "equals", "hello"), true},
		{leaf("s", "notEquals", "bye"), true},
		{leaf("s", "in", []any{"hello", "world"}), true},
		{leaf("s", "notIn", []any{"x", "y"}), true},
		{leaf("missing", "equals", "x"), false}, // unanswered → false
	}
	for i, c := range checks {
		if got := Evaluate(&c.cond, answers); got != c.want {
			t.Errorf("case %d: Evaluate = %v, want %v", i, got, c.want)
		}
	}
}

func TestEvaluate_NilShowsField(t *testing.T) {
	// no conditional means always visible — Evaluate(nil, ...) returns true
	if !Evaluate(nil, map[string]any{}) {
		t.Error("nil conditional should evaluate true (always visible)")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/platform/formschema/ -run TestValidateConditional -v; cd ../..
```
Expected: FAIL — tests expect rejections but the permissive stub accepts everything; `Evaluate` undefined.

- [ ] **Step 3: Implement the full conditional logic**

Replace the entire contents of `services/api/internal/platform/formschema/conditional.go`:
```go
package formschema

const (
	maxDepth     = 3
	maxLeafCount = 20
)

// Condition is a node in the AND/OR conditional tree. A node is either a group
// (Op is "and"/"or" with Rules) or a leaf (Field+Op+Value).
type Condition struct {
	Op    string      `json:"op"`
	Rules []Condition `json:"rules,omitempty"`
	Field string      `json:"field,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

func (c *Condition) isGroup() bool {
	return c.Op == "and" || c.Op == "or"
}

var leafOps = map[string]bool{
	"equals": true, "notEquals": true, "in": true, "notIn": true,
	"gt": true, "gte": true, "lt": true, "lte": true,
}

// validateConditional checks the tree is well-formed, references only known
// fields with a SMALLER display order (acyclic, no forward refs), uses valid
// operators per field type, and respects depth/leaf limits. selfOrder is the
// display order of the field that owns this conditional.
func validateConditional(cond *Condition, byKey map[string]Field, selfOrder int) error {
	if cond == nil {
		return nil
	}
	leaves := 0
	if err := walk(cond, byKey, selfOrder, 1, &leaves); err != nil {
		return err
	}
	return nil
}

func walk(c *Condition, byKey map[string]Field, selfOrder, depth int, leaves *int) error {
	if c.isGroup() {
		if depth > maxDepth {
			return errf("INVALID_CONDITIONAL", "conditional nesting exceeds depth %d", maxDepth)
		}
		if len(c.Rules) == 0 {
			return errf("INVALID_CONDITIONAL", "group %q must have rules", c.Op)
		}
		for i := range c.Rules {
			if err := walk(&c.Rules[i], byKey, selfOrder, depth+1, leaves); err != nil {
				return err
			}
		}
		return nil
	}
	// leaf
	*leaves++
	if *leaves > maxLeafCount {
		return errf("INVALID_CONDITIONAL", "conditional exceeds %d leaf rules", maxLeafCount)
	}
	if !leafOps[c.Op] {
		return errf("INVALID_CONDITIONAL", "unknown operator %q", c.Op)
	}
	ref, ok := byKey[c.Field]
	if !ok {
		return errf("CONDITIONAL_UNKNOWN_FIELD", "conditional references unknown field %q", c.Field)
	}
	if ref.DisplayOrder >= selfOrder {
		return errf("CONDITIONAL_CYCLE", "conditional may only reference earlier fields (%q)", c.Field)
	}
	if (c.Op == "gt" || c.Op == "gte" || c.Op == "lt" || c.Op == "lte") && !ref.Type.isNumeric() {
		return errf("INVALID_CONDITIONAL", "operator %q requires a numeric/date field, got %s", c.Op, ref.Type)
	}
	if c.Op == "in" || c.Op == "notIn" {
		if _, isArr := c.Value.([]any); !isArr {
			// also accept []interface{} from JSON unmarshalling
			if _, isArr2 := c.Value.([]interface{}); !isArr2 {
				return errf("INVALID_CONDITIONAL", "operator %q requires an array value", c.Op)
			}
		}
	}
	return nil
}

// Evaluate returns whether a field with this conditional should be visible given
// the answers. A nil conditional means always visible.
func Evaluate(cond *Condition, answers map[string]any) bool {
	if cond == nil {
		return true
	}
	if cond.isGroup() {
		if cond.Op == "and" {
			for i := range cond.Rules {
				if !Evaluate(&cond.Rules[i], answers) {
					return false
				}
			}
			return true
		}
		// or
		for i := range cond.Rules {
			if Evaluate(&cond.Rules[i], answers) {
				return true
			}
		}
		return false
	}
	return evalLeaf(cond, answers)
}

func evalLeaf(c *Condition, answers map[string]any) bool {
	got, ok := answers[c.Field]
	if !ok {
		return false
	}
	switch c.Op {
	case "equals":
		return equalsVal(got, c.Value)
	case "notEquals":
		return !equalsVal(got, c.Value)
	case "in":
		return inList(got, c.Value)
	case "notIn":
		return !inList(got, c.Value)
	case "gt", "gte", "lt", "lte":
		return compareNum(got, c.Value, c.Op)
	}
	return false
}

func equalsVal(a, b any) bool {
	if an, aok := toFloat(a); aok {
		if bn, bok := toFloat(b); bok {
			return an == bn
		}
	}
	return toStr(a) == toStr(b)
}

func inList(got, list any) bool {
	arr, ok := asArray(list)
	if !ok {
		return false
	}
	for _, item := range arr {
		if equalsVal(got, item) {
			return true
		}
	}
	return false
}

func compareNum(a, b any, op string) bool {
	an, aok := toFloat(a)
	bn, bok := toFloat(b)
	if !aok || !bok {
		return false
	}
	switch op {
	case "gt":
		return an > bn
	case "gte":
		return an >= bn
	case "lt":
		return an < bn
	case "lte":
		return an <= bn
	}
	return false
}

func asArray(v any) ([]any, bool) {
	if a, ok := v.([]any); ok {
		return a, true
	}
	if a, ok := v.([]interface{}); ok {
		return a, true
	}
	return nil, false
}
```
Note: `toFloat`, `toStr` helpers are defined in `answers.go` (Task 5). If Task 5 hasn't been written yet when this compiles, add them temporarily here and move them in Task 5 — OR write them now in a small `coerce.go`. **Cleanest: create `coerce.go` now** (Step 4).

- [ ] **Step 4: Add coercion helpers**

Create `services/api/internal/platform/formschema/coerce.go`:
```go
package formschema

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// toFloat coerces a value (JSON number, int, string-number) to float64.
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	}
	return 0, false
}

// toStr coerces a value to its string form for equality comparison.
func toStr(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", s)
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run:
```bash
cd services/api && go test ./internal/platform/formschema/ -v; cd ../..
```
Expected: PASS (conditional + validate tests). The Part 1 validate tests still pass since `validateConditional` now does real work but the valid fixtures remain valid.

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/platform/formschema/conditional.go \
  services/api/internal/platform/formschema/conditional_test.go \
  services/api/internal/platform/formschema/coerce.go
git commit -m "feat(formschema): implement AND/OR conditional validation and evaluation"
```

---

## Task 5: ValidateAnswers (preview/dry-run)

**Files:**
- Create: `services/api/internal/platform/formschema/answers.go`
- Test: `services/api/internal/platform/formschema/answers_test.go`

`ValidateAnswers` filters fields by category, evaluates conditionals to find visible
fields, then enforces required + validation rules on visible fields only. Used by the
preview endpoint — no persistence.

- [ ] **Step 1: Write the failing tests**

Create `services/api/internal/platform/formschema/answers_test.go`:
```go
package formschema

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateAnswers_RequiredVisibleField(t *testing.T) {
	fields := []Field{
		{Key: "wna", Type: TypeDropdown, DisplayOrder: 1, Options: []string{"Ya", "Tidak"}},
		{Key: "passport", Type: TypeText, DisplayOrder: 2, Required: true,
			Conditional: &Condition{Field: "wna", Op: "equals", Value: "Ya"}},
	}
	// wna=Ya → passport visible & required → missing → error
	errs := ValidateAnswers(fields, map[string]any{"wna": "Ya"}, nil)
	if len(errs) != 1 || errs[0].FieldKey != "passport" {
		t.Fatalf("expected passport required error, got %+v", errs)
	}
}

func TestValidateAnswers_HiddenFieldSkipped(t *testing.T) {
	fields := []Field{
		{Key: "wna", Type: TypeDropdown, DisplayOrder: 1, Options: []string{"Ya", "Tidak"}},
		{Key: "passport", Type: TypeText, DisplayOrder: 2, Required: true,
			Conditional: &Condition{Field: "wna", Op: "equals", Value: "Ya"}},
	}
	// wna=Tidak → passport hidden → not required → no error
	errs := ValidateAnswers(fields, map[string]any{"wna": "Tidak"}, nil)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %+v", errs)
	}
}

func TestValidateAnswers_MinLength(t *testing.T) {
	ml := 5
	fields := []Field{
		{Key: "nama", Type: TypeText, DisplayOrder: 1, Required: true, Validation: &Validation{MinLength: &ml}},
	}
	errs := ValidateAnswers(fields, map[string]any{"nama": "abc"}, nil)
	if len(errs) != 1 || errs[0].FieldKey != "nama" {
		t.Fatalf("expected minLength error, got %+v", errs)
	}
}

func TestValidateAnswers_CategoryScopeFilters(t *testing.T) {
	cat42 := uuid.New()
	cat10 := uuid.New()
	fields := []Field{
		{Key: "nama", Type: TypeText, DisplayOrder: 1, Required: true},
		{Key: "jersey", Type: TypeText, DisplayOrder: 2, Required: true, CategoryScope: []string{cat42.String()}},
	}
	// previewing cat10 → jersey is out of scope → not required → only nama matters
	errs := ValidateAnswers(fields, map[string]any{"nama": "Budi"}, &cat10)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for cat10, got %+v", errs)
	}
	// previewing cat42 → jersey in scope & required & missing → error
	errs = ValidateAnswers(fields, map[string]any{"nama": "Budi"}, &cat42)
	if len(errs) != 1 || errs[0].FieldKey != "jersey" {
		t.Fatalf("expected jersey required for cat42, got %+v", errs)
	}
}

func TestVisibleFields_FiltersByCategoryAndConditional(t *testing.T) {
	cat := uuid.New()
	fields := []Field{
		{Key: "a", Type: TypeText, DisplayOrder: 1},
		{Key: "b", Type: TypeText, DisplayOrder: 2, CategoryScope: []string{cat.String()}},
	}
	vis := VisibleFields(fields, map[string]any{}, nil) // no category → category-scoped b excluded
	if len(vis) != 1 || vis[0].Key != "a" {
		t.Fatalf("expected only a visible, got %+v", vis)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/platform/formschema/ -run 'TestValidateAnswers|TestVisibleFields' -v; cd ../..
```
Expected: FAIL — `undefined: ValidateAnswers`, `undefined: VisibleFields`.

- [ ] **Step 3: Implement answers validation**

Create `services/api/internal/platform/formschema/answers.go`:
```go
package formschema

import (
	"fmt"
	"regexp"
	"strings"
)

// FieldError is a per-field validation failure for preview.
type FieldError struct {
	FieldKey string `json:"fieldKey"`
	Message  string `json:"message"`
}

// inScope reports whether a field applies to the given category (nil category
// means "no category filter" — only fields with no scope apply).
func inScope(f Field, categoryID *string) bool {
	if len(f.CategoryScope) == 0 {
		return true // applies to all categories
	}
	if categoryID == nil {
		return false // scoped field, but no category context
	}
	for _, id := range f.CategoryScope {
		if id == *categoryID {
			return true
		}
	}
	return false
}

// VisibleFields returns the fields that apply to the category AND pass their
// conditional given the answers, in display order.
func VisibleFields(fields []Field, answers map[string]any, categoryID *string) []Field {
	var out []Field
	for _, f := range fields {
		if !inScope(f, categoryID) {
			continue
		}
		if !Evaluate(f.Conditional, answers) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// ValidateAnswers validates answers against the visible fields for a category.
// Returns one FieldError per failure; empty slice means valid.
func ValidateAnswers(fields []Field, answers map[string]any, categoryID *string) []FieldError {
	var errs []FieldError
	for _, f := range VisibleFields(fields, answers, categoryID) {
		raw, present := answers[f.Key]
		empty := !present || isEmpty(raw)
		if f.Required && empty {
			errs = append(errs, FieldError{FieldKey: f.Key, Message: "this field is required"})
			continue
		}
		if empty {
			continue // optional & empty → skip rule checks
		}
		if e := checkRules(f, raw); e != nil {
			errs = append(errs, *e)
		}
	}
	return errs
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s) == ""
	}
	if a, ok := asArray(v); ok {
		return len(a) == 0
	}
	return false
}

func checkRules(f Field, raw any) *FieldError {
	v := f.Validation
	if v == nil {
		return nil
	}
	if f.Type.isText() {
		s := toStr(raw)
		if v.MinLength != nil && len(s) < *v.MinLength {
			return &FieldError{f.Key, fmt.Sprintf("must be at least %d characters", *v.MinLength)}
		}
		if v.MaxLength != nil && len(s) > *v.MaxLength {
			return &FieldError{f.Key, fmt.Sprintf("must be at most %d characters", *v.MaxLength)}
		}
		if v.Pattern != nil {
			if ok, _ := regexp.MatchString(*v.Pattern, s); !ok {
				return &FieldError{f.Key, "invalid format"}
			}
		}
	}
	if f.Type.isNumeric() {
		n, ok := toFloat(raw)
		if !ok {
			return &FieldError{f.Key, "must be a number"}
		}
		if v.Min != nil && n < *v.Min {
			return &FieldError{f.Key, fmt.Sprintf("must be >= %v", *v.Min)}
		}
		if v.Max != nil && n > *v.Max {
			return &FieldError{f.Key, fmt.Sprintf("must be <= %v", *v.Max)}
		}
	}
	return nil
}
```
Note: `FieldError{f.Key, "..."}` uses positional fields — matches the struct `{FieldKey, Message}`. The test references `errs[0].FieldKey`.

- [ ] **Step 4: Run tests to verify they pass**

Run:
```bash
cd services/api && go test ./internal/platform/formschema/ -v; cd ../..
```
Expected: PASS (all formschema tests — validate, conditional, answers).

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/platform/formschema/answers.go services/api/internal/platform/formschema/answers_test.go
git commit -m "feat(formschema): add answer validation and visible-field filtering for preview"
```

---
