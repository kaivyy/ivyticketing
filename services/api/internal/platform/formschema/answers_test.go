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
