// Package etag provides HTTP conditional request handling with ETag
// and Last-Modified support. Reduces bandwidth by returning 304 Not
// Modified when content hasn't changed.
package etag

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Generate creates an ETag from content bytes.
// Returns a weak ETag (W/"hash") by default.
func Generate(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf(`W/"%s"`, hex.EncodeToString(h[:8]))
}

// GenerateStrong creates a strong ETag from content bytes.
func GenerateStrong(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf(`"%s"`, hex.EncodeToString(h[:16]))
}

// Match checks if a client's If-None-Match header matches the current ETag.
func Match(ifNoneMatch, currentETag string) bool {
	if ifNoneMatch == "" || currentETag == "" {
		return false
	}
	if ifNoneMatch == "*" {
		return true
	}

	// Handle comma-separated ETags.
	for _, tag := range strings.Split(ifNoneMatch, ",") {
		tag = strings.TrimSpace(tag)
		// Compare weak and strong ETags (weak comparison per RFC 7232).
		if weakEqual(tag, currentETag) {
			return true
		}
	}
	return false
}

// weakEqual compares ETags using weak comparison (ignoring W/ prefix).
func weakEqual(a, b string) bool {
	return stripWeak(a) == stripWeak(b)
}

func stripWeak(etag string) string {
	return strings.TrimPrefix(etag, "W/")
}

// NotModified checks if the request can be served with 304.
// Checks both If-None-Match and If-Modified-Since.
func NotModified(r *http.Request, etag string, lastModified time.Time) bool {
	// ETag takes priority per RFC 7232.
	if inm := r.Header.Get("If-None-Match"); inm != "" {
		return Match(inm, etag)
	}

	if ims := r.Header.Get("If-Modified-Since"); ims != "" {
		t, err := http.ParseTime(ims)
		if err != nil {
			return false
		}
		return !lastModified.After(t)
	}

	return false
}

// SetHeaders sets ETag and Last-Modified response headers.
func SetHeaders(w http.ResponseWriter, etag string, lastModified time.Time) {
	if etag != "" {
		w.Header().Set("ETag", etag)
	}
	if !lastModified.IsZero() {
		w.Header().Set("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
	}
}

// Middleware returns HTTP middleware that handles conditional requests.
// The handler function should set the ETag header before writing the body.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply to safe methods.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}

		cw := &conditionalWriter{
			ResponseWriter: w,
			request:        r,
		}
		next.ServeHTTP(cw, r)
	})
}

type conditionalWriter struct {
	http.ResponseWriter
	request     *http.Request
	wroteHeader bool
}

func (cw *conditionalWriter) WriteHeader(code int) {
	if cw.wroteHeader {
		return
	}
	cw.wroteHeader = true

	// Check for 304 before writing.
	if code == http.StatusOK {
		etag := cw.Header().Get("ETag")
		if etag != "" && Match(cw.request.Header.Get("If-None-Match"), etag) {
			cw.ResponseWriter.WriteHeader(http.StatusNotModified)
			return
		}
	}

	cw.ResponseWriter.WriteHeader(code)
}

func (cw *conditionalWriter) Write(b []byte) (int, error) {
	if !cw.wroteHeader {
		cw.WriteHeader(http.StatusOK)
	}
	return cw.ResponseWriter.Write(b)
}
