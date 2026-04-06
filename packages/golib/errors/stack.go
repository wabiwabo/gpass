package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// Frame represents a single stack frame.
type Frame struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// String returns a human-readable representation of the frame.
func (f Frame) String() string {
	return fmt.Sprintf("%s\n\t%s:%d", f.Function, f.File, f.Line)
}

// StackTrace is a slice of captured stack frames.
type StackTrace []Frame

// String returns a formatted stack trace.
func (st StackTrace) String() string {
	if len(st) == 0 {
		return ""
	}
	var b strings.Builder
	for i, f := range st {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(f.String())
	}
	return b.String()
}

// captureStack captures the current call stack, skipping skip frames.
func captureStack(skip int) StackTrace {
	const maxDepth = 32
	pcs := make([]uintptr, maxDepth)
	n := runtime.Callers(skip+2, pcs) // +2 for Callers and captureStack itself
	if n == 0 {
		return nil
	}

	frames := runtime.CallersFrames(pcs[:n])
	stack := make(StackTrace, 0, n)

	for {
		frame, more := frames.Next()
		// Skip runtime internals.
		if strings.HasPrefix(frame.Function, "runtime.") {
			if !more {
				break
			}
			continue
		}
		stack = append(stack, Frame{
			Function: frame.Function,
			File:     frame.File,
			Line:     frame.Line,
		})
		if !more {
			break
		}
	}
	return stack
}

// WrappedError extends AppError with stack traces and error chaining.
type WrappedError struct {
	AppError
	Cause error      `json:"-"`
	Stack StackTrace `json:"stack,omitempty"`
}

// Error returns the full error message including the cause.
func (e *WrappedError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return e.AppError.Error()
}

// Unwrap returns the underlying cause for errors.Is/As chain walking.
func (e *WrappedError) Unwrap() error {
	return e.Cause
}

// MarshalJSON provides structured serialization including stack trace.
func (e *WrappedError) MarshalJSON() ([]byte, error) {
	type alias struct {
		Code    string     `json:"error"`
		Message string     `json:"message"`
		Details any        `json:"details,omitempty"`
		Cause   string     `json:"cause,omitempty"`
		Stack   StackTrace `json:"stack,omitempty"`
	}

	a := alias{
		Code:    e.Code,
		Message: e.Message,
		Details: e.Details,
		Stack:   e.Stack,
	}
	if e.Cause != nil {
		a.Cause = e.Cause.Error()
	}
	return json.Marshal(a)
}

// Wrap wraps a cause error with an AppError, capturing the stack trace.
func Wrap(cause error, appErr *AppError) *WrappedError {
	return &WrappedError{
		AppError: *appErr,
		Cause:    cause,
		Stack:    captureStack(1),
	}
}

// WrapMsg wraps a cause error with a code, message, and HTTP status.
func WrapMsg(cause error, code, message string, httpStatus int) *WrappedError {
	return &WrappedError{
		AppError: AppError{
			Code:       code,
			Message:    message,
			HTTPStatus: httpStatus,
		},
		Cause: cause,
		Stack: captureStack(1),
	}
}

// NewWithStack creates a new AppError with a captured stack trace.
func NewWithStack(code, message string, httpStatus int) *WrappedError {
	return &WrappedError{
		AppError: AppError{
			Code:       code,
			Message:    message,
			HTTPStatus: httpStatus,
		},
		Stack: captureStack(1),
	}
}

// IsAppError checks if err is or wraps an AppError and returns it.
func IsAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	var wrapped *WrappedError
	if errors.As(err, &wrapped) {
		return &wrapped.AppError, true
	}
	return nil, false
}

// RootCause walks the error chain and returns the deepest cause.
func RootCause(err error) error {
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err
		}
		err = unwrapped
	}
}

// ErrorChain returns all errors in the chain from outermost to innermost.
func ErrorChain(err error) []error {
	var chain []error
	for err != nil {
		chain = append(chain, err)
		err = errors.Unwrap(err)
	}
	return chain
}
