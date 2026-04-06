// Package pgxutil provides PostgreSQL query utility functions.
// Helpers for building safe queries, handling NULL values, and
// common PostgreSQL patterns used across GarudaPass services.
package pgxutil

import (
	"fmt"
	"strings"
	"time"
)

// Placeholder generates a PostgreSQL placeholder ($N).
func Placeholder(n int) string {
	return fmt.Sprintf("$%d", n)
}

// Placeholders generates N placeholders starting from start.
// e.g., Placeholders(1, 3) = "$1, $2, $3"
func Placeholders(start, count int) string {
	parts := make([]string, count)
	for i := 0; i < count; i++ {
		parts[i] = fmt.Sprintf("$%d", start+i)
	}
	return strings.Join(parts, ", ")
}

// NullString returns a pointer to string for nullable fields.
// Returns nil for empty strings.
func NullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// NullInt returns a pointer to int for nullable fields.
func NullInt(n int) *int {
	if n == 0 {
		return nil
	}
	return &n
}

// NullTime returns a pointer to time for nullable fields.
func NullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// DerefString returns the string value or empty if nil.
func DerefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// DerefInt returns the int value or 0 if nil.
func DerefInt(n *int) int {
	if n == nil {
		return 0
	}
	return *n
}

// DerefTime returns the time value or zero time if nil.
func DerefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

// OrderByClause builds a safe ORDER BY clause from allowed columns.
func OrderByClause(column, direction string, allowed []string) string {
	// Validate column against allowed list to prevent injection
	valid := false
	for _, a := range allowed {
		if column == a {
			valid = true
			break
		}
	}
	if !valid {
		return ""
	}

	dir := "ASC"
	if strings.EqualFold(direction, "desc") {
		dir = "DESC"
	}
	return fmt.Sprintf("ORDER BY %s %s", column, dir)
}

// LimitOffset builds a LIMIT/OFFSET clause.
func LimitOffset(limit, offset int) string {
	if limit <= 0 {
		limit = 20
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}
	return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
}

// ILike generates a case-insensitive LIKE pattern.
// Escapes % and _ in the input to prevent wildcard injection.
func ILike(value string) string {
	value = strings.ReplaceAll(value, "%", "\\%")
	value = strings.ReplaceAll(value, "_", "\\_")
	return "%" + value + "%"
}
