// Package compress provides HTTP response compression middleware
// supporting gzip and deflate with configurable minimum size threshold.
package compress

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Config controls compression behavior.
type Config struct {
	MinSize       int      // Minimum response size to compress (default 1024).
	Level         int      // Compression level (1-9, default 6).
	ContentTypes  []string // Content types to compress. Empty = compress all.
	SkipPaths     map[string]bool // Paths to skip compression.
}

// DefaultConfig returns production defaults.
func DefaultConfig() Config {
	return Config{
		MinSize: 1024, // 1KB minimum.
		Level:   6,
		ContentTypes: []string{
			"application/json",
			"application/problem+json",
			"text/html",
			"text/plain",
			"text/css",
			"application/javascript",
			"application/xml",
			"text/xml",
		},
	}
}

var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

// Middleware returns HTTP middleware that compresses responses.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	if cfg.MinSize <= 0 {
		cfg.MinSize = 1024
	}
	if cfg.Level < 1 || cfg.Level > 9 {
		cfg.Level = 6
	}

	typeSet := make(map[string]bool, len(cfg.ContentTypes))
	for _, ct := range cfg.ContentTypes {
		typeSet[ct] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if path excluded.
			if cfg.SkipPaths != nil && cfg.SkipPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Check Accept-Encoding.
			encoding := negotiateEncoding(r.Header.Get("Accept-Encoding"))
			if encoding == "" {
				next.ServeHTTP(w, r)
				return
			}

			cw := &compressWriter{
				ResponseWriter: w,
				encoding:       encoding,
				minSize:        cfg.MinSize,
				level:          cfg.Level,
				typeSet:        typeSet,
			}
			defer cw.Close()

			// Remove Content-Length since it will change.
			w.Header().Del("Content-Length")

			next.ServeHTTP(cw, r)
		})
	}
}

func negotiateEncoding(accept string) string {
	for _, part := range strings.Split(accept, ",") {
		enc := strings.TrimSpace(strings.Split(part, ";")[0])
		switch enc {
		case "gzip":
			return "gzip"
		case "deflate":
			return "deflate"
		}
	}
	return ""
}

type compressWriter struct {
	http.ResponseWriter
	encoding    string
	minSize     int
	level       int
	typeSet     map[string]bool
	writer      io.WriteCloser
	buf         []byte
	headerSent  bool
	shouldCompress bool
}

func (cw *compressWriter) Write(b []byte) (int, error) {
	if !cw.headerSent {
		cw.buf = append(cw.buf, b...)
		if len(cw.buf) < cw.minSize {
			return len(b), nil // Buffer until we have enough.
		}
		// flush() drains the entire buffer (including the bytes from the
		// current Write call) into the underlying writer. Returning here
		// is required — falling through to cw.writer.Write(b) below would
		// emit those same bytes a second time, doubling the response body.
		cw.flush()
		return len(b), nil
	}

	if cw.writer != nil {
		return cw.writer.Write(b)
	}
	return cw.ResponseWriter.Write(b)
}

func (cw *compressWriter) flush() {
	if cw.headerSent {
		return
	}
	cw.headerSent = true

	// Check content type.
	ct := cw.Header().Get("Content-Type")
	if ct == "" {
		ct = http.DetectContentType(cw.buf)
	}
	// Extract base type (strip charset etc).
	if idx := strings.Index(ct, ";"); idx != -1 {
		ct = strings.TrimSpace(ct[:idx])
	}

	shouldCompress := len(cw.typeSet) == 0 || cw.typeSet[ct]
	if !shouldCompress || len(cw.buf) < cw.minSize {
		// Don't compress — write buffered data directly.
		cw.ResponseWriter.Write(cw.buf)
		cw.buf = nil
		return
	}

	// Set encoding headers.
	cw.Header().Set("Content-Encoding", cw.encoding)
	cw.Header().Add("Vary", "Accept-Encoding")
	cw.shouldCompress = true

	switch cw.encoding {
	case "gzip":
		gw := gzipWriterPool.Get().(*gzip.Writer)
		gw.Reset(cw.ResponseWriter)
		cw.writer = gw
	case "deflate":
		fw, _ := flate.NewWriter(cw.ResponseWriter, cw.level)
		cw.writer = fw
	}

	if cw.writer != nil {
		cw.writer.Write(cw.buf)
	}
	cw.buf = nil
}

func (cw *compressWriter) WriteHeader(code int) {
	if !cw.headerSent && len(cw.buf) > 0 {
		cw.flush()
	}
	cw.ResponseWriter.WriteHeader(code)
}

// Close flushes remaining buffered data and closes the compressor.
func (cw *compressWriter) Close() {
	if !cw.headerSent && len(cw.buf) > 0 {
		cw.flush()
	} else if !cw.headerSent {
		// Nothing buffered — write empty.
		cw.headerSent = true
	}

	if cw.writer != nil {
		cw.writer.Close()
		if gw, ok := cw.writer.(*gzip.Writer); ok {
			gzipWriterPool.Put(gw)
		}
	}
}
