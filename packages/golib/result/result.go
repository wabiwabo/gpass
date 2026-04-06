// Package result provides a generic Result type for representing
// success/failure outcomes without panicking. Inspired by Rust's
// Result<T, E> type, adapted for Go idioms.
package result

// Result represents either a success value or an error.
type Result[T any] struct {
	value T
	err   error
	ok    bool
}

// Ok creates a successful result.
func Ok[T any](value T) Result[T] {
	return Result[T]{value: value, ok: true}
}

// Err creates a failed result.
func Err[T any](err error) Result[T] {
	return Result[T]{err: err}
}

// IsOk returns true if the result is successful.
func (r Result[T]) IsOk() bool {
	return r.ok
}

// IsErr returns true if the result is an error.
func (r Result[T]) IsErr() bool {
	return !r.ok
}

// Value returns the success value. Panics if result is an error.
func (r Result[T]) Value() T {
	if !r.ok {
		panic("result: called Value() on error result")
	}
	return r.value
}

// Error returns the error. Returns nil if result is ok.
func (r Result[T]) Error() error {
	return r.err
}

// ValueOr returns the value if ok, otherwise returns the default.
func (r Result[T]) ValueOr(def T) T {
	if r.ok {
		return r.value
	}
	return def
}

// Unwrap returns (value, error) for traditional Go error handling.
func (r Result[T]) Unwrap() (T, error) {
	return r.value, r.err
}

// Map transforms the success value.
func Map[T, U any](r Result[T], fn func(T) U) Result[U] {
	if r.ok {
		return Ok(fn(r.value))
	}
	return Err[U](r.err)
}

// FlatMap transforms with a function that may fail.
func FlatMap[T, U any](r Result[T], fn func(T) Result[U]) Result[U] {
	if r.ok {
		return fn(r.value)
	}
	return Err[U](r.err)
}

// From converts a Go (value, error) pair to a Result.
func From[T any](value T, err error) Result[T] {
	if err != nil {
		return Err[T](err)
	}
	return Ok(value)
}

// Must extracts the value, panicking on error.
func Must[T any](r Result[T]) T {
	if !r.ok {
		panic(r.err)
	}
	return r.value
}

// Collect converts a slice of Results to a Result of a slice.
// Returns the first error encountered.
func Collect[T any](results []Result[T]) Result[[]T] {
	values := make([]T, 0, len(results))
	for _, r := range results {
		if r.IsErr() {
			return Err[[]T](r.err)
		}
		values = append(values, r.value)
	}
	return Ok(values)
}
