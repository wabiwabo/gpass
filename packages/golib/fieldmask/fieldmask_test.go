package fieldmask

import (
	"testing"
)

func TestNew(t *testing.T) {
	fm := New("name", "email", "phone")
	if fm.Len() != 3 {
		t.Errorf("Len = %d, want 3", fm.Len())
	}
}

func TestNew_Deduplication(t *testing.T) {
	fm := New("name", "email", "name")
	if fm.Len() != 2 {
		t.Errorf("Len = %d, want 2 (deduped)", fm.Len())
	}
}

func TestNew_TrimsWhitespace(t *testing.T) {
	fm := New("  name  ", " email ", "")
	if fm.Len() != 2 {
		t.Errorf("Len = %d, want 2", fm.Len())
	}
	if !fm.Contains("name") {
		t.Error("should contain trimmed 'name'")
	}
	if !fm.Contains("email") {
		t.Error("should contain trimmed 'email'")
	}
}

func TestNew_Empty(t *testing.T) {
	fm := New()
	if !fm.IsEmpty() {
		t.Error("should be empty")
	}
	if fm.Len() != 0 {
		t.Errorf("Len = %d", fm.Len())
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
		paths []string
	}{
		{"simple", "name,email,phone", 3, []string{"email", "name", "phone"}},
		{"nested", "name,address.city,address.zip", 3, []string{"address.city", "address.zip", "name"}},
		{"spaces", " name , email , phone ", 3, []string{"email", "name", "phone"}},
		{"empty string", "", 0, nil},
		{"whitespace only", "   ", 0, nil},
		{"single field", "name", 1, []string{"name"}},
		{"trailing comma", "name,email,", 2, []string{"email", "name"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := Parse(tt.input)
			if fm.Len() != tt.want {
				t.Errorf("Len = %d, want %d", fm.Len(), tt.want)
			}
			paths := fm.Paths()
			if tt.paths == nil && paths != nil {
				t.Errorf("Paths = %v, want nil", paths)
			}
			for i, p := range paths {
				if i < len(tt.paths) && p != tt.paths[i] {
					t.Errorf("Paths[%d] = %q, want %q", i, p, tt.paths[i])
				}
			}
		})
	}
}

func TestContains(t *testing.T) {
	fm := New("name", "email", "address.city")

	if !fm.Contains("name") {
		t.Error("should contain 'name'")
	}
	if !fm.Contains("address.city") {
		t.Error("should contain 'address.city'")
	}
	if fm.Contains("phone") {
		t.Error("should not contain 'phone'")
	}
	if fm.Contains("address") {
		t.Error("should not contain 'address' (only 'address.city')")
	}
}

func TestContains_NilMask(t *testing.T) {
	var fm *FieldMask
	if fm.Contains("anything") {
		t.Error("nil mask should not contain anything")
	}
}

func TestContainsPrefix(t *testing.T) {
	fm := New("name", "address.city", "address.zip", "metadata.key1")

	tests := []struct {
		prefix string
		want   bool
	}{
		{"address", true},
		{"metadata", true},
		{"name", true},
		{"phone", false},
		{"addr", false},
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			if got := fm.ContainsPrefix(tt.prefix); got != tt.want {
				t.Errorf("ContainsPrefix(%q) = %v, want %v", tt.prefix, got, tt.want)
			}
		})
	}
}

func TestContainsPrefix_NilMask(t *testing.T) {
	var fm *FieldMask
	if fm.ContainsPrefix("anything") {
		t.Error("nil mask should not contain prefix")
	}
}

func TestPaths_Sorted(t *testing.T) {
	fm := New("z_field", "a_field", "m_field")
	paths := fm.Paths()

	if len(paths) != 3 {
		t.Fatalf("len = %d", len(paths))
	}
	if paths[0] != "a_field" || paths[1] != "m_field" || paths[2] != "z_field" {
		t.Errorf("Paths not sorted: %v", paths)
	}
}

func TestPaths_NilMask(t *testing.T) {
	var fm *FieldMask
	if fm.Paths() != nil {
		t.Error("nil mask paths should be nil")
	}
}

func TestIsEmpty(t *testing.T) {
	if !New().IsEmpty() {
		t.Error("empty mask should be empty")
	}
	if New("a").IsEmpty() {
		t.Error("non-empty mask should not be empty")
	}

	var fm *FieldMask
	if !fm.IsEmpty() {
		t.Error("nil mask should be empty")
	}
}

func TestLen_NilMask(t *testing.T) {
	var fm *FieldMask
	if fm.Len() != 0 {
		t.Errorf("nil mask Len = %d", fm.Len())
	}
}

func TestAdd(t *testing.T) {
	fm := New("a")
	fm.Add("b")
	fm.Add("  c  ")
	fm.Add("") // ignored

	if fm.Len() != 3 {
		t.Errorf("Len = %d, want 3", fm.Len())
	}
	if !fm.Contains("c") {
		t.Error("should contain trimmed 'c'")
	}
}

func TestRemove(t *testing.T) {
	fm := New("a", "b", "c")
	fm.Remove("b")

	if fm.Len() != 2 {
		t.Errorf("Len = %d, want 2", fm.Len())
	}
	if fm.Contains("b") {
		t.Error("should not contain 'b' after removal")
	}
}

func TestRemove_NonExistent(t *testing.T) {
	fm := New("a")
	fm.Remove("z") // should not panic
	if fm.Len() != 1 {
		t.Errorf("Len = %d", fm.Len())
	}
}

func TestUnion(t *testing.T) {
	a := New("x", "y")
	b := New("y", "z")
	u := a.Union(b)

	if u.Len() != 3 {
		t.Errorf("Union Len = %d, want 3", u.Len())
	}
	for _, p := range []string{"x", "y", "z"} {
		if !u.Contains(p) {
			t.Errorf("Union should contain %q", p)
		}
	}

	// Original masks unchanged
	if a.Len() != 2 {
		t.Error("original 'a' modified")
	}
}

func TestUnion_NilOther(t *testing.T) {
	a := New("x", "y")
	u := a.Union(nil)
	if u.Len() != 2 {
		t.Errorf("Union with nil Len = %d", u.Len())
	}
}

func TestIntersect(t *testing.T) {
	a := New("x", "y", "z")
	b := New("y", "z", "w")
	i := a.Intersect(b)

	if i.Len() != 2 {
		t.Errorf("Intersect Len = %d, want 2", i.Len())
	}
	if !i.Contains("y") || !i.Contains("z") {
		t.Errorf("Intersect should contain y and z, got %v", i.Paths())
	}
}

func TestIntersect_NilOther(t *testing.T) {
	a := New("x")
	i := a.Intersect(nil)
	if i.Len() != 0 {
		t.Errorf("Intersect with nil Len = %d", i.Len())
	}
}

func TestIntersect_NilSelf(t *testing.T) {
	var a *FieldMask
	b := New("x")
	i := a.Intersect(b)
	if i.Len() != 0 {
		t.Errorf("nil intersect Len = %d", i.Len())
	}
}

func TestString(t *testing.T) {
	fm := New("b", "a", "c")
	s := fm.String()
	if s != "a,b,c" {
		t.Errorf("String() = %q, want 'a,b,c'", s)
	}
}

func TestString_Empty(t *testing.T) {
	fm := New()
	if fm.String() != "" {
		t.Errorf("String() = %q, want empty", fm.String())
	}
}

func TestValidate_AllValid(t *testing.T) {
	fm := New("name", "email")
	allowed := []string{"name", "email", "phone"}
	if invalid := Validate(fm, allowed); invalid != "" {
		t.Errorf("Validate returned %q", invalid)
	}
}

func TestValidate_InvalidField(t *testing.T) {
	fm := New("name", "ssn", "email")
	allowed := []string{"name", "email", "phone"}
	if invalid := Validate(fm, allowed); invalid != "ssn" {
		t.Errorf("Validate = %q, want ssn", invalid)
	}
}

func TestValidate_NilMask(t *testing.T) {
	if invalid := Validate(nil, []string{"a"}); invalid != "" {
		t.Errorf("Validate nil = %q", invalid)
	}
}

func TestValidate_EmptyMask(t *testing.T) {
	if invalid := Validate(New(), []string{"a"}); invalid != "" {
		t.Errorf("Validate empty = %q", invalid)
	}
}

func TestFilter(t *testing.T) {
	data := map[string]interface{}{
		"name":  "John",
		"email": "john@example.com",
		"phone": "+628123456789",
		"ssn":   "1234567890",
	}

	fm := New("name", "email")
	filtered := Filter(data, fm)

	if len(filtered) != 2 {
		t.Errorf("Filter len = %d, want 2", len(filtered))
	}
	if filtered["name"] != "John" {
		t.Errorf("name = %v", filtered["name"])
	}
	if filtered["email"] != "john@example.com" {
		t.Errorf("email = %v", filtered["email"])
	}
	if _, ok := filtered["ssn"]; ok {
		t.Error("ssn should be filtered out")
	}
}

func TestFilter_NilMask(t *testing.T) {
	data := map[string]interface{}{"a": 1, "b": 2}
	filtered := Filter(data, nil)
	if len(filtered) != 2 {
		t.Error("nil mask should return all data")
	}
}

func TestFilter_EmptyMask(t *testing.T) {
	data := map[string]interface{}{"a": 1, "b": 2}
	filtered := Filter(data, New())
	if len(filtered) != 2 {
		t.Error("empty mask should return all data")
	}
}

func TestRoundTrip_ParseString(t *testing.T) {
	original := "address.city,email,name"
	fm := Parse(original)
	if fm.String() != original {
		t.Errorf("roundtrip: %q != %q", fm.String(), original)
	}
}
