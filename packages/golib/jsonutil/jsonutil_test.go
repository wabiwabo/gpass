package jsonutil

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestMustMarshal(t *testing.T) {
	tests := []struct {
		name string
		v    interface{}
		want string
	}{
		{"string", "hello", `"hello"`},
		{"number", 42, "42"},
		{"bool", true, "true"},
		{"null", nil, "null"},
		{"map", map[string]int{"a": 1}, `{"a":1}`},
		{"slice", []int{1, 2, 3}, "[1,2,3]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(MustMarshal(tt.v))
			if got != tt.want {
				t.Errorf("MustMarshal() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestMustMarshalPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for unmarshalable value")
		}
		msg, ok := r.(string)
		if !ok || !strings.HasPrefix(msg, "jsonutil:") {
			t.Errorf("panic message should start with 'jsonutil:': %v", r)
		}
	}()
	MustMarshal(make(chan int))
}

func TestPretty(t *testing.T) {
	v := map[string]int{"a": 1}
	data, err := Pretty(v)
	if err != nil {
		t.Fatalf("Pretty error: %v", err)
	}
	if !strings.Contains(string(data), "\n") {
		t.Error("Pretty output should contain newlines")
	}
	if !strings.Contains(string(data), "  ") {
		t.Error("Pretty output should contain indentation")
	}
}

func TestPrettyString(t *testing.T) {
	v := map[string]string{"key": "value"}
	s := PrettyString(v)
	if !strings.Contains(s, "key") {
		t.Error("PrettyString should contain key")
	}
	if !strings.Contains(s, "value") {
		t.Error("PrettyString should contain value")
	}
}

func TestPrettyStringInvalid(t *testing.T) {
	got := PrettyString(make(chan int))
	if got != "{}" {
		t.Errorf("PrettyString of invalid = %q, want %q", got, "{}")
	}
}

func TestCompact(t *testing.T) {
	input := []byte(`{
		"name": "test",
		"value": 123
	}`)
	got, err := Compact(input)
	if err != nil {
		t.Fatalf("Compact error: %v", err)
	}
	if bytes.Contains(got, []byte("\n")) || bytes.Contains(got, []byte("\t")) {
		t.Error("Compact should remove whitespace")
	}
	if !json.Valid(got) {
		t.Error("Compact output should be valid JSON")
	}
}

func TestCompactInvalid(t *testing.T) {
	_, err := Compact([]byte("{invalid"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValid(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"valid_object", []byte(`{"a":1}`), true},
		{"valid_array", []byte(`[1,2,3]`), true},
		{"valid_string", []byte(`"hello"`), true},
		{"valid_number", []byte(`42`), true},
		{"valid_null", []byte(`null`), true},
		{"invalid", []byte(`{bad`), false},
		{"empty", []byte(``), false},
		{"trailing_comma", []byte(`{"a":1,}`), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Valid(tt.data); got != tt.want {
				t.Errorf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecodeReader(t *testing.T) {
	r := strings.NewReader(`{"name":"test","age":30}`)
	var result struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	if err := DecodeReader(r, &result); err != nil {
		t.Fatalf("DecodeReader error: %v", err)
	}
	if result.Name != "test" {
		t.Errorf("Name: got %q, want %q", result.Name, "test")
	}
	if result.Age != 30 {
		t.Errorf("Age: got %d, want 30", result.Age)
	}
}

func TestDecodeReaderInvalid(t *testing.T) {
	r := strings.NewReader("{invalid")
	var result map[string]interface{}
	if err := DecodeReader(r, &result); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestClone(t *testing.T) {
	type inner struct {
		Value int `json:"value"`
	}
	type outer struct {
		Name  string `json:"name"`
		Inner inner  `json:"inner"`
	}

	original := outer{Name: "test", Inner: inner{Value: 42}}
	cloned, err := Clone(original)
	if err != nil {
		t.Fatalf("Clone error: %v", err)
	}
	if cloned.Name != original.Name {
		t.Errorf("Name: got %q, want %q", cloned.Name, original.Name)
	}
	if cloned.Inner.Value != original.Inner.Value {
		t.Errorf("Inner.Value: got %d, want %d", cloned.Inner.Value, original.Inner.Value)
	}
	// Verify independence
	cloned.Name = "modified"
	if original.Name == "modified" {
		t.Error("Clone should produce independent copy")
	}
}

func TestCloneSlice(t *testing.T) {
	original := []int{1, 2, 3}
	cloned, err := Clone(original)
	if err != nil {
		t.Fatalf("Clone error: %v", err)
	}
	if len(cloned) != len(original) {
		t.Fatalf("length: got %d, want %d", len(cloned), len(original))
	}
	cloned[0] = 99
	if original[0] == 99 {
		t.Error("Clone should produce independent copy")
	}
}

func TestCloneMap(t *testing.T) {
	original := map[string][]int{"a": {1, 2}, "b": {3, 4}}
	cloned, err := Clone(original)
	if err != nil {
		t.Fatalf("Clone error: %v", err)
	}
	cloned["a"][0] = 99
	if original["a"][0] == 99 {
		t.Error("Clone should deep-copy nested slices")
	}
}

func TestMerge(t *testing.T) {
	a := []byte(`{"name":"alice","age":30}`)
	b := []byte(`{"age":31,"role":"admin"}`)
	result, err := Merge(a, b)
	if err != nil {
		t.Fatalf("Merge error: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}
	if string(m["name"]) != `"alice"` {
		t.Errorf("name: got %s, want %q", m["name"], "alice")
	}
	if string(m["age"]) != "31" {
		t.Errorf("age: got %s, want 31", m["age"])
	}
	if string(m["role"]) != `"admin"` {
		t.Errorf("role: got %s, want %q", m["role"], "admin")
	}
}

func TestMergeThree(t *testing.T) {
	a := []byte(`{"x":1}`)
	b := []byte(`{"y":2}`)
	c := []byte(`{"x":3,"z":4}`)
	result, err := Merge(a, b, c)
	if err != nil {
		t.Fatalf("Merge error: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if string(m["x"]) != "3" {
		t.Errorf("x should be overridden to 3, got %s", m["x"])
	}
	if string(m["y"]) != "2" {
		t.Errorf("y: got %s, want 2", m["y"])
	}
	if string(m["z"]) != "4" {
		t.Errorf("z: got %s, want 4", m["z"])
	}
}

func TestMergeInvalid(t *testing.T) {
	_, err := Merge([]byte(`{"a":1}`), []byte("{invalid"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMergeEmpty(t *testing.T) {
	result, err := Merge()
	if err != nil {
		t.Fatalf("Merge empty error: %v", err)
	}
	if !json.Valid(result) {
		t.Error("Merge empty should return valid JSON")
	}
}

func TestExtractField(t *testing.T) {
	data := []byte(`{"name":"test","value":42,"nested":{"a":1}}`)

	tests := []struct {
		name  string
		field string
		want  string
		ok    bool
	}{
		{"string_field", "name", `"test"`, true},
		{"number_field", "value", "42", true},
		{"nested_field", "nested", `{"a":1}`, true},
		{"missing_field", "nonexistent", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ExtractField(data, tt.field)
			if ok != tt.ok {
				t.Errorf("ok: got %v, want %v", ok, tt.ok)
			}
			if ok && string(got) != tt.want {
				t.Errorf("value: got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestExtractFieldInvalidJSON(t *testing.T) {
	_, ok := ExtractField([]byte("{bad"), "field")
	if ok {
		t.Error("expected ok=false for invalid JSON")
	}
}

func TestExtractFieldArray(t *testing.T) {
	data := []byte(`{"items":[1,2,3]}`)
	got, ok := ExtractField(data, "items")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if string(got) != "[1,2,3]" {
		t.Errorf("got %s, want [1,2,3]", got)
	}
}
