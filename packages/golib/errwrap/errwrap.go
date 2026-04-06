// Package errwrap provides error wrapping utilities.
// Adds context to errors while preserving the error chain
// for type assertion with errors.Is and errors.As.
package errwrap

import (
	"errors"
	"fmt"
)

// Wrap adds context to an error without losing the original.
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// Wrapf adds formatted context to an error.
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(format+": %w", append(args, err)...)
}

// New creates a new error with the given message.
func New(msg string) error {
	return errors.New(msg)
}

// Newf creates a new formatted error.
func Newf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

// Is reports whether any error in err's chain matches target.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's chain matching target.
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// Join combines multiple errors into one.
func Join(errs ...error) error {
	return errors.Join(errs...)
}

// Unwrap returns the underlying error.
func Unwrap(err error) error {
	return errors.Unwrap(err)
}

// IgnoreNil filters nil errors from a slice.
func IgnoreNil(errs []error) []error {
	result := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			result = append(result, err)
		}
	}
	return result
}

// First returns the first non-nil error.
func First(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
