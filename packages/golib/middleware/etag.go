package middleware

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
)

// ETag returns middleware that adds weak ETag headers and handles conditional
// requests. For GET requests, if the client sends an If-None-Match header that
// matches the computed ETag, a 304 Not Modified is returned with no body.
// The ETag is computed as the SHA-256 hash of the response body.
func ETag(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only compute ETags for GET and HEAD requests.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}

		rec := &etagRecorder{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
			statusCode:     http.StatusOK,
		}
		next.ServeHTTP(rec, r)

		body := rec.body.Bytes()
		hash := sha256.Sum256(body)
		etag := fmt.Sprintf(`W/"%x"`, hash)

		if matchesETag(r.Header.Get("If-None-Match"), etag) {
			w.Header().Set("ETag", etag)
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("ETag", etag)
		w.WriteHeader(rec.statusCode)
		w.Write(body)
	})
}

// StrongETag returns middleware with strong ETags suitable for static content.
// Strong ETags use the format "<hash>" without the W/ prefix.
func StrongETag(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}

		rec := &etagRecorder{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
			statusCode:     http.StatusOK,
		}
		next.ServeHTTP(rec, r)

		body := rec.body.Bytes()
		hash := sha256.Sum256(body)
		etag := fmt.Sprintf(`"%x"`, hash)

		if matchesETag(r.Header.Get("If-None-Match"), etag) {
			w.Header().Set("ETag", etag)
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("ETag", etag)
		w.WriteHeader(rec.statusCode)
		w.Write(body)
	})
}

// matchesETag checks whether the etag matches any value in the If-None-Match
// header. Supports comma-separated values and the wildcard "*".
func matchesETag(ifNoneMatch, etag string) bool {
	if ifNoneMatch == "" {
		return false
	}

	parts := strings.Split(ifNoneMatch, ",")
	for _, part := range parts {
		candidate := strings.TrimSpace(part)
		if candidate == "*" {
			return true
		}
		if candidate == etag {
			return true
		}
	}
	return false
}

// etagRecorder captures the response body and status code so the ETag can be
// computed before writing to the client.
type etagRecorder struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
	written    bool
}

func (r *etagRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.written = true
}

func (r *etagRecorder) Write(b []byte) (int, error) {
	if !r.written {
		r.written = true
	}
	return r.body.Write(b)
}
