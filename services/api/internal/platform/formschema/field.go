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
	MinLength *int     `json:"minLength,omitempty"`
	MaxLength *int     `json:"maxLength,omitempty"`
	Pattern   *string  `json:"pattern,omitempty"`
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
}

// Field is the canonical in-memory representation of a form field definition.
// The service builds these from DB rows (unmarshalling jsonb) before validating.
type Field struct {
	Key           string      `json:"fieldKey"`
	Type          FieldType   `json:"fieldType"`
	Label         string      `json:"label"`
	Required      bool        `json:"isRequired"`
	DisplayOrder  int         `json:"displayOrder"`
	Options       []string    `json:"options,omitempty"`
	Validation    *Validation `json:"validation,omitempty"`
	Conditional   *Condition  `json:"conditional,omitempty"`
	CategoryScope []string    `json:"categoryScope,omitempty"`
}
