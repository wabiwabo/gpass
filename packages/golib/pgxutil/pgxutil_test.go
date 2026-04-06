package pgxutil

import (
	"strings"
	"testing"
	"time"
)

func TestPlaceholder(t *testing.T) {
	if Placeholder(1) != "$1" { t.Error("$1") }
	if Placeholder(10) != "$10" { t.Error("$10") }
}

func TestPlaceholders(t *testing.T) {
	if Placeholders(1, 3) != "$1, $2, $3" { t.Errorf("got %q", Placeholders(1, 3)) }
	if Placeholders(5, 2) != "$5, $6" { t.Errorf("got %q", Placeholders(5, 2)) }
}

func TestNullString(t *testing.T) {
	p := NullString("hello")
	if p == nil || *p != "hello" { t.Error("non-empty") }
	if NullString("") != nil { t.Error("empty should be nil") }
}

func TestNullInt(t *testing.T) {
	p := NullInt(42)
	if p == nil || *p != 42 { t.Error("non-zero") }
	if NullInt(0) != nil { t.Error("zero should be nil") }
}

func TestNullTime(t *testing.T) {
	now := time.Now()
	p := NullTime(now)
	if p == nil { t.Error("non-zero") }
	if NullTime(time.Time{}) != nil { t.Error("zero should be nil") }
}

func TestDerefString(t *testing.T) {
	s := "hello"
	if DerefString(&s) != "hello" { t.Error("deref") }
	if DerefString(nil) != "" { t.Error("nil") }
}

func TestDerefInt(t *testing.T) {
	n := 42
	if DerefInt(&n) != 42 { t.Error("deref") }
	if DerefInt(nil) != 0 { t.Error("nil") }
}

func TestDerefTime(t *testing.T) {
	now := time.Now()
	if DerefTime(&now) != now { t.Error("deref") }
	if !DerefTime(nil).IsZero() { t.Error("nil") }
}

func TestOrderByClause(t *testing.T) {
	allowed := []string{"name", "created_at", "email"}

	got := OrderByClause("name", "asc", allowed)
	if got != "ORDER BY name ASC" { t.Errorf("got %q", got) }

	got = OrderByClause("created_at", "desc", allowed)
	if got != "ORDER BY created_at DESC" { t.Errorf("got %q", got) }

	// Injection attempt
	got = OrderByClause("name; DROP TABLE users", "asc", allowed)
	if got != "" { t.Errorf("injection should return empty: %q", got) }
}

func TestLimitOffset(t *testing.T) {
	if LimitOffset(10, 20) != "LIMIT 10 OFFSET 20" { t.Error("normal") }
	if LimitOffset(0, 0) != "LIMIT 20 OFFSET 0" { t.Error("defaults") }
	if LimitOffset(5000, 0) != "LIMIT 1000 OFFSET 0" { t.Error("cap") }
	if LimitOffset(10, -5) != "LIMIT 10 OFFSET 0" { t.Error("negative offset") }
}

func TestILike(t *testing.T) {
	got := ILike("john")
	if got != "%john%" { t.Errorf("got %q", got) }

	// Escape wildcards
	got = ILike("50%_off")
	if !strings.Contains(got, "\\%") { t.Error("should escape %") }
	if !strings.Contains(got, "\\_") { t.Error("should escape _") }
}
