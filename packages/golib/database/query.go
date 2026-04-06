package database

import (
	"fmt"
	"strings"
)

// QueryBuilder constructs SQL queries with parameterized arguments.
// All values are passed as parameters ($1, $2, ...) — never interpolated.
type QueryBuilder struct {
	operation string // SELECT, INSERT, UPDATE, DELETE
	table     string
	columns   []string
	wheres    []whereClause
	orderBy   string
	orderDir  string
	limit     int
	offset    int
	args      []interface{}
	returning []string
	sets      []setClause
	values    []interface{} // for INSERT
}

type whereClause struct {
	column string
	op     string // =, !=, >, <, >=, <=, LIKE, IN, IS NULL, IS NOT NULL
}

type setClause struct {
	column string
}

// Select starts a SELECT query.
func Select(columns ...string) *QueryBuilder {
	return &QueryBuilder{
		operation: "SELECT",
		columns:   columns,
	}
}

// From sets the table name.
func (q *QueryBuilder) From(table string) *QueryBuilder {
	q.table = table
	return q
}

// Where adds a WHERE clause. Value is added as a parameter.
func (q *QueryBuilder) Where(column, op string, value interface{}) *QueryBuilder {
	q.wheres = append(q.wheres, whereClause{column: column, op: op})
	q.args = append(q.args, value)
	return q
}

// WhereNull adds WHERE column IS NULL.
func (q *QueryBuilder) WhereNull(column string) *QueryBuilder {
	q.wheres = append(q.wheres, whereClause{column: column, op: "IS NULL"})
	return q
}

// WhereNotNull adds WHERE column IS NOT NULL.
func (q *QueryBuilder) WhereNotNull(column string) *QueryBuilder {
	q.wheres = append(q.wheres, whereClause{column: column, op: "IS NOT NULL"})
	return q
}

// OrderBy sets ORDER BY.
func (q *QueryBuilder) OrderBy(column, direction string) *QueryBuilder {
	q.orderBy = column
	q.orderDir = direction
	return q
}

// Limit sets LIMIT.
func (q *QueryBuilder) Limit(n int) *QueryBuilder {
	q.limit = n
	return q
}

// Offset sets OFFSET.
func (q *QueryBuilder) Offset(n int) *QueryBuilder {
	q.offset = n
	return q
}

// InsertInto starts an INSERT query.
func InsertInto(table string, columns ...string) *QueryBuilder {
	return &QueryBuilder{
		operation: "INSERT",
		table:     table,
		columns:   columns,
	}
}

// Values adds a row of values for INSERT.
func (q *QueryBuilder) Values(values ...interface{}) *QueryBuilder {
	q.values = append(q.values, values...)
	return q
}

// Returning adds RETURNING clause.
func (q *QueryBuilder) Returning(columns ...string) *QueryBuilder {
	q.returning = columns
	return q
}

// Update starts an UPDATE query.
func Update(table string) *QueryBuilder {
	return &QueryBuilder{
		operation: "UPDATE",
		table:     table,
	}
}

// Set adds a SET clause for UPDATE.
func (q *QueryBuilder) Set(column string, value interface{}) *QueryBuilder {
	q.sets = append(q.sets, setClause{column: column})
	q.args = append(q.args, value)
	return q
}

// DeleteFrom starts a DELETE query.
func DeleteFrom(table string) *QueryBuilder {
	return &QueryBuilder{
		operation: "DELETE",
		table:     table,
	}
}

// Build generates the SQL string and parameter slice.
func (q *QueryBuilder) Build() (string, []interface{}) {
	switch q.operation {
	case "SELECT":
		return q.buildSelect()
	case "INSERT":
		return q.buildInsert()
	case "UPDATE":
		return q.buildUpdate()
	case "DELETE":
		return q.buildDelete()
	default:
		return "", nil
	}
}

// Count wraps the query in SELECT COUNT(*).
func (q *QueryBuilder) Count() (string, []interface{}) {
	// Save original columns, replace with COUNT(*)
	origCols := q.columns
	q.columns = []string{"COUNT(*)"}
	sql, args := q.buildSelect()
	q.columns = origCols
	return sql, args
}

func (q *QueryBuilder) buildSelect() (string, []interface{}) {
	var b strings.Builder

	cols := "*"
	if len(q.columns) > 0 {
		cols = strings.Join(q.columns, ", ")
	}
	fmt.Fprintf(&b, "SELECT %s FROM %s", cols, q.table)

	allArgs := make([]interface{}, len(q.args))
	copy(allArgs, q.args)

	q.appendWhere(&b, allArgs)

	if q.orderBy != "" {
		fmt.Fprintf(&b, " ORDER BY %s %s", q.orderBy, q.orderDir)
	}
	if q.limit > 0 {
		fmt.Fprintf(&b, " LIMIT %d", q.limit)
	}
	if q.offset > 0 {
		fmt.Fprintf(&b, " OFFSET %d", q.offset)
	}

	return b.String(), allArgs
}

func (q *QueryBuilder) buildInsert() (string, []interface{}) {
	var b strings.Builder

	fmt.Fprintf(&b, "INSERT INTO %s (%s) VALUES (", q.table, strings.Join(q.columns, ", "))

	params := make([]string, len(q.values))
	for i := range q.values {
		params[i] = fmt.Sprintf("$%d", i+1)
	}
	b.WriteString(strings.Join(params, ", "))
	b.WriteString(")")

	if len(q.returning) > 0 {
		fmt.Fprintf(&b, " RETURNING %s", strings.Join(q.returning, ", "))
	}

	allArgs := make([]interface{}, len(q.values))
	copy(allArgs, q.values)

	return b.String(), allArgs
}

func (q *QueryBuilder) buildUpdate() (string, []interface{}) {
	var b strings.Builder

	fmt.Fprintf(&b, "UPDATE %s SET ", q.table)

	setClauses := make([]string, len(q.sets))
	// SET args come first in q.args, then WHERE args
	// We need to number them sequentially
	argIndex := 1
	for i, s := range q.sets {
		setClauses[i] = fmt.Sprintf("%s = $%d", s.column, argIndex)
		argIndex++
	}
	b.WriteString(strings.Join(setClauses, ", "))

	allArgs := make([]interface{}, len(q.args))
	copy(allArgs, q.args)

	// WHERE clauses use args after the SET args
	if len(q.wheres) > 0 {
		b.WriteString(" WHERE ")
		whereParts := make([]string, 0, len(q.wheres))
		for _, w := range q.wheres {
			if w.op == "IS NULL" || w.op == "IS NOT NULL" {
				whereParts = append(whereParts, fmt.Sprintf("%s %s", w.column, w.op))
			} else {
				whereParts = append(whereParts, fmt.Sprintf("%s %s $%d", w.column, w.op, argIndex))
				argIndex++
			}
		}
		b.WriteString(strings.Join(whereParts, " AND "))
	}

	return b.String(), allArgs
}

func (q *QueryBuilder) buildDelete() (string, []interface{}) {
	var b strings.Builder

	fmt.Fprintf(&b, "DELETE FROM %s", q.table)

	allArgs := make([]interface{}, len(q.args))
	copy(allArgs, q.args)

	q.appendWhere(&b, allArgs)

	return b.String(), allArgs
}

func (q *QueryBuilder) appendWhere(b *strings.Builder, _ []interface{}) {
	if len(q.wheres) == 0 {
		return
	}

	b.WriteString(" WHERE ")
	argIndex := 1
	parts := make([]string, 0, len(q.wheres))
	for _, w := range q.wheres {
		if w.op == "IS NULL" || w.op == "IS NOT NULL" {
			parts = append(parts, fmt.Sprintf("%s %s", w.column, w.op))
		} else {
			parts = append(parts, fmt.Sprintf("%s %s $%d", w.column, w.op, argIndex))
			argIndex++
		}
	}
	b.WriteString(strings.Join(parts, " AND "))
}
