// Package paginatedb provides SQL pagination helpers for database
// queries. It generates SQL clauses for offset-based and keyset
// pagination, and produces structured page metadata.
package paginatedb

import (
	"fmt"
	"strings"
)

// PageRequest represents a pagination request.
type PageRequest struct {
	Page    int    // 1-indexed page number.
	PerPage int    // Items per page.
	SortBy  string // Column to sort by.
	SortDir string // "ASC" or "DESC".
}

// Normalize ensures valid pagination parameters.
func (p *PageRequest) Normalize(maxPerPage int) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PerPage < 1 {
		p.PerPage = 20
	}
	if maxPerPage > 0 && p.PerPage > maxPerPage {
		p.PerPage = maxPerPage
	}
	p.SortDir = strings.ToUpper(p.SortDir)
	if p.SortDir != "ASC" && p.SortDir != "DESC" {
		p.SortDir = "ASC"
	}
}

// Offset returns the SQL offset for this page.
func (p *PageRequest) Offset() int {
	return (p.Page - 1) * p.PerPage
}

// SQLClause generates ORDER BY, LIMIT, OFFSET clause.
// sortColumn must be a safe column name (not user input).
func (p *PageRequest) SQLClause(sortColumn string) string {
	if sortColumn == "" {
		sortColumn = p.SortBy
	}
	if sortColumn == "" {
		return fmt.Sprintf("LIMIT %d OFFSET %d", p.PerPage, p.Offset())
	}
	return fmt.Sprintf("ORDER BY %s %s LIMIT %d OFFSET %d",
		sortColumn, p.SortDir, p.PerPage, p.Offset())
}

// PageResult holds pagination metadata for a result set.
type PageResult struct {
	Page       int  `json:"page"`
	PerPage    int  `json:"per_page"`
	Total      int  `json:"total"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// NewPageResult computes pagination metadata.
func NewPageResult(page, perPage, total int) PageResult {
	if perPage <= 0 {
		perPage = 20
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + perPage - 1) / perPage
	}

	return PageResult{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}

// KeysetRequest represents keyset (cursor) pagination.
type KeysetRequest struct {
	After   string // Cursor value to paginate after.
	Before  string // Cursor value to paginate before.
	Limit   int    // Max items to return.
	SortCol string // Column for ordering.
	SortDir string // "ASC" or "DESC".
}

// Normalize ensures valid keyset parameters.
func (k *KeysetRequest) Normalize(maxLimit int) {
	if k.Limit < 1 {
		k.Limit = 20
	}
	if maxLimit > 0 && k.Limit > maxLimit {
		k.Limit = maxLimit
	}
	k.SortDir = strings.ToUpper(k.SortDir)
	if k.SortDir != "ASC" && k.SortDir != "DESC" {
		k.SortDir = "ASC"
	}
}

// WhereClause generates a keyset pagination WHERE clause.
// Returns the clause and argument value, or empty string if no cursor.
func (k *KeysetRequest) WhereClause(column string) (string, string) {
	if column == "" {
		column = k.SortCol
	}
	if k.After != "" {
		op := ">"
		if k.SortDir == "DESC" {
			op = "<"
		}
		return fmt.Sprintf("%s %s $1", column, op), k.After
	}
	if k.Before != "" {
		op := "<"
		if k.SortDir == "DESC" {
			op = ">"
		}
		return fmt.Sprintf("%s %s $1", column, op), k.Before
	}
	return "", ""
}

// KeysetResult holds keyset pagination metadata.
type KeysetResult struct {
	HasMore     bool   `json:"has_more"`
	NextCursor  string `json:"next_cursor,omitempty"`
	PrevCursor  string `json:"prev_cursor,omitempty"`
	Count       int    `json:"count"`
}
