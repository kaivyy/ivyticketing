package formschema

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// FieldError is a per-field validation failure for preview.
type FieldError struct {
	FieldKey string `json:"fieldKey"`
	Message  string `json:"message"`
}

// inScope reports whether a field applies to the given category (nil category
// means "no category filter" — only fields with no scope apply).
func inScope(f Field, categoryID *uuid.UUID) bool {
	if len(f.CategoryScope) == 0 {
		return true // applies to all categories
	}
	if categoryID == nil {
		return false // scoped field, but no category context
	}
	idStr := categoryID.String()
	for _, id := range f.CategoryScope {
		if id == idStr {
			return true
		}
	}
	return false
}

// VisibleFields returns the fields that apply to the category AND pass their
// conditional given the answers, in display order.
func VisibleFields(fields []Field, answers map[string]any, categoryID *uuid.UUID) []Field {
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
func ValidateAnswers(fields []Field, answers map[string]any, categoryID *uuid.UUID) []FieldError {
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
