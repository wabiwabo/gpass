package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
)

// RequestTransformer modifies incoming requests before they reach the handler.
type RequestTransformer func(r *http.Request) *http.Request

// ResponseTransformer modifies outgoing responses before they reach the client.
type ResponseTransformer func(status int, headers http.Header, body []byte) (int, http.Header, []byte)

// TransformRequest returns middleware that applies request transformers in order.
func TransformRequest(transformers ...RequestTransformer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, t := range transformers {
				r = t(r)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// TransformResponse returns middleware that applies response transformers in order.
func TransformResponse(transformers ...ResponseTransformer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &responseBuffer{
				header: make(http.Header),
			}
			next.ServeHTTP(rec, r)

			status := rec.status
			headers := rec.header
			body := rec.body.Bytes()

			for _, t := range transformers {
				status, headers, body = t(status, headers, body)
			}

			// Copy transformed headers to the real response writer
			for k, vals := range headers {
				for _, v := range vals {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(status)
			w.Write(body)
		})
	}
}

// StripPrefix removes a path prefix from the request URL.
func StripPrefix(prefix string) RequestTransformer {
	return func(r *http.Request) *http.Request {
		r2 := r.Clone(r.Context())
		r2.URL.Path = strings.TrimPrefix(r2.URL.Path, prefix)
		if r2.URL.Path == "" {
			r2.URL.Path = "/"
		}
		if r2.URL.RawPath != "" {
			r2.URL.RawPath = strings.TrimPrefix(r2.URL.RawPath, prefix)
			if r2.URL.RawPath == "" {
				r2.URL.RawPath = "/"
			}
		}
		return r2
	}
}

// AddHeader adds a header to all requests.
func AddHeader(key, value string) RequestTransformer {
	return func(r *http.Request) *http.Request {
		r2 := r.Clone(r.Context())
		r2.Header.Set(key, value)
		return r2
	}
}

// RedactResponseField removes a field from JSON response bodies.
// Non-JSON responses are returned unmodified.
func RedactResponseField(field string) ResponseTransformer {
	return func(status int, headers http.Header, body []byte) (int, http.Header, []byte) {
		ct := headers.Get("Content-Type")
		if !strings.Contains(ct, "application/json") {
			return status, headers, body
		}

		var m map[string]interface{}
		if err := json.Unmarshal(body, &m); err != nil {
			return status, headers, body
		}

		delete(m, field)

		newBody, err := json.Marshal(m)
		if err != nil {
			return status, headers, body
		}

		return status, headers, newBody
	}
}

// AddResponseHeader adds a header to all responses.
func AddResponseHeader(key, value string) ResponseTransformer {
	return func(status int, headers http.Header, body []byte) (int, http.Header, []byte) {
		headers.Set(key, value)
		return status, headers, body
	}
}

// responseBuffer captures the response for transformation.
type responseBuffer struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (r *responseBuffer) Header() http.Header {
	return r.header
}

func (r *responseBuffer) Write(b []byte) (int, error) {
	return r.body.Write(b)
}

func (r *responseBuffer) WriteHeader(statusCode int) {
	r.status = statusCode
}
