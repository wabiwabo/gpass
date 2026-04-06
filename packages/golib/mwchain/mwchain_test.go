package mwchain

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func headerMiddleware(name, value string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(name, value)
			next.ServeHTTP(w, r)
		})
	}
}

func TestChain_Use(t *testing.T) {
	c := New().
		Use("header-a", headerMiddleware("X-A", "1")).
		Use("header-b", headerMiddleware("X-B", "2"))

	handler := c.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-A") != "1" {
		t.Error("X-A should be set")
	}
	if w.Header().Get("X-B") != "2" {
		t.Error("X-B should be set")
	}
}

func TestChain_UseIf_Enabled(t *testing.T) {
	c := New().
		UseIf("conditional", true, headerMiddleware("X-Cond", "yes"))

	handler := c.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Cond") != "yes" {
		t.Error("enabled middleware should run")
	}
}

func TestChain_UseIf_Disabled(t *testing.T) {
	c := New().
		UseIf("conditional", false, headerMiddleware("X-Cond", "yes"))

	handler := c.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Cond") != "" {
		t.Error("disabled middleware should not run")
	}
}

func TestChain_Names(t *testing.T) {
	c := New().
		Use("a", headerMiddleware("X-A", "1")).
		UseIf("b", false, headerMiddleware("X-B", "2")).
		Use("c", headerMiddleware("X-C", "3"))

	names := c.Names()
	if len(names) != 2 || names[0] != "a" || names[1] != "c" {
		t.Errorf("names: got %v", names)
	}
}

func TestChain_Len(t *testing.T) {
	c := New().Use("a", headerMiddleware("X", "1")).Use("b", headerMiddleware("Y", "2"))
	if c.Len() != 2 {
		t.Errorf("len: got %d", c.Len())
	}
}

func TestChain_EnabledCount(t *testing.T) {
	c := New().
		Use("a", headerMiddleware("X", "1")).
		UseIf("b", false, headerMiddleware("Y", "2")).
		Use("c", headerMiddleware("Z", "3"))

	if c.EnabledCount() != 2 {
		t.Errorf("enabled: got %d", c.EnabledCount())
	}
}

func TestChain_Merge(t *testing.T) {
	a := New().Use("recovery", headerMiddleware("X-Recovery", "1"))
	b := New().Use("logging", headerMiddleware("X-Logging", "1"))

	merged := a.Merge(b)
	if merged.Len() != 2 {
		t.Errorf("merged len: got %d", merged.Len())
	}
}

func TestChain_ThenFunc(t *testing.T) {
	c := New().Use("test", headerMiddleware("X-Test", "1"))

	handler := c.ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Test") != "1" {
		t.Error("ThenFunc should work")
	}
}

func TestChain_Order(t *testing.T) {
	var order []string
	mw := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}

	c := New().Use("first", mw("first")).Use("second", mw("second")).Use("third", mw("third"))
	handler := c.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if len(order) != 3 || order[0] != "first" || order[1] != "second" || order[2] != "third" {
		t.Errorf("order: got %v", order)
	}
}

func TestStandard(t *testing.T) {
	c := Standard(
		headerMiddleware("X-Recovery", "1"),
		headerMiddleware("X-RequestID", "1"),
		nil, // No logging.
		headerMiddleware("X-Security", "1"),
		nil, // No timeout.
	)

	if c.EnabledCount() != 3 {
		t.Errorf("enabled: got %d", c.EnabledCount())
	}
}

func TestChain_Empty(t *testing.T) {
	c := New()
	handler := c.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("empty chain: got %d", w.Code)
	}
}
