package cqrs

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// Command represents a write operation that modifies state.
type Command interface {
	CommandName() string
}

// Query represents a read operation that returns data.
type Query interface {
	QueryName() string
}

// CommandHandler processes a specific command type.
type CommandHandler[C Command] interface {
	Handle(ctx context.Context, cmd C) error
}

// QueryHandler processes a specific query type and returns a result.
type QueryHandler[Q Query, R any] interface {
	Handle(ctx context.Context, query Q) (R, error)
}

// Middleware wraps command or query processing.
type Middleware func(ctx context.Context, name string, next func(context.Context) error) error

// CommandBus dispatches commands to their registered handlers.
type CommandBus struct {
	mu          sync.RWMutex
	handlers    map[string]interface{}
	middlewares []Middleware
}

// NewCommandBus creates a new command bus.
func NewCommandBus() *CommandBus {
	return &CommandBus{
		handlers: make(map[string]interface{}),
	}
}

// Register registers a command handler by command name.
func (b *CommandBus) Register(name string, handler interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[name] = handler
}

// Use adds middleware to the command bus.
func (b *CommandBus) Use(mw Middleware) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.middlewares = append(b.middlewares, mw)
}

// Dispatch sends a command to its registered handler.
func (b *CommandBus) Dispatch(ctx context.Context, cmd Command) error {
	b.mu.RLock()
	handler, ok := b.handlers[cmd.CommandName()]
	middlewares := make([]Middleware, len(b.middlewares))
	copy(middlewares, b.middlewares)
	b.mu.RUnlock()

	if !ok {
		return fmt.Errorf("cqrs: no handler registered for command %q", cmd.CommandName())
	}

	execute := func(ctx context.Context) error {
		return callHandler(ctx, handler, cmd)
	}

	// Apply middlewares in reverse order (first registered runs first).
	for i := len(middlewares) - 1; i >= 0; i-- {
		mw := middlewares[i]
		next := execute
		name := cmd.CommandName()
		execute = func(ctx context.Context) error {
			return mw(ctx, name, next)
		}
	}

	return execute(ctx)
}

// QueryBus dispatches queries to their registered handlers.
type QueryBus struct {
	mu          sync.RWMutex
	handlers    map[string]interface{}
	middlewares []Middleware
}

// NewQueryBus creates a new query bus.
func NewQueryBus() *QueryBus {
	return &QueryBus{
		handlers: make(map[string]interface{}),
	}
}

// Register registers a query handler by query name.
func (b *QueryBus) Register(name string, handler interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[name] = handler
}

// Use adds middleware to the query bus.
func (b *QueryBus) Use(mw Middleware) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.middlewares = append(b.middlewares, mw)
}

// Dispatch sends a query to its registered handler and returns the result.
func (b *QueryBus) Dispatch(ctx context.Context, query Query) (interface{}, error) {
	b.mu.RLock()
	handler, ok := b.handlers[query.QueryName()]
	middlewares := make([]Middleware, len(b.middlewares))
	copy(middlewares, b.middlewares)
	b.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("cqrs: no handler registered for query %q", query.QueryName())
	}

	var result interface{}
	execute := func(ctx context.Context) error {
		var err error
		result, err = callQueryHandler(ctx, handler, query)
		return err
	}

	for i := len(middlewares) - 1; i >= 0; i-- {
		mw := middlewares[i]
		next := execute
		name := query.QueryName()
		execute = func(ctx context.Context) error {
			return mw(ctx, name, next)
		}
	}

	err := execute(ctx)
	return result, err
}

// callHandler uses reflection to call a typed handler.
func callHandler(ctx context.Context, handler interface{}, cmd Command) error {
	hv := reflect.ValueOf(handler)
	method := hv.MethodByName("Handle")
	if !method.IsValid() {
		return fmt.Errorf("cqrs: handler does not have Handle method")
	}

	results := method.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(cmd),
	})

	if len(results) == 0 {
		return nil
	}

	if results[len(results)-1].Interface() != nil {
		return results[len(results)-1].Interface().(error)
	}
	return nil
}

// callQueryHandler uses reflection to call a typed query handler.
func callQueryHandler(ctx context.Context, handler interface{}, query Query) (interface{}, error) {
	hv := reflect.ValueOf(handler)
	method := hv.MethodByName("Handle")
	if !method.IsValid() {
		return nil, fmt.Errorf("cqrs: handler does not have Handle method")
	}

	results := method.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(query),
	})

	if len(results) < 2 {
		return nil, fmt.Errorf("cqrs: query handler must return (result, error)")
	}

	var err error
	if results[1].Interface() != nil {
		err = results[1].Interface().(error)
	}

	return results[0].Interface(), err
}

// LoggingMiddleware logs command/query execution.
type LoggingMiddleware struct {
	Logger func(msg string, args ...interface{})
}

// Wrap returns a Middleware that logs command/query names and errors.
func (m *LoggingMiddleware) Wrap() Middleware {
	return func(ctx context.Context, name string, next func(context.Context) error) error {
		m.Logger("executing", "name", name)
		err := next(ctx)
		if err != nil {
			m.Logger("failed", "name", name, "error", err)
		}
		return err
	}
}
