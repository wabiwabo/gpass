package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// compressedContentTypes lists content types that are already compressed
// and should not be compressed again.
var compressedContentTypes = map[string]bool{
	"image/png":              true,
	"image/jpeg":             true,
	"image/gif":              true,
	"image/webp":             true,
	"video/mp4":              true,
	"video/webm":             true,
	"audio/mpeg":             true,
	"application/zip":        true,
	"application/gzip":       true,
	"application/x-gzip":     true,
	"application/octet-stream": true,
}

var gzipWriterPool = sync.Pool{
	New: func() any {
		return gzip.NewWriter(io.Discard)
	},
}

// gzipResponseWriter wraps http.ResponseWriter to buffer and conditionally
// compress the response body.
type gzipResponseWriter struct {
	http.ResponseWriter
	gw         *gzip.Writer
	minSize    int
	buf        []byte
	statusCode int
	written    bool
	compressed bool
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.statusCode = code
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	w.buf = append(w.buf, b...)
	return len(b), nil
}

// flush writes the buffered data to the underlying ResponseWriter,
// compressing if the conditions are met.
func (w *gzipResponseWriter) flush() {
	if w.written {
		return
	}
	w.written = true

	ct := w.Header().Get("Content-Type")
	if ct == "" {
		ct = http.DetectContentType(w.buf)
		w.Header().Set("Content-Type", ct)
	}

	// Check if content type is already compressed
	baseCT := ct
	if idx := strings.Index(ct, ";"); idx != -1 {
		baseCT = strings.TrimSpace(ct[:idx])
	}

	shouldCompress := len(w.buf) > w.minSize && !compressedContentTypes[baseCT]

	if shouldCompress {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
		w.compressed = true

		if w.statusCode == 0 {
			w.statusCode = http.StatusOK
		}
		w.ResponseWriter.WriteHeader(w.statusCode)

		w.gw.Reset(w.ResponseWriter)
		w.gw.Write(w.buf)
		w.gw.Close()
	} else {
		if w.statusCode == 0 {
			w.statusCode = http.StatusOK
		}
		w.ResponseWriter.WriteHeader(w.statusCode)
		w.ResponseWriter.Write(w.buf)
	}
}

// Compress returns middleware that gzip-compresses responses when the client
// supports it (Accept-Encoding: gzip) and the response is large enough.
// Only compresses responses > minSize bytes. Sets Content-Encoding: gzip.
// Skips compression for already-compressed content types (images, video).
func Compress(minSize int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			gw := gzipWriterPool.Get().(*gzip.Writer)
			defer gzipWriterPool.Put(gw)

			grw := &gzipResponseWriter{
				ResponseWriter: w,
				gw:             gw,
				minSize:        minSize,
			}

			next.ServeHTTP(grw, r)
			grw.flush()
		})
	}
}
