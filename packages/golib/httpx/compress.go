package httpx

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// MinCompressBytes is the size threshold below which Compress passes the
// response through uncompressed. Compressing tiny payloads costs more
// CPU than it saves bytes.
const MinCompressBytes = 1024

// gzipWriterPool reuses gzip.Writer allocations across requests. The pool
// is critical for high-RPS services — gzip.NewWriter allocates ~250KB of
// internal buffers and reusing them cuts steady-state allocations to zero.
var gzipWriterPool = sync.Pool{
	New: func() any { return gzip.NewWriter(io.Discard) },
}

// gzipResponseWriter buffers writes until it knows the body exceeds
// MinCompressBytes, then either flushes the buffer through gzip or
// flushes it raw. This makes the threshold honest: a small response is
// never paid the gzip framing overhead.
type gzipResponseWriter struct {
	http.ResponseWriter
	buf       []byte
	gz        *gzip.Writer
	hijacked  bool
	statusSet bool
	wroteGzip bool
}

func (g *gzipResponseWriter) WriteHeader(code int) {
	g.statusSet = true
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if g.hijacked {
		return g.ResponseWriter.Write(b)
	}
	if g.wroteGzip {
		return g.gz.Write(b)
	}
	g.buf = append(g.buf, b...)
	if len(g.buf) >= MinCompressBytes {
		// Promote to gzip
		hdr := g.ResponseWriter.Header()
		// Don't double-encode
		if hdr.Get("Content-Encoding") == "" && !isPrecompressed(hdr.Get("Content-Type")) {
			hdr.Set("Content-Encoding", "gzip")
			hdr.Add("Vary", "Accept-Encoding")
			hdr.Del("Content-Length") // unknown after compression
			gz := gzipWriterPool.Get().(*gzip.Writer)
			gz.Reset(g.ResponseWriter)
			g.gz = gz
			g.wroteGzip = true
			n, err := gz.Write(g.buf)
			g.buf = nil
			return n, err
		}
		// Pre-compressed or has Content-Encoding already → flush raw
		g.hijacked = true
		_, _ = g.ResponseWriter.Write(g.buf)
		g.buf = nil
	}
	return len(b), nil
}

// flush emits any buffered bytes that never reached the gzip threshold.
func (g *gzipResponseWriter) flush() {
	if g.wroteGzip {
		_ = g.gz.Close()
		gzipWriterPool.Put(g.gz)
		return
	}
	if len(g.buf) > 0 {
		_, _ = g.ResponseWriter.Write(g.buf)
		g.buf = nil
	}
}

// Compress wraps h with adaptive gzip compression. Responses below
// MinCompressBytes are passed through uncompressed; larger responses are
// gzipped. Already-encoded responses (Content-Encoding set, or content
// type indicates already-compressed format) are never re-compressed.
//
// Clients without Accept-Encoding: gzip get the raw bytes — there's no
// way to negotiate something else here.
func Compress(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			h.ServeHTTP(w, r)
			return
		}
		gw := &gzipResponseWriter{ResponseWriter: w}
		defer gw.flush()
		h.ServeHTTP(gw, r)
	})
}

// isPrecompressed returns true for Content-Type values that already encode
// their own compression. Re-gzipping them wastes CPU and produces larger
// output.
func isPrecompressed(ct string) bool {
	ct = strings.ToLower(ct)
	for _, prefix := range []string{
		"image/jpeg", "image/png", "image/webp", "image/gif", "image/avif",
		"video/", "audio/",
		"application/zip", "application/gzip", "application/x-gzip",
		"application/x-bzip2", "application/x-xz", "application/x-7z",
		"application/pdf",
	} {
		if strings.HasPrefix(ct, prefix) {
			return true
		}
	}
	return false
}
