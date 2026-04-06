package routegroup

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGroup_Handle(t *testing.T) {
	mux := http.NewServeMux()
	g := New(mux, "/api/v1")

	var called bool
	g.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if !called {
		t.Error("handler should be called")
	}
}

func TestGroup_Middleware(t *testing.T) {
	mux := http.NewServeMux()

	headerMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Group", "api")
			next.ServeHTTP(w, r)
		})
	}

	g := New(mux, "/api", headerMW)
	g.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Header().Get("X-Group") != "api" {
		t.Error("middleware should set header")
	}
}

func TestGroup_SubGroup(t *testing.T) {
	mux := http.NewServeMux()
	api := New(mux, "/api")
	v1 := api.SubGroup("/v1")

	var called bool
	v1.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if !called {
		t.Error("subgroup handler should be called")
	}
}

func TestGroup_SubGroup_InheritsMiddleware(t *testing.T) {
	mux := http.NewServeMux()

	parentMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Parent", "yes")
			next.ServeHTTP(w, r)
		})
	}

	api := New(mux, "/api", parentMW)
	v1 := api.SubGroup("/v1")
	v1.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Header().Get("X-Parent") != "yes" {
		t.Error("subgroup should inherit parent middleware")
	}
}

func TestGroup_Use(t *testing.T) {
	mux := http.NewServeMux()
	g := New(mux, "/api")

	g.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Used", "true")
			next.ServeHTTP(w, r)
		})
	})

	g.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Header().Get("X-Used") != "true" {
		t.Error("Use middleware should apply")
	}
}

func TestGroup_Prefix(t *testing.T) {
	mux := http.NewServeMux()
	g := New(mux, "/api/v1")
	if g.Prefix() != "/api/v1" {
		t.Errorf("Prefix = %q", g.Prefix())
	}

	sub := g.SubGroup("/users")
	if sub.Prefix() != "/api/v1/users" {
		t.Errorf("SubGroup Prefix = %q", sub.Prefix())
	}
}

func TestGroup_TrailingSlash(t *testing.T) {
	mux := http.NewServeMux()
	g := New(mux, "/api/")
	if g.Prefix() != "/api" {
		t.Errorf("should trim trailing slash: %q", g.Prefix())
	}
}
