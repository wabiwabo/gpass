package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestWrap_CapturesStack(t *testing.T) {
	cause := fmt.Errorf("connection refused")
	wrapped := Wrap(cause, Internal(CodeUpstreamError, "dukcapil unavailable"))

	if wrapped.Stack == nil {
		t.Fatal("expected stack trace to be captured")
	}
	if len(wrapped.Stack) == 0 {
		t.Fatal("expected non-empty stack trace")
	}

	// First frame should be in this test file.
	found := false
	for _, f := range wrapped.Stack {
		if strings.Contains(f.File, "stack_test.go") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected stack to contain stack_test.go, got:\n%s", wrapped.Stack)
	}
}

func TestWrap_ErrorMessage(t *testing.T) {
	cause := fmt.Errorf("timeout")
	wrapped := Wrap(cause, BadGateway(CodeUpstreamError, "AHU service error"))

	msg := wrapped.Error()
	if !strings.Contains(msg, "upstream_error") {
		t.Errorf("missing code in error: %s", msg)
	}
	if !strings.Contains(msg, "AHU service error") {
		t.Errorf("missing message in error: %s", msg)
	}
	if !strings.Contains(msg, "timeout") {
		t.Errorf("missing cause in error: %s", msg)
	}
}

func TestWrap_Unwrap(t *testing.T) {
	cause := fmt.Errorf("original")
	wrapped := Wrap(cause, Internal(CodeServiceUnavailable, "wrapped"))

	if !errors.Is(wrapped, cause) {
		t.Error("errors.Is should find the cause")
	}

	unwrapped := errors.Unwrap(wrapped)
	if unwrapped != cause {
		t.Error("Unwrap should return the cause")
	}
}

func TestWrap_NilCause(t *testing.T) {
	wrapped := &WrappedError{
		AppError: *Internal(CodeServiceUnavailable, "no cause"),
	}

	msg := wrapped.Error()
	if strings.Contains(msg, "<nil>") {
		t.Error("nil cause should not appear in error message")
	}
}

func TestWrapMsg(t *testing.T) {
	cause := fmt.Errorf("db error")
	wrapped := WrapMsg(cause, CodeServiceUnavailable, "database failed", http.StatusInternalServerError)

	if wrapped.HTTPStatus != 500 {
		t.Errorf("HTTPStatus: got %d, want 500", wrapped.HTTPStatus)
	}
	if wrapped.Code != CodeServiceUnavailable {
		t.Errorf("Code: got %q, want %q", wrapped.Code, CodeServiceUnavailable)
	}
	if wrapped.Cause != cause {
		t.Error("Cause should match")
	}
	if len(wrapped.Stack) == 0 {
		t.Error("should capture stack")
	}
}

func TestNewWithStack(t *testing.T) {
	err := NewWithStack(CodeInvalidRequest, "bad input", http.StatusBadRequest)

	if err.HTTPStatus != 400 {
		t.Errorf("HTTPStatus: got %d, want 400", err.HTTPStatus)
	}
	if err.Cause != nil {
		t.Error("Cause should be nil")
	}
	if len(err.Stack) == 0 {
		t.Error("should capture stack")
	}
}

func TestIsAppError_DirectAppError(t *testing.T) {
	err := BadRequest(CodeInvalidNIK, "invalid NIK format")
	appErr, ok := IsAppError(err)
	if !ok {
		t.Fatal("should find AppError")
	}
	if appErr.Code != CodeInvalidNIK {
		t.Errorf("code: got %q, want %q", appErr.Code, CodeInvalidNIK)
	}
}

func TestIsAppError_WrappedError(t *testing.T) {
	wrapped := Wrap(fmt.Errorf("cause"), NotFound(CodeResourceNotFound, "user not found"))
	appErr, ok := IsAppError(wrapped)
	if !ok {
		t.Fatal("should find AppError in wrapped")
	}
	if appErr.Code != CodeResourceNotFound {
		t.Errorf("code: got %q", appErr.Code)
	}
}

func TestIsAppError_NotAppError(t *testing.T) {
	_, ok := IsAppError(fmt.Errorf("plain error"))
	if ok {
		t.Error("plain error should not be AppError")
	}
}

func TestRootCause(t *testing.T) {
	root := fmt.Errorf("root cause")
	mid := fmt.Errorf("mid: %w", root)
	outer := Wrap(mid, Internal("code", "outer"))

	got := RootCause(outer)
	if got != root {
		t.Errorf("root cause: got %q, want %q", got, root)
	}
}

func TestRootCause_NilError(t *testing.T) {
	var err error
	got := RootCause(err)
	if got != nil {
		t.Error("nil error should return nil")
	}
}

func TestRootCause_SingleError(t *testing.T) {
	err := fmt.Errorf("single")
	got := RootCause(err)
	if got != err {
		t.Error("single error should return itself")
	}
}

func TestErrorChain(t *testing.T) {
	root := fmt.Errorf("root")
	mid := fmt.Errorf("mid: %w", root)
	outer := Wrap(mid, Internal("code", "outer"))

	chain := ErrorChain(outer)
	if len(chain) != 3 {
		t.Fatalf("chain length: got %d, want 3", len(chain))
	}
	if chain[0] != outer {
		t.Error("chain[0] should be outer")
	}
	if chain[2] != root {
		t.Error("chain[2] should be root")
	}
}

func TestErrorChain_NilError(t *testing.T) {
	chain := ErrorChain(nil)
	if len(chain) != 0 {
		t.Error("nil error should return empty chain")
	}
}

func TestWrappedError_MarshalJSON(t *testing.T) {
	wrapped := Wrap(
		fmt.Errorf("connection refused"),
		Internal(CodeUpstreamError, "dukcapil down"),
	)
	wrapped.Details = map[string]string{"service": "dukcapil"}

	data, err := json.Marshal(wrapped)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if parsed["error"] != CodeUpstreamError {
		t.Errorf("error code: got %v", parsed["error"])
	}
	if parsed["message"] != "dukcapil down" {
		t.Errorf("message: got %v", parsed["message"])
	}
	if parsed["cause"] != "connection refused" {
		t.Errorf("cause: got %v", parsed["cause"])
	}
	if parsed["stack"] == nil {
		t.Error("stack should be present")
	}
	if parsed["details"] == nil {
		t.Error("details should be present")
	}
}

func TestWrappedError_MarshalJSON_NoCause(t *testing.T) {
	err := NewWithStack(CodeInvalidRequest, "bad", 400)
	data, _ := json.Marshal(err)

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if _, ok := parsed["cause"]; ok {
		cause := parsed["cause"]
		if cause != nil && cause != "" {
			t.Error("cause should be omitted when nil")
		}
	}
}

func TestFrame_String(t *testing.T) {
	f := Frame{
		Function: "main.handler",
		File:     "/opt/gpass/main.go",
		Line:     42,
	}
	s := f.String()
	if !strings.Contains(s, "main.handler") {
		t.Errorf("missing function: %s", s)
	}
	if !strings.Contains(s, "/opt/gpass/main.go:42") {
		t.Errorf("missing file:line: %s", s)
	}
}

func TestStackTrace_String(t *testing.T) {
	st := StackTrace{
		{Function: "a", File: "a.go", Line: 1},
		{Function: "b", File: "b.go", Line: 2},
	}
	s := st.String()
	if !strings.Contains(s, "a.go:1") {
		t.Error("missing first frame")
	}
	if !strings.Contains(s, "b.go:2") {
		t.Error("missing second frame")
	}
}

func TestStackTrace_Empty(t *testing.T) {
	var st StackTrace
	if st.String() != "" {
		t.Error("empty stack should return empty string")
	}
}
