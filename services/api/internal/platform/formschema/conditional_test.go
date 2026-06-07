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
