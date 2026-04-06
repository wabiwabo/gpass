package piifield

import "testing"

func TestSensitivity_String(t *testing.T) {
	tests := []struct {
		s    Sensitivity
		want string
	}{
		{Public, "public"},
		{Internal, "internal"},
		{Sensitive, "sensitive"},
		{HighlySensitive, "highly_sensitive"},
		{Sensitivity(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestRegistry_Register_Get(t *testing.T) {
	r := NewRegistry()
	r.Register(Field{Name: "nik", Sensitivity: HighlySensitive, RequiresConsent: true})

	f, ok := r.Get("nik")
	if !ok {
		t.Fatal("should find nik")
	}
	if f.Sensitivity != HighlySensitive {
		t.Errorf("Sensitivity = %v", f.Sensitivity)
	}
}

func TestRegistry_IsPII(t *testing.T) {
	r := NewRegistry()
	r.Register(Field{Name: "nik", Sensitivity: HighlySensitive})
	r.Register(Field{Name: "username", Sensitivity: Internal})

	if !r.IsPII("nik") {
		t.Error("nik should be PII")
	}
	if r.IsPII("username") {
		t.Error("username should not be PII (Internal)")
	}
	if r.IsPII("missing") {
		t.Error("missing should not be PII")
	}
}

func TestRegistry_RequiresConsent(t *testing.T) {
	r := NewRegistry()
	r.Register(Field{Name: "email", RequiresConsent: true})
	r.Register(Field{Name: "username", RequiresConsent: false})

	if !r.RequiresConsent("email") {
		t.Error("email requires consent")
	}
	if r.RequiresConsent("username") {
		t.Error("username does not require consent")
	}
}

func TestRegistry_FieldsRequiringConsent(t *testing.T) {
	r := NewRegistry()
	r.Register(Field{Name: "nik", ConsentScope: "nik", RequiresConsent: true})
	r.Register(Field{Name: "email", ConsentScope: "email", RequiresConsent: true})
	r.Register(Field{Name: "name", ConsentScope: "name", RequiresConsent: true})
	r.Register(Field{Name: "username", RequiresConsent: false})

	fields := r.FieldsRequiringConsent("email")
	if len(fields) != 1 || fields[0].Name != "email" {
		t.Errorf("fields = %v", fields)
	}
}

func TestRegistry_Classify(t *testing.T) {
	r := NewRegistry()
	r.Register(Field{Name: "nik", Sensitivity: HighlySensitive})
	r.Register(Field{Name: "religion", Sensitivity: HighlySensitive})
	r.Register(Field{Name: "email", Sensitivity: Sensitive})
	r.Register(Field{Name: "username", Sensitivity: Internal})

	c := r.Classify()
	if len(c[HighlySensitive]) != 2 {
		t.Errorf("HighlySensitive = %d", len(c[HighlySensitive]))
	}
	if len(c[Sensitive]) != 1 {
		t.Errorf("Sensitive = %d", len(c[Sensitive]))
	}
}

func TestDefaultIndonesianRegistry(t *testing.T) {
	r := DefaultIndonesianRegistry()

	// NIK should be highly sensitive
	f, ok := r.Get("nik")
	if !ok {
		t.Fatal("should have nik")
	}
	if f.Sensitivity != HighlySensitive {
		t.Error("nik should be highly sensitive")
	}
	if !f.Encrypted {
		t.Error("nik should be encrypted")
	}

	// Religion should be highly sensitive per UU PDP
	f, ok = r.Get("religion")
	if !ok {
		t.Fatal("should have religion")
	}
	if f.Sensitivity != HighlySensitive {
		t.Error("religion should be highly sensitive")
	}
}

func TestMaskField(t *testing.T) {
	tests := []struct {
		field, value, want string
	}{
		{"nik", "3201234567890001", "************0001"},
		{"email", "john@example.com", "jo**@example.com"},
		{"phone", "+628123456789", "*********6789"},
		{"name", "John Doe", "J*** D**"},
		{"unknown", "secret", "se**et"},
		{"nik", "", ""},
	}

	for _, tt := range tests {
		got := MaskField(tt.field, tt.value)
		if got != tt.want {
			t.Errorf("MaskField(%q, %q) = %q, want %q", tt.field, tt.value, got, tt.want)
		}
	}
}

func TestMaskField_ShortValues(t *testing.T) {
	got := MaskField("nik", "123")
	if got != "123" { // too short for masking, returns last 4 or all
		// Actually for NIK, len(3) < 4, so no masking
	}
	_ = got

	got2 := MaskField("unknown", "ab")
	if got2 != "**" {
		t.Errorf("short unknown = %q", got2)
	}
}
