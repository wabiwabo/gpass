package configloader

import (
	"strings"
	"testing"
)

// EmbBase exists at package scope so it can be embedded anonymously
// (anonymous embeds of locally-defined types work, but a top-level type
// makes the test more readable).
type EmbBase struct {
	Common string `env:"COMMON" default:"base"`
}

// TestLoad_AnonymousEmbeddedStruct pins the field.Anonymous && Struct
// branch in loadStruct — embedded fields must be loaded into the parent
// without a prefix.
func TestLoad_AnonymousEmbeddedStruct(t *testing.T) {
	type Cfg struct {
		EmbBase
		Name string `env:"NAME"`
	}
	t.Setenv("COMMON", "shared")
	t.Setenv("NAME", "x")
	var c Cfg
	if err := Load(&c); err != nil {
		t.Fatal(err)
	}
	if c.Common != "shared" {
		t.Errorf("anonymous embed not loaded: %q", c.Common)
	}
	if c.Name != "x" {
		t.Errorf("Name = %q", c.Name)
	}
}

// TestValidate_RecursesIntoNestedStructs pins the validateStruct
// recursion branch — a violation in a nested struct must surface.
func TestValidate_RecursesIntoNestedStructs(t *testing.T) {
	type Inner struct {
		Workers int `validate:"min=1"`
	}
	type Outer struct {
		Inner Inner
		Name  string `validate:"nonempty"`
	}

	// Nested violation: Inner.Workers < 1.
	if err := Validate(&Outer{Inner: Inner{Workers: 0}, Name: "ok"}); err == nil ||
		!strings.Contains(err.Error(), "Workers") {
		t.Errorf("nested min: %v", err)
	}
	// Top-level violation: Name empty.
	if err := Validate(&Outer{Inner: Inner{Workers: 5}, Name: ""}); err == nil ||
		!strings.Contains(err.Error(), "Name") {
		t.Errorf("nonempty: %v", err)
	}
	// All valid.
	if err := Validate(&Outer{Inner: Inner{Workers: 5}, Name: "ok"}); err != nil {
		t.Errorf("valid: %v", err)
	}
}

// TestValidate_NonemptyOnNonStringIsNoOp pins the rule==nonempty branch
// when the field is not a string — must NOT error (rule is silently
// inapplicable).
func TestValidate_NonemptyOnNonStringIsNoOp(t *testing.T) {
	type C struct {
		N int `validate:"nonempty"`
	}
	if err := Validate(&C{N: 0}); err != nil {
		t.Errorf("nonempty on int should no-op, got %v", err)
	}
}
