package responsecache

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Config holds the configuration for the response cache.
type Config struct {
	DefaultTTL time.Duration
	MaxEntries int
	SkipPaths  []string
}

// CacheStats holds cache statistics.
type CacheStats struct {
	Hits      int64
	Misses    int64
	Entries   int64
	Evictions int64
}

// cacheEntry stores a cached HTTP response.
type cacheEntry struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	ExpiresAt  time.Time
	ETag       string
	CreatedAt  time.Time
}

// Cache is an in-memory HTTP response cache.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	cfg     Config

	hits      int64
	misses    int64
	evictions int64
}

// New creates a new Cache with the given configuration.
func New(cfg Config) *Cache {
	return &Cache{
		entries: make(map[string]*cacheEntry),
		cfg:     cfg,
	}
}

// Middleware returns an HTTP middleware that caches GET responses.
func (c *Cache) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only cache GET requests.
			if r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}

			// Skip configured paths.
			for _, p := range c.cfg.SkipPaths {
				if r.URL.Path == p {
					next.ServeHTTP(w, r)
					return
				}
			}

			key := r.URL.RequestURI()

			// Check for cache hit.
			c.mu.RLock()
			entry, ok := c.entries[key]
			c.mu.RUnlock()

			if ok && time.Now().Before(entry.ExpiresAt) {
				atomic.AddInt64(&c.hits, 1)

				// Handle conditional request.
				if inm := r.Header.Get("If-None-Match"); inm == entry.ETag {
					w.Header().Set("ETag", entry.ETag)
					w.Header().Set("X-Cache", "HIT")
					w.WriteHeader(http.StatusNotModified)
					return
				}

				// Serve from cache.
				for k, vals := range entry.Headers {
					for _, v := range vals {
						w.Header().Add(k, v)
					}
				}
				ttlSecs := int(time.Until(entry.ExpiresAt).Seconds())
				if ttlSecs < 0 {
					ttlSecs = 0
				}
				w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", ttlSecs))
				w.Header().Set("ETag", entry.ETag)
				w.Header().Set("X-Cache", "HIT")
				w.WriteHeader(entry.StatusCode)
				w.Write(entry.Body)
				return
			}

			// Cache miss: capture response.
			atomic.AddInt64(&c.misses, 1)
			rec := &responseRecorder{
				ResponseWriter: w,
				body:           &bytes.Buffer{},
				statusCode:     http.StatusOK,
			}
			next.ServeHTTP(rec, r)

			// Store in cache.
			body := rec.body.Bytes()
			hash := sha256.Sum256(body)
			etag := fmt.Sprintf(`"%x"`, hash)
			ttlSecs := int(c.cfg.DefaultTTL.Seconds())

			now := time.Now()
			newEntry := &cacheEntry{
				StatusCode: rec.statusCode,
				Headers:    rec.Header().Clone(),
				Body:       body,
				ExpiresAt:  now.Add(c.cfg.DefaultTTL),
				ETag:       etag,
				CreatedAt:  now,
			}

			c.mu.Lock()
			// Evict if at capacity (before inserting).
			if c.cfg.MaxEntries > 0 && len(c.entries) >= c.cfg.MaxEntries {
				if _, exists := c.entries[key]; !exists {
					c.evictOldest()
				}
			}
			c.entries[key] = newEntry
			c.mu.Unlock()

			// Set cache headers on the actual response.
			w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", ttlSecs))
			w.Header().Set("ETag", etag)
			w.Header().Set("X-Cache", "MISS")
		})
	}
}

// evictOldest removes the oldest entry. Must be called with c.mu held.
func (c *Cache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true

	for k, e := range c.entries {
		if first || e.CreatedAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = e.CreatedAt
			first = false
		}
	}

	if !first {
		delete(c.entries, oldestKey)
		atomic.AddInt64(&c.evictions, 1)
	}
}

// Invalidate removes a specific path from the cache.
func (c *Cache) Invalidate(path string) {
	c.mu.Lock()
	delete(c.entries, path)
	c.mu.Unlock()
}

// InvalidatePrefix removes all cached entries whose key starts with the given prefix.
func (c *Cache) InvalidatePrefix(prefix string) {
	c.mu.Lock()
	// Collect keys first to avoid modifying map during iteration.
	var keys []string
	for k := range c.entries {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	for _, k := range keys {
		delete(c.entries, k)
	}
	c.mu.Unlock()
}

// Flush clears the entire cache.
func (c *Cache) Flush() {
	c.mu.Lock()
	c.entries = make(map[string]*cacheEntry)
	c.mu.Unlock()
}

// Stats returns current cache statistics.
func (c *Cache) Stats() CacheStats {
	c.mu.RLock()
	entries := int64(len(c.entries))
	c.mu.RUnlock()

	return CacheStats{
		Hits:      atomic.LoadInt64(&c.hits),
		Misses:    atomic.LoadInt64(&c.misses),
		Entries:   entries,
		Evictions: atomic.LoadInt64(&c.evictions),
	}
}

// responseRecorder captures the response for caching.
type responseRecorder struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
	wroteHeader bool
}

func (r *responseRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.statusCode = code
		r.wroteHeader = true
		r.ResponseWriter.WriteHeader(code)
	}
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// sortedKeys returns sorted keys for deterministic iteration. Used in tests.
func sortedKeys(m map[string]*cacheEntry) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
