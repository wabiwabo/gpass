// Package mwgzip provides gzip compression middleware for HTTP
// responses. Compresses responses when the client supports gzip
// and the response size exceeds a minimum threshold.
package mwgzip

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Config controls gzip compression.
type Config struct {
	// Level is the gzip compression level (1-9, default 6).
	Level int
	// MinSize is the minimum response size to compress (default 1024).
	MinSize int
	// ContentTypes to compress (empty = compress all).
	ContentTypes []string
}

// DefaultConfig returns production defaults.
func DefaultConfig() Config {
	return Config{
		Level:   gzip.DefaultCompression,
		MinSize: 1024,
		ContentTypes: []string{
			"application/json",
			"text/html",
			"text/plain",
			"text/css",
			"application/javascript",
		},
	}
}

var gzipPool = sync.Pool{
	New: func() interface{} {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

type gzipWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	wroteHeader bool
	minSize     int
	buf         []byte
	compressed  bool
	types       map[string]bool
}

func (w *gzipWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.buf = append(w.buf, b...)
		if len(w.buf) < w.minSize {
			return len(b), nil
		}
		w.flush()
		return len(b), nil
	}
	if w.compressed {
		return w.gz.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func (w *gzipWriter) flush() {
	w.wroteHeader = true

	ct := w.ResponseWriter.Header().Get("Content-Type")
	shouldCompress := len(w.types) == 0
	if !shouldCompress {
		for t := range w.types {
			if strings.HasPrefix(ct, t) {
				shouldCompress = true
				break
			}
		}
	}

	if shouldCompress && len(w.buf) >= w.minSize {
		w.compressed = true
		w.ResponseWriter.Header().Set("Content-Encoding", "gzip")
		w.ResponseWriter.Header().Del("Content-Length")
		w.gz.Reset(w.ResponseWriter)
		w.gz.Write(w.buf)
	} else {
		w.ResponseWriter.Write(w.buf)
	}
}

func (w *gzipWriter) close() {
	if !w.wroteHeader && len(w.buf) > 0 {
		w.flush()
	}
	if w.compressed {
		w.gz.Close()
	}
}

// Middleware returns gzip compression middleware.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	if cfg.Level < 1 || cfg.Level > 9 {
		cfg.Level = gzip.DefaultCompression
	}
	if cfg.MinSize <= 0 {
		cfg.MinSize = 1024
	}

	typeSet := make(map[string]bool, len(cfg.ContentTypes))
	for _, t := range cfg.ContentTypes {
		typeSet[t] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			gz := gzipPool.Get().(*gzip.Writer)
			defer gzipPool.Put(gz)

			gw := &gzipWriter{
				ResponseWriter: w,
				gz:             gz,
				minSize:        cfg.MinSize,
				types:          typeSet,
			}
			defer gw.close()

			next.ServeHTTP(gw, r)
		})
	}
}
