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
	visible := formschema.VisibleFields(fields, map[string]any{}, categoryID)
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
	errs := formschema.ValidateAnswers(fields, answers, categoryID)
	visible := formschema.VisibleFields(fields, answers, categoryID)
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
