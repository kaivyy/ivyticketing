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

// marshalOptions marshals a string slice to []byte for a jsonb column, or nil if empty.
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
