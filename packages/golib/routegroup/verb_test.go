package routegroup

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestVerbHelpers_RouteThroughPrefix is a regression test for a bug
// discovered while bumping coverage: Group.GET/POST/etc. expand to
// "VERB "+path inside Handle, which then concatenates the group prefix
// in front, producing patterns like "/apiGET /users" — invalid.
//
// The correct pattern is "VERB /api/users". This test pins the working
// behavior so future refactors can't regress it.
func TestVerbHelpers_RouteThroughPrefix(t *testing.T) {
	cases := []struct {
		method string
		register func(g *Group, path string, fn http.HandlerFunc)
	}{
		{http.MethodGet, func(g *Group, p string, fn http.HandlerFunc) { g.GET(p, fn) }},
		{http.MethodPost, func(g *Group, p string, fn http.HandlerFunc) { g.POST(p, fn) }},
		{http.MethodPut, func(g *Group, p string, fn http.HandlerFunc) { g.PUT(p, fn) }},
		{http.MethodPatch, func(g *Group, p string, fn http.HandlerFunc) { g.PATCH(p, fn) }},
		{http.MethodDelete, func(g *Group, p string, fn http.HandlerFunc) { g.DELETE(p, fn) }},
	}

	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			mux := http.NewServeMux()
			g := New(mux, "/api")
			called := false
			tc.register(g, "/users", func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(tc.method, "/api/users", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("status = %d, want 200 (mux did not match)", w.Code)
			}
			if !called {
				t.Errorf("handler was not invoked")
			}
		})
	}
}

// TestVerbHelpers_WrongMethod ensures the Go 1.22 method-routed mux
// rejects mismatched methods with 405.
func TestVerbHelpers_WrongMethod(t *testing.T) {
	mux := http.NewServeMux()
	g := New(mux, "/api")
	g.GET("/things", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/things", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST on GET-only route: status = %d, want 405", w.Code)
	}
}
