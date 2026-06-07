package forms

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/varin/ivyticketing/services/api/internal/db"
	apperr "github.com/varin/ivyticketing/services/api/internal/platform/errors"
	"github.com/varin/ivyticketing/services/api/internal/platform/formschema"
)

type fakeRepo struct {
	events  map[uuid.UUID]db.Event
	schemas map[uuid.UUID]db.FormSchema // by event_id
	fields  map[uuid.UUID]db.FormField  // by field id
	nextOrd int32
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		events:  map[uuid.UUID]db.Event{},
		schemas: map[uuid.UUID]db.FormSchema{},
		fields:  map[uuid.UUID]db.FormField{},
	}
}

func (f *fakeRepo) seedEvent(orgID uuid.UUID) db.Event {
	e := db.Event{ID: uuid.New(), OrganizationID: orgID, Name: "E", Slug: "e", Status: "draft"}
	f.events[e.ID] = e
	return e
}

func (f *fakeRepo) ExecTx(ctx context.Context, fn func(Repository) error) error { return fn(f) }
func (f *fakeRepo) GetEventByID(_ context.Context, id uuid.UUID) (db.Event, error) {
	e, ok := f.events[id]
	if !ok {
		return db.Event{}, pgx.ErrNoRows
	}
	return e, nil
}
func (f *fakeRepo) GetCategoryByID(_ context.Context, id uuid.UUID) (db.EventCategory, error) {
	return db.EventCategory{}, pgx.ErrNoRows
}
func (f *fakeRepo) GetFormSchemaByEvent(_ context.Context, eventID uuid.UUID) (db.FormSchema, error) {
	s, ok := f.schemas[eventID]
	if !ok {
		return db.FormSchema{}, pgx.ErrNoRows
	}
	return s, nil
}
func (f *fakeRepo) CreateFormSchema(_ context.Context, arg db.CreateFormSchemaParams) (db.FormSchema, error) {
	s := db.FormSchema{ID: uuid.New(), OrganizationID: arg.OrganizationID, EventID: arg.EventID, Name: arg.Name}
	f.schemas[arg.EventID] = s
	return s, nil
}
func (f *fakeRepo) UpdateFormSchemaName(_ context.Context, arg db.UpdateFormSchemaNameParams) (db.FormSchema, error) {
	for eid, s := range f.schemas {
		if s.ID == arg.ID {
			s.Name = arg.Name
			f.schemas[eid] = s
			return s, nil
		}
	}
	return db.FormSchema{}, pgx.ErrNoRows
}
func (f *fakeRepo) ListFieldsBySchema(_ context.Context, schemaID uuid.UUID) ([]db.FormField, error) {
	var out []db.FormField
	for _, fld := range f.fields {
		if fld.FormSchemaID == schemaID {
			out = append(out, fld)
		}
	}
	// sort by display order
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].DisplayOrder < out[i].DisplayOrder {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}
func (f *fakeRepo) GetFieldByID(_ context.Context, id uuid.UUID) (db.FormField, error) {
	fld, ok := f.fields[id]
	if !ok {
		return db.FormField{}, pgx.ErrNoRows
	}
	return fld, nil
}
func (f *fakeRepo) CreateField(_ context.Context, arg db.CreateFieldParams) (db.FormField, error) {
	fld := db.FormField{
		ID: uuid.New(), OrganizationID: arg.OrganizationID, FormSchemaID: arg.FormSchemaID,
		FieldType: arg.FieldType, Label: arg.Label, FieldKey: arg.FieldKey,
		HelpText: arg.HelpText, IsRequired: arg.IsRequired, DisplayOrder: arg.DisplayOrder,
		Options: arg.Options, Validation: arg.Validation, Conditional: arg.Conditional, CategoryScope: arg.CategoryScope,
	}
	f.fields[fld.ID] = fld
	return fld, nil
}
func (f *fakeRepo) UpdateField(_ context.Context, arg db.UpdateFieldParams) (db.FormField, error) {
	fld := f.fields[arg.ID]
	fld.FieldType = arg.FieldType
	fld.Label = arg.Label
	fld.FieldKey = arg.FieldKey
	fld.HelpText = arg.HelpText
	fld.IsRequired = arg.IsRequired
	fld.Options = arg.Options
	fld.Validation = arg.Validation
	fld.Conditional = arg.Conditional
	fld.CategoryScope = arg.CategoryScope
	f.fields[arg.ID] = fld
	return fld, nil
}
func (f *fakeRepo) UpdateFieldOrder(_ context.Context, arg db.UpdateFieldOrderParams) error {
	fld := f.fields[arg.ID]
	fld.DisplayOrder = arg.DisplayOrder
	f.fields[arg.ID] = fld
	return nil
}
func (f *fakeRepo) DeleteField(_ context.Context, arg db.DeleteFieldParams) error {
	delete(f.fields, arg.ID)
	return nil
}
func (f *fakeRepo) MaxFieldOrder(_ context.Context, schemaID uuid.UUID) (int32, error) {
	var max int32
	for _, fld := range f.fields {
		if fld.FormSchemaID == schemaID && fld.DisplayOrder > max {
			max = fld.DisplayOrder
		}
	}
	return max, nil
}

func TestGetForm_AutoCreatesEmpty(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	ev := repo.seedEvent(orgID)

	form, err := svc.GetForm(context.Background(), orgID, ev.ID)
	if err != nil {
		t.Fatalf("get form: %v", err)
	}
	if form.EventID != ev.ID || len(form.Fields) != 0 {
		t.Fatalf("expected empty auto-created form, got %+v", form)
	}
}

func TestGetForm_TenantMismatch(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ev := repo.seedEvent(uuid.New())
	if _, err := svc.GetForm(context.Background(), uuid.New(), ev.ID); err != ErrEventNotFound {
		t.Fatalf("err = %v, want ErrEventNotFound", err)
	}
}

func TestAddField_ValidatesAndPersists(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	ev := repo.seedEvent(orgID)

	got, err := svc.AddField(context.Background(), orgID, ev.ID, FieldRequest{
		FieldType: "text", Label: "Nama", FieldKey: "nama", IsRequired: true,
	})
	if err != nil {
		t.Fatalf("add field: %v", err)
	}
	if got.FieldKey != "nama" {
		t.Errorf("fieldKey = %q", got.FieldKey)
	}
}

func TestAddField_RejectsDuplicateKey(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	ev := repo.seedEvent(orgID)
	if _, err := svc.AddField(context.Background(), orgID, ev.ID, FieldRequest{FieldType: "text", Label: "N", FieldKey: "nama"}); err != nil {
		t.Fatalf("first add: %v", err)
	}
	_, err := svc.AddField(context.Background(), orgID, ev.ID, FieldRequest{FieldType: "email", Label: "E", FieldKey: "nama"})
	if err == nil {
		t.Fatal("expected duplicate key error")
	}
	ae := apperrCode(err)
	if ae != "DUPLICATE_FIELD_KEY" {
		t.Fatalf("code = %q, want DUPLICATE_FIELD_KEY", ae)
	}
}

func TestAddField_RejectsCyclicConditional(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	ev := repo.seedEvent(orgID)
	// field referencing a non-existent earlier field
	_, err := svc.AddField(context.Background(), orgID, ev.ID, FieldRequest{
		FieldType: "text", Label: "P", FieldKey: "passport",
		Conditional: parseCondForTest(`{"field":"ghost","op":"equals","value":"x"}`),
	})
	if err == nil {
		t.Fatal("expected conditional unknown-field error")
	}
}

func TestReorder_RejectsMismatchedSet(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	ev := repo.seedEvent(orgID)
	svc.AddField(context.Background(), orgID, ev.ID, FieldRequest{FieldType: "text", Label: "A", FieldKey: "a"})
	// reorder with an unknown id
	if err := svc.Reorder(context.Background(), orgID, ev.ID, []uuid.UUID{uuid.New()}); err != ErrInvalidReorder {
		t.Fatalf("err = %v, want ErrInvalidReorder", err)
	}
}

func TestDeleteField_RejectsReferenced(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	orgID := uuid.New()
	ev := repo.seedEvent(orgID)
	a, _ := svc.AddField(context.Background(), orgID, ev.ID, FieldRequest{FieldType: "dropdown", Label: "WNA", FieldKey: "wna", Options: []string{"Ya", "Tidak"}})
	_, _ = svc.AddField(context.Background(), orgID, ev.ID, FieldRequest{
		FieldType: "text", Label: "Passport", FieldKey: "passport",
		Conditional: parseCondForTest(`{"field":"wna","op":"equals","value":"Ya"}`),
	})
	if err := svc.DeleteField(context.Background(), orgID, ev.ID, a.ID); err != ErrFieldReferenced {
		t.Fatalf("err = %v, want ErrFieldReferenced", err)
	}
}

// helpers
var _ = pgtype.Text{}

func apperrCode(err error) string {
	var ae *apperr.APIError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

func parseCondForTest(s string) *formschema.Condition {
	var c formschema.Condition
	_ = json.Unmarshal([]byte(s), &c)
	return &c
}
