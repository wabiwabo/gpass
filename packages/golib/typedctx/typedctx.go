// Package typedctx provides type-safe context value storage using
// generics. Eliminates string key collisions and type assertion
// boilerplate when passing values through context.
package typedctx

import "context"

// Key is a type-safe context key for values of type T.
type Key[T any] struct {
	name string
}

// NewKey creates a new typed context key.
func NewKey[T any](name string) Key[T] {
	return Key[T]{name: name}
}

// Set stores a value in the context.
func (k Key[T]) Set(ctx context.Context, value T) context.Context {
	return context.WithValue(ctx, k, value)
}

// Get retrieves a value from the context.
// Returns the value and whether it was found.
func (k Key[T]) Get(ctx context.Context) (T, bool) {
	v, ok := ctx.Value(k).(T)
	return v, ok
}

// MustGet retrieves a value or panics if not found.
func (k Key[T]) MustGet(ctx context.Context) T {
	v, ok := k.Get(ctx)
	if !ok {
		panic("typedctx: key " + k.name + " not found in context")
	}
	return v
}

// GetOrDefault retrieves a value or returns the default.
func (k Key[T]) GetOrDefault(ctx context.Context, def T) T {
	v, ok := k.Get(ctx)
	if !ok {
		return def
	}
	return v
}

// Name returns the key's descriptive name.
func (k Key[T]) Name() string {
	return k.name
}

// Common pre-defined keys for enterprise patterns.
var (
	UserIDKey       = NewKey[string]("user_id")
	TenantIDKey     = NewKey[string]("tenant_id")
	RequestIDKey    = NewKey[string]("request_id")
	CorrelationKey  = NewKey[string]("correlation_id")
	TraceIDKey      = NewKey[string]("trace_id")
	SessionIDKey    = NewKey[string]("session_id")
	RolesKey        = NewKey[[]string]("roles")
	PermissionsKey  = NewKey[[]string]("permissions")
	LocaleKey       = NewKey[string]("locale")
)
