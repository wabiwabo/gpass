// Package jsonutil provides JSON utility functions for safe
// marshaling, pretty printing, and common JSON operations.
package jsonutil

import (
	"bytes"
	"encoding/json"
	"io"
)

// MustMarshal marshals to JSON, panicking on error.
func MustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic("jsonutil: " + err.Error())
	}
	return data
}

// Pretty returns indented JSON.
func Pretty(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// PrettyString returns indented JSON as string.
func PrettyString(v interface{}) string {
	data, err := Pretty(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// Compact removes whitespace from JSON.
func Compact(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Valid checks if data is valid JSON.
func Valid(data []byte) bool {
	return json.Valid(data)
}

// DecodeReader decodes JSON from a reader into v.
func DecodeReader(r io.Reader, v interface{}) error {
	return json.NewDecoder(r).Decode(v)
}

// Clone deep-clones a value via JSON round-trip.
func Clone[T any](v T) (T, error) {
	data, err := json.Marshal(v)
	if err != nil {
		var zero T
		return zero, err
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		var zero T
		return zero, err
	}
	return result, nil
}

// Merge merges multiple JSON objects. Later values override earlier.
func Merge(objects ...[]byte) ([]byte, error) {
	result := make(map[string]json.RawMessage)
	for _, obj := range objects {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(obj, &m); err != nil {
			return nil, err
		}
		for k, v := range m {
			result[k] = v
		}
	}
	return json.Marshal(result)
}

// ExtractField extracts a single field from JSON without full parsing.
func ExtractField(data []byte, field string) (json.RawMessage, bool) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, false
	}
	v, ok := m[field]
	return v, ok
}
