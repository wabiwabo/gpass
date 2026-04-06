package httpchain

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouter_GET(t *testing.T) {
	r := NewRouter("/api/v1")
	r.GET("/users", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("users"))
	})

	mux := http.NewServeMux()
	r.Mount(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET: got %d", w.Code)
	}
	if w.Body.String() != "users" {
		t.Errorf("body: got %q", w.Body.String())
	}
}

func TestRouter_POST(t *testing.T) {
	r := NewRouter("/api/v1")
	r.POST("/users", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	mux := http.NewServeMux()
	r.Mount(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("POST: got %d", w.Code)
	}
}

func TestRouter_AllMethods(t *testing.T) {
	r := NewRouter("")
	r.GET("/r", func(w http.ResponseWriter, req *http.Request) { w.WriteHeader(200) })
	r.POST("/r", func(w http.ResponseWriter, req *http.Request) { w.WriteHeader(201) })
	r.PUT("/r", func(w http.ResponseWriter, req *http.Request) { w.WriteHeader(200) })
	r.PATCH("/r", func(w http.ResponseWriter, req *http.Request) { w.WriteHeader(200) })
	r.DELETE("/r", func(w http.ResponseWriter, req *http.Request) { w.WriteHeader(204) })

	if r.Count() != 5 {
		t.Errorf("routes: got %d", r.Count())
	}
}

func TestRouter_PerRouteMiddleware(t *testing.T) {
	authMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Auth", "checked")
			next.ServeHTTP(w, r)
		})
	}

	r := NewRouter("/api")
	r.GET("/public", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.GET("/private", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, authMW)

	mux := http.NewServeMux()
	r.Mount(mux)

	// Public — no auth header.
	req := httptest.NewRequest(http.MethodGet, "/api/public", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Header().Get("X-Auth") != "" {
		t.Error("public should not have auth")
	}

	// Private — auth header.
	req = httptest.NewRequest(http.MethodGet, "/api/private", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Header().Get("X-Auth") != "checked" {
		t.Error("private should have auth")
	}
}

func TestRouter_GlobalMiddleware(t *testing.T) {
	r := NewRouter("/api")
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Global", "yes")
			next.ServeHTTP(w, req)
		})
	})
	r.GET("/test", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux := http.NewServeMux()
	r.Mount(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Header().Get("X-Global") != "yes" {
		t.Error("global middleware should apply")
	}
}

func TestRouter_Group(t *testing.T) {
	r := NewRouter("/api/v1")
	admin := r.Group("/admin")
	admin.GET("/users", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux := http.NewServeMux()
	admin.Mount(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("group: got %d", w.Code)
	}
}

func TestRouter_Routes(t *testing.T) {
	r := NewRouter("/api")
	r.GET("/a", func(w http.ResponseWriter, req *http.Request) {})
	r.POST("/b", func(w http.ResponseWriter, req *http.Request) {})

	routes := r.Routes()
	if len(routes) != 2 {
		t.Errorf("routes: got %d", len(routes))
	}

	// Verify it's a copy.
	routes[0].Pattern = "mutated"
	if r.Routes()[0].Pattern == "mutated" {
		t.Error("Routes() should return a copy")
	}
}

func TestRouter_EmptyPrefix(t *testing.T) {
	r := NewRouter("")
	r.GET("/health", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux := http.NewServeMux()
	r.Mount(mux)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("empty prefix: got %d", w.Code)
	}
}
