package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRequestIDOptions_UUIDFormat(t *testing.T) {
	handler := RequestIDWithConfig(RequestIDConfig{Format: FormatUUID})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	id := w.Header().Get("X-Request-Id")
	if !isValidUUID(id) {
		t.Errorf("expected valid UUID, got %q", id)
	}
}

func TestRequestIDOptions_ULIDFormat(t *testing.T) {
	handler := RequestIDWithConfig(RequestIDConfig{Format: FormatULID})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	id := w.Header().Get("X-Request-Id")
	if len(id) != 26 {
		t.Errorf("ULID should be 26 chars, got %d: %q", len(id), id)
	}
	if !isValidULID(id) {
		t.Errorf("ULID should be valid Crockford base32, got %q", id)
	}
}

func TestRequestIDOptions_ULIDSortable(t *testing.T) {
	id1 := GenerateULID()
	// Small sleep to ensure different timestamp.
	time.Sleep(2 * time.Millisecond)
	id2 := GenerateULID()

	if id2 <= id1 {
		t.Errorf("later ULID should be greater: %q <= %q", id2, id1)
	}
}

func TestRequestIDOptions_SnowflakeContainsTimestamp(t *testing.T) {
	before := time.Now().UnixMilli() - snowflakeEpoch
	id := GenerateSnowflake(1)
	after := time.Now().UnixMilli() - snowflakeEpoch

	// Parse the numeric ID and extract timestamp (top 41 bits after shifting).
	val, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		t.Fatalf("snowflake should be numeric: %v", err)
	}

	// Extract timestamp: shift right by 23 bits (10 node + 13 seq).
	ts := val >> 23

	if ts < before || ts > after {
		t.Errorf("snowflake timestamp %d not in range [%d, %d]", ts, before, after)
	}
}

func TestRequestIDOptions_PrefixedStartsWithPrefix(t *testing.T) {
	id := GeneratePrefixed("txn")
	if !strings.HasPrefix(id, "txn_") {
		t.Errorf("expected prefix 'txn_', got %q", id)
	}

	// The part after prefix should be a valid UUID.
	uuidPart := strings.TrimPrefix(id, "txn_")
	if !isValidUUID(uuidPart) {
		t.Errorf("expected valid UUID after prefix, got %q", uuidPart)
	}
}

func TestRequestIDOptions_ConfigApplied(t *testing.T) {
	tests := []struct {
		name   string
		config RequestIDConfig
		check  func(t *testing.T, id string)
	}{
		{
			name:   "UUID",
			config: RequestIDConfig{Format: FormatUUID},
			check: func(t *testing.T, id string) {
				if !isValidUUID(id) {
					t.Errorf("expected UUID, got %q", id)
				}
			},
		},
		{
			name:   "ULID",
			config: RequestIDConfig{Format: FormatULID},
			check: func(t *testing.T, id string) {
				if len(id) != 26 {
					t.Errorf("expected 26-char ULID, got %d chars: %q", len(id), id)
				}
			},
		},
		{
			name:   "Snowflake",
			config: RequestIDConfig{Format: FormatSnowflake, NodeID: 42},
			check: func(t *testing.T, id string) {
				if _, err := strconv.ParseInt(id, 10, 64); err != nil {
					t.Errorf("expected numeric snowflake, got %q", id)
				}
			},
		},
		{
			name:   "Prefixed",
			config: RequestIDConfig{Format: FormatPrefixed, Prefix: "evt"},
			check: func(t *testing.T, id string) {
				if !strings.HasPrefix(id, "evt_") {
					t.Errorf("expected evt_ prefix, got %q", id)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RequestIDWithConfig(tt.config)(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}),
			)

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)

			id := w.Header().Get("X-Request-Id")
			tt.check(t, id)
		})
	}
}

func TestRequestIDOptions_DefaultUsesUUID(t *testing.T) {
	handler := RequestIDWithConfig(RequestIDConfig{})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	id := w.Header().Get("X-Request-Id")
	if !isValidUUID(id) {
		t.Errorf("default format should produce UUID, got %q", id)
	}
}

func TestRequestIDOptions_ExistingPreserved(t *testing.T) {
	formats := []RequestIDConfig{
		{Format: FormatUUID},
		{Format: FormatULID},
		{Format: FormatSnowflake, NodeID: 1},
		{Format: FormatPrefixed, Prefix: "req"},
	}

	for _, cfg := range formats {
		t.Run(cfg.Format.String(), func(t *testing.T) {
			handler := RequestIDWithConfig(cfg)(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}),
			)

			existingID := "existing-id-12345"
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("X-Request-Id", existingID)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)

			if got := w.Header().Get("X-Request-Id"); got != existingID {
				t.Errorf("expected preserved ID %q, got %q", existingID, got)
			}
		})
	}
}

func TestRequestIDOptions_AllFormatsUnique(t *testing.T) {
	generators := []struct {
		name string
		gen  func() string
	}{
		{"UUID", generateUUID},
		{"ULID", GenerateULID},
		{"Snowflake", func() string { return GenerateSnowflake(1) }},
		{"Prefixed", func() string { return GeneratePrefixed("req") }},
	}

	for _, g := range generators {
		t.Run(g.name, func(t *testing.T) {
			seen := make(map[string]bool, 100)
			for i := 0; i < 100; i++ {
				id := g.gen()
				if seen[id] {
					t.Fatalf("duplicate ID generated at iteration %d: %q", i, id)
				}
				seen[id] = true
			}
		})
	}
}

// String returns a name for the format (used in test names).
func (f RequestIDFormat) String() string {
	switch f {
	case FormatUUID:
		return "UUID"
	case FormatULID:
		return "ULID"
	case FormatSnowflake:
		return "Snowflake"
	case FormatPrefixed:
		return "Prefixed"
	default:
		return "Unknown"
	}
}
