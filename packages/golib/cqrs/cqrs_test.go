package cqrs

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

// Test command
type CreateUserCmd struct {
	Name  string
	Email string
}

func (c CreateUserCmd) CommandName() string { return "create_user" }

type createUserHandler struct {
	called atomic.Int32
}

func (h *createUserHandler) Handle(_ context.Context, cmd CreateUserCmd) error {
	h.called.Add(1)
	if cmd.Name == "" {
		return errors.New("name required")
	}
	return nil
}

// Test query
type GetUserQuery struct {
	ID string
}

func (q GetUserQuery) QueryName() string { return "get_user" }

type getUserHandler struct{}

func (h *getUserHandler) Handle(_ context.Context, query GetUserQuery) (map[string]string, error) {
	if query.ID == "" {
		return nil, errors.New("id required")
	}
	return map[string]string{"id": query.ID, "name": "John"}, nil
}

func TestCommandBus_Dispatch(t *testing.T) {
	bus := NewCommandBus()
	h := &createUserHandler{}
	bus.Register("create_user", h)

	err := bus.Dispatch(context.Background(), CreateUserCmd{Name: "Alice", Email: "alice@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if h.called.Load() != 1 {
		t.Errorf("handler called: got %d, want 1", h.called.Load())
	}
}

func TestCommandBus_Dispatch_Error(t *testing.T) {
	bus := NewCommandBus()
	bus.Register("create_user", &createUserHandler{})

	err := bus.Dispatch(context.Background(), CreateUserCmd{Name: ""})
	if err == nil {
		t.Error("should return error")
	}
}

func TestCommandBus_Dispatch_NoHandler(t *testing.T) {
	bus := NewCommandBus()
	err := bus.Dispatch(context.Background(), CreateUserCmd{Name: "Bob"})
	if err == nil {
		t.Error("should fail with no handler")
	}
}

func TestCommandBus_Middleware(t *testing.T) {
	bus := NewCommandBus()
	bus.Register("create_user", &createUserHandler{})

	var middlewareCalled atomic.Int32
	bus.Use(func(ctx context.Context, name string, next func(context.Context) error) error {
		middlewareCalled.Add(1)
		return next(ctx)
	})

	bus.Dispatch(context.Background(), CreateUserCmd{Name: "Test"})

	if middlewareCalled.Load() != 1 {
		t.Errorf("middleware called: got %d", middlewareCalled.Load())
	}
}

func TestCommandBus_MiddlewareOrder(t *testing.T) {
	bus := NewCommandBus()
	bus.Register("create_user", &createUserHandler{})

	var order []string
	bus.Use(func(ctx context.Context, name string, next func(context.Context) error) error {
		order = append(order, "first")
		return next(ctx)
	})
	bus.Use(func(ctx context.Context, name string, next func(context.Context) error) error {
		order = append(order, "second")
		return next(ctx)
	})

	bus.Dispatch(context.Background(), CreateUserCmd{Name: "Test"})

	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("middleware order: got %v, want [first second]", order)
	}
}

func TestQueryBus_Dispatch(t *testing.T) {
	bus := NewQueryBus()
	bus.Register("get_user", &getUserHandler{})

	result, err := bus.Dispatch(context.Background(), GetUserQuery{ID: "123"})
	if err != nil {
		t.Fatal(err)
	}

	user, ok := result.(map[string]string)
	if !ok {
		t.Fatal("expected map result")
	}
	if user["id"] != "123" {
		t.Errorf("user id: got %q", user["id"])
	}
}

func TestQueryBus_Dispatch_Error(t *testing.T) {
	bus := NewQueryBus()
	bus.Register("get_user", &getUserHandler{})

	_, err := bus.Dispatch(context.Background(), GetUserQuery{ID: ""})
	if err == nil {
		t.Error("should return error")
	}
}

func TestQueryBus_Dispatch_NoHandler(t *testing.T) {
	bus := NewQueryBus()
	_, err := bus.Dispatch(context.Background(), GetUserQuery{ID: "123"})
	if err == nil {
		t.Error("should fail with no handler")
	}
}

func TestQueryBus_Middleware(t *testing.T) {
	bus := NewQueryBus()
	bus.Register("get_user", &getUserHandler{})

	var mwCalled atomic.Int32
	bus.Use(func(ctx context.Context, name string, next func(context.Context) error) error {
		mwCalled.Add(1)
		return next(ctx)
	})

	bus.Dispatch(context.Background(), GetUserQuery{ID: "1"})
	if mwCalled.Load() != 1 {
		t.Errorf("middleware: got %d", mwCalled.Load())
	}
}

func TestLoggingMiddleware(t *testing.T) {
	var logs []string
	lm := &LoggingMiddleware{
		Logger: func(msg string, args ...interface{}) {
			logs = append(logs, msg)
		},
	}

	bus := NewCommandBus()
	bus.Register("create_user", &createUserHandler{})
	bus.Use(lm.Wrap())

	bus.Dispatch(context.Background(), CreateUserCmd{Name: "Test"})
	if len(logs) != 1 || logs[0] != "executing" {
		t.Errorf("logs: got %v", logs)
	}
}

func TestLoggingMiddleware_LogsError(t *testing.T) {
	var logs []string
	lm := &LoggingMiddleware{
		Logger: func(msg string, args ...interface{}) {
			logs = append(logs, msg)
		},
	}

	bus := NewCommandBus()
	bus.Register("create_user", &createUserHandler{})
	bus.Use(lm.Wrap())

	bus.Dispatch(context.Background(), CreateUserCmd{Name: ""})
	if len(logs) != 2 {
		t.Errorf("expected 2 log entries (executing + failed), got %d", len(logs))
	}
}
