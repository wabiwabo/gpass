package envx

import (
	"os"
	"testing"
	"time"
)

func TestString(t *testing.T) {
	os.Setenv("TEST_STR", "hello")
	defer os.Unsetenv("TEST_STR")

	if String("TEST_STR", "default") != "hello" {
		t.Error("should return env value")
	}
	if String("TEST_MISSING", "default") != "default" {
		t.Error("should return default")
	}
}

func TestRequired(t *testing.T) {
	os.Setenv("TEST_REQ", "value")
	defer os.Unsetenv("TEST_REQ")

	v, err := Required("TEST_REQ")
	if err != nil || v != "value" {
		t.Error("should return value")
	}

	_, err = Required("TEST_MISSING_REQ")
	if err == nil {
		t.Error("should error for missing")
	}
}

func TestInt(t *testing.T) {
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	if Int("TEST_INT", 0) != 42 {
		t.Error("should parse int")
	}
	if Int("TEST_MISSING_INT", 99) != 99 {
		t.Error("should return default")
	}

	os.Setenv("TEST_BAD_INT", "abc")
	defer os.Unsetenv("TEST_BAD_INT")
	if Int("TEST_BAD_INT", 99) != 99 {
		t.Error("invalid int should return default")
	}
}

func TestInt64(t *testing.T) {
	os.Setenv("TEST_I64", "9999999999")
	defer os.Unsetenv("TEST_I64")

	if Int64("TEST_I64", 0) != 9999999999 {
		t.Error("should parse int64")
	}
}

func TestBool(t *testing.T) {
	tests := []struct {
		val  string
		want bool
	}{
		{"true", true},
		{"TRUE", true},
		{"1", true},
		{"yes", true},
		{"on", true},
		{"false", false},
		{"0", false},
		{"no", false},
	}

	for _, tt := range tests {
		os.Setenv("TEST_BOOL", tt.val)
		if Bool("TEST_BOOL", false) != tt.want {
			t.Errorf("Bool(%q) = %v, want %v", tt.val, Bool("TEST_BOOL", false), tt.want)
		}
	}
	os.Unsetenv("TEST_BOOL")

	if Bool("TEST_MISSING_BOOL", true) != true {
		t.Error("missing should return default")
	}
}

func TestDuration(t *testing.T) {
	os.Setenv("TEST_DUR", "5s")
	defer os.Unsetenv("TEST_DUR")

	if Duration("TEST_DUR", 0) != 5*time.Second {
		t.Error("should parse duration")
	}
	if Duration("TEST_MISSING_DUR", 10*time.Second) != 10*time.Second {
		t.Error("should return default")
	}
}

func TestFloat64(t *testing.T) {
	os.Setenv("TEST_FLOAT", "3.14")
	defer os.Unsetenv("TEST_FLOAT")

	if Float64("TEST_FLOAT", 0) != 3.14 {
		t.Error("should parse float")
	}
}

func TestList(t *testing.T) {
	os.Setenv("TEST_LIST", "a,b,c")
	defer os.Unsetenv("TEST_LIST")

	list := List("TEST_LIST", ",")
	if len(list) != 3 || list[0] != "a" {
		t.Errorf("List = %v", list)
	}

	if List("TEST_MISSING_LIST", ",") != nil {
		t.Error("missing should return nil")
	}
}

func TestList_SpaceDelimited(t *testing.T) {
	os.Setenv("TEST_SLIST", "x | y | z")
	defer os.Unsetenv("TEST_SLIST")

	list := List("TEST_SLIST", "|")
	if len(list) != 3 {
		t.Errorf("List = %v", list)
	}
}

func TestOneOf(t *testing.T) {
	os.Setenv("TEST_ENV", "staging")
	defer os.Unsetenv("TEST_ENV")

	if OneOf("TEST_ENV", []string{"dev", "staging", "prod"}, "dev") != "staging" {
		t.Error("should return matching value")
	}
	if OneOf("TEST_ENV", []string{"dev", "prod"}, "dev") != "dev" {
		t.Error("invalid should return default")
	}
}

func TestMustAll(t *testing.T) {
	os.Setenv("TEST_A", "1")
	os.Setenv("TEST_B", "2")
	defer os.Unsetenv("TEST_A")
	defer os.Unsetenv("TEST_B")

	if err := MustAll("TEST_A", "TEST_B"); err != nil {
		t.Error("all set should pass")
	}

	if err := MustAll("TEST_A", "TEST_MISSING_C"); err == nil {
		t.Error("should error for missing")
	}
}
