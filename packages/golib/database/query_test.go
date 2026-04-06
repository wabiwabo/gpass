package database

import (
	"regexp"
	"testing"
)

func TestSelectWithWhere(t *testing.T) {
	uid := "550e8400-e29b-41d4-a716-446655440000"
	sql, args := Select("name", "email").From("users").Where("id", "=", uid).Build()

	expected := "SELECT name, email FROM users WHERE id = $1"
	if sql != expected {
		t.Errorf("got SQL %q, want %q", sql, expected)
	}
	if len(args) != 1 || args[0] != uid {
		t.Errorf("got args %v, want [%s]", args, uid)
	}
}

func TestSelectWithMultipleWheres(t *testing.T) {
	sql, args := Select("*").From("users").
		Where("status", "=", "active").
		Where("age", ">=", 18).
		Build()

	expected := "SELECT * FROM users WHERE status = $1 AND age >= $2"
	if sql != expected {
		t.Errorf("got SQL %q, want %q", sql, expected)
	}
	if len(args) != 2 {
		t.Fatalf("got %d args, want 2", len(args))
	}
	if args[0] != "active" {
		t.Errorf("args[0] = %v, want %q", args[0], "active")
	}
	if args[1] != 18 {
		t.Errorf("args[1] = %v, want 18", args[1])
	}
}

func TestSelectWithOrderByLimitOffset(t *testing.T) {
	sql, args := Select("name").From("users").
		OrderBy("created_at", "DESC").
		Limit(10).
		Offset(20).
		Build()

	expected := "SELECT name FROM users ORDER BY created_at DESC LIMIT 10 OFFSET 20"
	if sql != expected {
		t.Errorf("got SQL %q, want %q", sql, expected)
	}
	if len(args) != 0 {
		t.Errorf("got %d args, want 0", len(args))
	}
}

func TestSelectWithWhereNull(t *testing.T) {
	sql, args := Select("name").From("users").WhereNull("deleted_at").Build()

	expected := "SELECT name FROM users WHERE deleted_at IS NULL"
	if sql != expected {
		t.Errorf("got SQL %q, want %q", sql, expected)
	}
	if len(args) != 0 {
		t.Errorf("got %d args, want 0", len(args))
	}
}

func TestSelectWithWhereNotNull(t *testing.T) {
	sql, args := Select("name").From("users").WhereNotNull("email").Build()

	expected := "SELECT name FROM users WHERE email IS NOT NULL"
	if sql != expected {
		t.Errorf("got SQL %q, want %q", sql, expected)
	}
	if len(args) != 0 {
		t.Errorf("got %d args, want 0", len(args))
	}
}

func TestInsertIntoWithValues(t *testing.T) {
	sql, args := InsertInto("users", "name", "email").
		Values("John", "john@example.com").
		Build()

	expected := "INSERT INTO users (name, email) VALUES ($1, $2)"
	if sql != expected {
		t.Errorf("got SQL %q, want %q", sql, expected)
	}
	if len(args) != 2 {
		t.Fatalf("got %d args, want 2", len(args))
	}
	if args[0] != "John" {
		t.Errorf("args[0] = %v, want %q", args[0], "John")
	}
	if args[1] != "john@example.com" {
		t.Errorf("args[1] = %v, want %q", args[1], "john@example.com")
	}
}

func TestInsertIntoWithReturning(t *testing.T) {
	sql, args := InsertInto("users", "name", "email").
		Values("John", "john@example.com").
		Returning("id", "created_at").
		Build()

	expected := "INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id, created_at"
	if sql != expected {
		t.Errorf("got SQL %q, want %q", sql, expected)
	}
	if len(args) != 2 {
		t.Errorf("got %d args, want 2", len(args))
	}
}

func TestUpdateWithSetAndWhere(t *testing.T) {
	uid := "abc-123"
	sql, args := Update("users").
		Set("name", "Jane").
		Where("id", "=", uid).
		Build()

	expected := "UPDATE users SET name = $1 WHERE id = $2"
	if sql != expected {
		t.Errorf("got SQL %q, want %q", sql, expected)
	}
	if len(args) != 2 {
		t.Fatalf("got %d args, want 2", len(args))
	}
	if args[0] != "Jane" {
		t.Errorf("args[0] = %v, want %q", args[0], "Jane")
	}
	if args[1] != uid {
		t.Errorf("args[1] = %v, want %q", args[1], uid)
	}
}

func TestDeleteFromWithWhere(t *testing.T) {
	sql, args := DeleteFrom("sessions").
		Where("expired_at", "<", "2024-01-01").
		Build()

	expected := "DELETE FROM sessions WHERE expired_at < $1"
	if sql != expected {
		t.Errorf("got SQL %q, want %q", sql, expected)
	}
	if len(args) != 1 {
		t.Fatalf("got %d args, want 1", len(args))
	}
	if args[0] != "2024-01-01" {
		t.Errorf("args[0] = %v, want %q", args[0], "2024-01-01")
	}
}

func TestCount(t *testing.T) {
	sql, args := Select("name", "email").From("users").
		Where("status", "=", "active").
		Count()

	expected := "SELECT COUNT(*) FROM users WHERE status = $1"
	if sql != expected {
		t.Errorf("got SQL %q, want %q", sql, expected)
	}
	if len(args) != 1 || args[0] != "active" {
		t.Errorf("got args %v, want [active]", args)
	}
}

func TestEmptyWhereList(t *testing.T) {
	sql, _ := Select("*").From("users").Build()

	expected := "SELECT * FROM users"
	if sql != expected {
		t.Errorf("got SQL %q, want %q", sql, expected)
	}
}

func TestParametersNeverInterpolated(t *testing.T) {
	dangerousValue := "Robert'; DROP TABLE users;--"
	sql, args := Select("name").From("users").
		Where("name", "=", dangerousValue).
		Build()

	// The dangerous value must NOT appear in the SQL string
	if regexp.MustCompile(regexp.QuoteMeta(dangerousValue)).MatchString(sql) {
		t.Error("dangerous value was interpolated into SQL string")
	}

	// It must only be in the args
	if len(args) != 1 || args[0] != dangerousValue {
		t.Errorf("args should contain the dangerous value, got %v", args)
	}

	// SQL should use $N placeholders
	if !regexp.MustCompile(`\$\d+`).MatchString(sql) {
		t.Error("SQL does not use $N parameter placeholders")
	}
}
