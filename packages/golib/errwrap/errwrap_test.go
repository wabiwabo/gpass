package errwrap

import (
	"errors"
	"strings"
	"testing"
)

var errBase = New("base error")

func TestWrap(t *testing.T) {
	err := Wrap(errBase, "context")
	if err == nil {
		t.Fatal("should not be nil")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Error("should contain context")
	}
	if !strings.Contains(err.Error(), "base error") {
		t.Error("should contain base error")
	}
	if !Is(err, errBase) {
		t.Error("should match base error with Is")
	}
}

func TestWrapNil(t *testing.T) {
	if Wrap(nil, "context") != nil {
		t.Error("wrapping nil should return nil")
	}
}

func TestWrapf(t *testing.T) {
	err := Wrapf(errBase, "operation %s failed", "create")
	if !strings.Contains(err.Error(), "operation create failed") {
		t.Errorf("got %q", err.Error())
	}
	if !Is(err, errBase) {
		t.Error("should match base error")
	}
}

func TestWrapfNil(t *testing.T) {
	if Wrapf(nil, "context %d", 1) != nil {
		t.Error("wrapping nil should return nil")
	}
}

func TestNew(t *testing.T) {
	err := New("test error")
	if err == nil {
		t.Fatal("should not be nil")
	}
	if err.Error() != "test error" {
		t.Errorf("got %q", err.Error())
	}
}

func TestNewf(t *testing.T) {
	err := Newf("error %d: %s", 42, "details")
	if !strings.Contains(err.Error(), "42") {
		t.Errorf("got %q", err.Error())
	}
}

func TestIs(t *testing.T) {
	wrapped := Wrap(errBase, "layer1")
	wrapped2 := Wrap(wrapped, "layer2")
	if !Is(wrapped2, errBase) {
		t.Error("Is should traverse chain")
	}
}

type customErr struct {
	Code int
}

func (e *customErr) Error() string { return "custom" }

func TestAs(t *testing.T) {
	ce := &customErr{Code: 404}

	err := Wrapf(ce, "not found")

	var target *customErr
	if !As(err, &target) {
		t.Fatal("As should find custom error")
	}
	if target.Code != 404 {
		t.Errorf("Code: got %d", target.Code)
	}
}

func TestJoin(t *testing.T) {
	e1 := New("error 1")
	e2 := New("error 2")
	joined := Join(e1, e2)
	if joined == nil {
		t.Fatal("should not be nil")
	}
	if !Is(joined, e1) {
		t.Error("should contain error 1")
	}
	if !Is(joined, e2) {
		t.Error("should contain error 2")
	}
}

func TestJoinNils(t *testing.T) {
	joined := Join(nil, nil)
	if joined != nil {
		t.Error("joining nils should return nil")
	}
}

func TestUnwrap(t *testing.T) {
	wrapped := Wrap(errBase, "context")
	inner := Unwrap(wrapped)
	if !errors.Is(inner, errBase) {
		t.Error("unwrapped should be base error")
	}
}

func TestIgnoreNil(t *testing.T) {
	errs := []error{New("a"), nil, New("b"), nil, New("c")}
	got := IgnoreNil(errs)
	if len(got) != 3 {
		t.Errorf("length: got %d, want 3", len(got))
	}
}

func TestIgnoreNilAllNil(t *testing.T) {
	got := IgnoreNil([]error{nil, nil})
	if len(got) != 0 {
		t.Errorf("length: got %d, want 0", len(got))
	}
}

func TestFirst(t *testing.T) {
	e := New("first")
	got := First(nil, nil, e, New("second"))
	if got != e {
		t.Error("should return first non-nil error")
	}
}

func TestFirstAllNil(t *testing.T) {
	got := First(nil, nil, nil)
	if got != nil {
		t.Error("should return nil when all nil")
	}
}
