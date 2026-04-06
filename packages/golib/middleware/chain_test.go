package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChain_AppliesInCorrectOrder(t *testing.T) {
	var order []string

	mwA := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "A-before")
			next.ServeHTTP(w, r)
			order = append(order, "A-after")
		})
	}
	mwB := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "B-before")
			next.ServeHTTP(w, r)
			order = append(order, "B-after")
		})
	}
	mwC := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "C-before")
			next.ServeHTTP(w, r)
			order = append(order, "C-after")
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	})

	chain := Chain(handler, mwA, mwB, mwC)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	expected := []string{"A-before", "B-before", "C-before", "handler", "C-after", "B-after", "A-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(order), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("position %d: expected %q, got %q", i, v, order[i])
		}
	}
}

func TestChain_EmptyReturnsHandlerUnchanged(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	chain := Chain(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if !called {
		t.Error("handler was not called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestChainFunc_WorksWithHandlerFunc(t *testing.T) {
	headerMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Chain-Test", "applied")
			next.ServeHTTP(w, r)
		})
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	chain := ChainFunc(handler, headerMW)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if w.Header().Get("X-Chain-Test") != "applied" {
		t.Error("middleware was not applied via ChainFunc")
	}
}

func TestBuilder_UseAddsMiddleware(t *testing.T) {
	var order []string

	mwA := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "A")
			next.ServeHTTP(w, r)
		})
	}
	mwB := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "B")
			next.ServeHTTP(w, r)
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	})

	chain := NewBuilder().Use(mwA).Use(mwB).Build(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	expected := []string{"A", "B", "handler"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(order), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("position %d: expected %q, got %q", i, v, order[i])
		}
	}
}

func TestBuilder_UseIfTrue_AddsMiddleware(t *testing.T) {
	applied := false
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			applied = true
			next.ServeHTTP(w, r)
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	chain := NewBuilder().UseIf(true, mw).Build(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if !applied {
		t.Error("UseIf(true) should add middleware")
	}
}

func TestBuilder_UseIfFalse_SkipsMiddleware(t *testing.T) {
	applied := false
	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			applied = true
			next.ServeHTTP(w, r)
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	chain := NewBuilder().UseIf(false, mw).Build(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if applied {
		t.Error("UseIf(false) should skip middleware")
	}
}

func TestBuilder_Build_ComposesAll(t *testing.T) {
	headerMW := func(key, value string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set(key, value)
				next.ServeHTTP(w, r)
			})
		}
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	chain := NewBuilder().
		Use(headerMW("X-First", "1")).
		UseIf(true, headerMW("X-Second", "2")).
		UseIf(false, headerMW("X-Skipped", "no")).
		Use(headerMW("X-Third", "3")).
		Build(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	chain.ServeHTTP(w, req)

	if w.Header().Get("X-First") != "1" {
		t.Error("X-First header missing")
	}
	if w.Header().Get("X-Second") != "2" {
		t.Error("X-Second header missing")
	}
	if w.Header().Get("X-Skipped") != "" {
		t.Error("X-Skipped header should not be set")
	}
	if w.Header().Get("X-Third") != "3" {
		t.Error("X-Third header missing")
	}
}
