// Package mwcsp provides Content-Security-Policy middleware.
// Sets CSP headers to prevent XSS and data injection attacks.
// Supports API-only and web application presets.
package mwcsp

import (
	"net/http"
	"strings"
)

// Policy defines CSP directives.
type Policy struct {
	DefaultSrc []string
	ScriptSrc  []string
	StyleSrc   []string
	ImgSrc     []string
	FontSrc    []string
	ConnectSrc []string
	FrameSrc   []string
	ObjectSrc  []string
	BaseURI    []string
	FormAction []string
	ReportURI  string
}

// APIPolicy returns a strict CSP for API-only servers.
func APIPolicy() Policy {
	return Policy{
		DefaultSrc: []string{"'none'"},
		FrameSrc:   []string{"'none'"},
		BaseURI:    []string{"'none'"},
		FormAction: []string{"'none'"},
	}
}

// WebPolicy returns a reasonable CSP for web applications.
func WebPolicy() Policy {
	return Policy{
		DefaultSrc: []string{"'self'"},
		ScriptSrc:  []string{"'self'"},
		StyleSrc:   []string{"'self'", "'unsafe-inline'"},
		ImgSrc:     []string{"'self'", "data:"},
		FontSrc:    []string{"'self'"},
		ConnectSrc: []string{"'self'"},
		FrameSrc:   []string{"'none'"},
		ObjectSrc:  []string{"'none'"},
		BaseURI:    []string{"'self'"},
		FormAction: []string{"'self'"},
	}
}

// String builds the CSP header value.
func (p Policy) String() string {
	var parts []string
	add := func(directive string, values []string) {
		if len(values) > 0 {
			parts = append(parts, directive+" "+strings.Join(values, " "))
		}
	}
	add("default-src", p.DefaultSrc)
	add("script-src", p.ScriptSrc)
	add("style-src", p.StyleSrc)
	add("img-src", p.ImgSrc)
	add("font-src", p.FontSrc)
	add("connect-src", p.ConnectSrc)
	add("frame-src", p.FrameSrc)
	add("object-src", p.ObjectSrc)
	add("base-uri", p.BaseURI)
	add("form-action", p.FormAction)
	if p.ReportURI != "" {
		parts = append(parts, "report-uri "+p.ReportURI)
	}
	return strings.Join(parts, "; ")
}

// Middleware returns CSP middleware with the given policy.
func Middleware(policy Policy) func(http.Handler) http.Handler {
	header := policy.String()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Security-Policy", header)
			next.ServeHTTP(w, r)
		})
	}
}

// API returns middleware with strict API-only CSP.
func API(next http.Handler) http.Handler {
	return Middleware(APIPolicy())(next)
}

// Web returns middleware with web application CSP.
func Web(next http.Handler) http.Handler {
	return Middleware(WebPolicy())(next)
}
