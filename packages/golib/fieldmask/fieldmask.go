// Package fieldmask implements Google-style field masks for partial
// updates. Allows clients to specify which fields to update, preventing
// unintended overwrites of unset fields in PATCH operations.
package fieldmask

import (
	"sort"
	"strings"
)

// FieldMask represents a set of fields to include in an update.
type FieldMask struct {
	paths map[string]bool
}

// New creates a FieldMask from a list of field paths.
func New(paths ...string) *FieldMask {
	fm := &FieldMask{paths: make(map[string]bool, len(paths))}
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p != "" {
			fm.paths[p] = true
		}
	}
	return fm
}

// Parse creates a FieldMask from a comma-separated string.
// e.g., "name,email,address.city"
func Parse(s string) *FieldMask {
	if strings.TrimSpace(s) == "" {
		return New()
	}
	parts := strings.Split(s, ",")
	paths := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, p)
		}
	}
	return New(paths...)
}

// Contains checks if a field path is in the mask.
func (fm *FieldMask) Contains(path string) bool {
	if fm == nil || len(fm.paths) == 0 {
		return false
	}
	return fm.paths[path]
}

// ContainsPrefix checks if any field in the mask starts with prefix.
// e.g., ContainsPrefix("address") matches "address.city", "address.zip".
func (fm *FieldMask) ContainsPrefix(prefix string) bool {
	if fm == nil || len(fm.paths) == 0 {
		return false
	}
	dotPrefix := prefix + "."
	for p := range fm.paths {
		if p == prefix || strings.HasPrefix(p, dotPrefix) {
			return true
		}
	}
	return false
}

// Paths returns all field paths sorted alphabetically.
func (fm *FieldMask) Paths() []string {
	if fm == nil || len(fm.paths) == 0 {
		return nil
	}
	result := make([]string, 0, len(fm.paths))
	for p := range fm.paths {
		result = append(result, p)
	}
	sort.Strings(result)
	return result
}

// IsEmpty returns true if the mask has no paths.
func (fm *FieldMask) IsEmpty() bool {
	return fm == nil || len(fm.paths) == 0
}

// Len returns the number of paths in the mask.
func (fm *FieldMask) Len() int {
	if fm == nil {
		return 0
	}
	return len(fm.paths)
}

// Add adds a path to the mask.
func (fm *FieldMask) Add(path string) {
	path = strings.TrimSpace(path)
	if path != "" {
		fm.paths[path] = true
	}
}

// Remove removes a path from the mask.
func (fm *FieldMask) Remove(path string) {
	delete(fm.paths, path)
}

// Union returns a new FieldMask containing all paths from both masks.
func (fm *FieldMask) Union(other *FieldMask) *FieldMask {
	result := New(fm.Paths()...)
	if other != nil {
		for _, p := range other.Paths() {
			result.Add(p)
		}
	}
	return result
}

// Intersect returns a new FieldMask containing only paths in both masks.
func (fm *FieldMask) Intersect(other *FieldMask) *FieldMask {
	result := New()
	if fm == nil || other == nil {
		return result
	}
	for p := range fm.paths {
		if other.Contains(p) {
			result.Add(p)
		}
	}
	return result
}

// String returns the comma-separated field mask string.
func (fm *FieldMask) String() string {
	paths := fm.Paths()
	if len(paths) == 0 {
		return ""
	}
	return strings.Join(paths, ",")
}

// Validate checks that all paths in the mask are in the allowed set.
// Returns the first invalid path, or empty string if all valid.
func Validate(fm *FieldMask, allowed []string) string {
	if fm == nil || fm.IsEmpty() {
		return ""
	}
	set := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		set[a] = true
	}
	for _, p := range fm.Paths() {
		if !set[p] {
			return p
		}
	}
	return ""
}

// Filter returns a map containing only the fields present in the mask.
func Filter(data map[string]interface{}, fm *FieldMask) map[string]interface{} {
	if fm == nil || fm.IsEmpty() {
		return data
	}
	result := make(map[string]interface{})
	for key, val := range data {
		if fm.Contains(key) {
			result[key] = val
		}
	}
	return result
}
