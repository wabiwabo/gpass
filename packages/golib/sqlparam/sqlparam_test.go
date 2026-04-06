package sqlparam

import "testing"

func TestBuilder(t *testing.T) {
	b := New("SELECT * FROM users WHERE ")
	b.WriteParam("name = ?", "John")
	b.Write(" AND ")
	b.WriteParam("age > ?", 25)

	if b.SQL() != "SELECT * FROM users WHERE name = $1 AND age > $2" {
		t.Errorf("SQL = %q", b.SQL())
	}
	if b.ParamCount() != 2 { t.Errorf("params = %d", b.ParamCount()) }
	if b.Params()[0] != "John" { t.Error("param 0") }
	if b.Params()[1] != 25 { t.Error("param 1") }
}

func TestParam(t *testing.T) {
	b := New("")
	p1 := b.Param("val1")
	p2 := b.Param("val2")
	if p1 != "$1" { t.Errorf("p1 = %q", p1) }
	if p2 != "$2" { t.Errorf("p2 = %q", p2) }
}

func TestWhereIn(t *testing.T) {
	sql, params := WhereIn("id", []interface{}{1, 2, 3})
	if sql != "id IN ($1, $2, $3)" { t.Errorf("SQL = %q", sql) }
	if len(params) != 3 { t.Errorf("params = %d", len(params)) }
}

func TestWhereIn_Empty(t *testing.T) {
	sql, params := WhereIn("id", nil)
	if sql != "FALSE" { t.Errorf("SQL = %q", sql) }
	if params != nil { t.Error("params should be nil") }
}

func TestInsertColumns(t *testing.T) {
	sql := InsertColumns("users", []string{"name", "email", "age"})
	want := "INSERT INTO users (name, email, age) VALUES ($1, $2, $3)"
	if sql != want { t.Errorf("SQL = %q", sql) }
}

func TestUpdateSet(t *testing.T) {
	sql := UpdateSet([]string{"name", "email"}, 1)
	if sql != "name = $1, email = $2" { t.Errorf("SQL = %q", sql) }
}

func TestUpdateSet_StartParam(t *testing.T) {
	sql := UpdateSet([]string{"status"}, 3)
	if sql != "status = $3" { t.Errorf("SQL = %q", sql) }
}

func TestBuilder_Complex(t *testing.T) {
	b := New("SELECT * FROM users WHERE ")
	b.WriteParam("name = ?", "John")
	b.Write(" AND ")
	b.WriteParam("email = ?", "john@example.com")
	b.Write(" AND ")
	b.WriteParam("tenant_id = ?", "t-1")

	expected := "SELECT * FROM users WHERE name = $1 AND email = $2 AND tenant_id = $3"
	if b.SQL() != expected { t.Errorf("SQL = %q", b.SQL()) }
	if b.ParamCount() != 3 { t.Errorf("params = %d", b.ParamCount()) }
}
