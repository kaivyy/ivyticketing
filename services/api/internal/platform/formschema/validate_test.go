package formschema

import "testing"

func f(key string, t FieldType, order int) Field {
	return Field{Key: key, Type: t, Label: key, DisplayOrder: order}
}

func TestValidateFields_OK(t *testing.T) {
	fields := []Field{
		f("nama", TypeText, 1),
		func() Field { x := f("gender", TypeDropdown, 2); x.Options = []string{"Pria", "Wanita"}; return x }(),
	}
	if err := ValidateFields(fields); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestValidateFields_RejectsBadKey(t *testing.T) {
	fields := []Field{f("Nama Lengkap", TypeText, 1)} // spaces, uppercase
	if err := ValidateFields(fields); err == nil {
		t.Fatal("expected invalid key error")
	}
}

func TestValidateFields_RejectsDuplicateKey(t *testing.T) {
	fields := []Field{f("nama", TypeText, 1), f("nama", TypeEmail, 2)}
	if err := ValidateFields(fields); err == nil {
		t.Fatal("expected duplicate key error")
	}
}

func TestValidateFields_RejectsUnknownType(t *testing.T) {
	fields := []Field{f("x", FieldType("rocket"), 1)}
	if err := ValidateFields(fields); err == nil {
		t.Fatal("expected unknown type error")
	}
}

func TestValidateFields_OptionsRequiredForDropdown(t *testing.T) {
	fields := []Field{f("g", TypeDropdown, 1)} // no options
	if err := ValidateFields(fields); err == nil {
		t.Fatal("expected options-required error")
	}
}

func TestValidateFields_OptionsNotAllowedForText(t *testing.T) {
	x := f("nama", TypeText, 1)
	x.Options = []string{"a"}
	if err := ValidateFields([]Field{x}); err == nil {
		t.Fatal("expected options-not-allowed error")
	}
}

func TestValidateFields_ValidationRuleMustMatchType(t *testing.T) {
	x := f("umur", TypeNumber, 1)
	ml := 3
	x.Validation = &Validation{MinLength: &ml} // minLength on number → invalid
	if err := ValidateFields([]Field{x}); err == nil {
		t.Fatal("expected invalid validation rule error")
	}
}
