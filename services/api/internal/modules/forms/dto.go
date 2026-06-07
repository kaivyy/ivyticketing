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
