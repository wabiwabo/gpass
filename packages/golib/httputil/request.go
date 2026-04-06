package httputil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const maxBodySize = 1 << 20 // 1MB

// UserID extracts the X-User-ID header from the request.
// Returns an error if the header is empty or missing.
func UserID(r *http.Request) (string, error) {
	id := r.Header.Get("X-User-ID")
	if id == "" {
		return "", errors.New("missing X-User-ID header")
	}
	return id, nil
}

// DecodeJSON decodes the JSON body of the request into v.
// It enforces a maximum body size of 1MB.
func DecodeJSON(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return errors.New("request body is empty")
	}

	limited := io.LimitReader(r.Body, maxBodySize+1)
	decoder := json.NewDecoder(limited)

	if err := decoder.Decode(v); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return errors.New("request body is empty")
		}
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			return fmt.Errorf("malformed JSON at position %d", syntaxErr.Offset)
		}
		var unmarshalErr *json.UnmarshalTypeError
		if errors.As(err, &unmarshalErr) {
			return fmt.Errorf("invalid value for field %q", unmarshalErr.Field)
		}
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Check if the body exceeded the limit by trying to read one more byte.
	buf := make([]byte, 1)
	if n, _ := limited.Read(buf); n > 0 {
		return errors.New("request body too large")
	}

	return nil
}

// PathParam extracts a path parameter by name using the standard library's
// http.Request.PathValue method.
func PathParam(r *http.Request, name string) string {
	return r.PathValue(name)
}
