// Package sqlparam provides safe SQL parameter building for
// dynamic queries. Prevents SQL injection by using parameterized
// placeholders ($1, $2) and never interpolating values into SQL.
package sqlparam

import (
	"fmt"
	"strings"
)

// Builder constructs parameterized SQL queries.
type Builder struct {
	sql    strings.Builder
	params []interface{}
}

// New creates a query builder with an initial SQL fragment.
func New(sql string) *Builder {
	b := &Builder{}
	b.sql.WriteString(sql)
	return b
}

// Param adds a parameter and returns its placeholder ($N).
func (b *Builder) Param(value interface{}) string {
	b.params = append(b.params, value)
	return fmt.Sprintf("$%d", len(b.params))
}

// Write appends raw SQL text.
func (b *Builder) Write(sql string) *Builder {
	b.sql.WriteString(sql)
	return b
}

// WriteParam appends SQL with an inline parameter placeholder.
func (b *Builder) WriteParam(sql string, value interface{}) *Builder {
	placeholder := b.Param(value)
	b.sql.WriteString(strings.Replace(sql, "?", placeholder, 1))
	return b
}

// SQL returns the built query string.
func (b *Builder) SQL() string {
	return b.sql.String()
}

// Params returns all parameters in order.
func (b *Builder) Params() []interface{} {
	return b.params
}

// ParamCount returns the number of parameters.
func (b *Builder) ParamCount() int {
	return len(b.params)
}

// WhereIn builds a WHERE column IN ($1, $2, ...) clause.
func WhereIn(column string, values []interface{}) (string, []interface{}) {
	if len(values) == 0 {
		return "FALSE", nil
	}
	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	return fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ", ")), values
}

// InsertColumns generates INSERT INTO table (cols) VALUES ($1, $2, ...).
func InsertColumns(table string, columns []string) string {
	placeholders := make([]string, len(columns))
	for i := range columns {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)
}

// UpdateSet generates SET col1 = $1, col2 = $2 for UPDATE queries.
func UpdateSet(columns []string, startParam int) string {
	parts := make([]string, len(columns))
	for i, col := range columns {
		parts[i] = fmt.Sprintf("%s = $%d", col, startParam+i)
	}
	return strings.Join(parts, ", ")
}
