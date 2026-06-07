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
