package jsonpatch

import (
	"encoding/json"
	"testing"
)

func TestPatch_Add(t *testing.T) {
	doc := `{"name":"John"}`
	patch := NewPatch(AddOp("/age", 30))

	result, err := patch.Apply([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	json.Unmarshal(result, &m)
	if m["age"] != float64(30) {
		t.Errorf("age: got %v", m["age"])
	}
	if m["name"] != "John" {
		t.Errorf("name should be preserved: got %v", m["name"])
	}
}

func TestPatch_Remove(t *testing.T) {
	doc := `{"name":"John","age":30}`
	patch := NewPatch(RemoveOp("/age"))

	result, err := patch.Apply([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	json.Unmarshal(result, &m)
	if _, ok := m["age"]; ok {
		t.Error("age should be removed")
	}
}

func TestPatch_Replace(t *testing.T) {
	doc := `{"name":"John","age":30}`
	patch := NewPatch(ReplaceOp("/name", "Jane"))

	result, err := patch.Apply([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	json.Unmarshal(result, &m)
	if m["name"] != "Jane" {
		t.Errorf("name: got %v", m["name"])
	}
}

func TestPatch_Test(t *testing.T) {
	doc := `{"name":"John"}`

	// Test should pass.
	patch := NewPatch(TestOp("/name", "John"))
	_, err := patch.Apply([]byte(doc))
	if err != nil {
		t.Errorf("test should pass: %v", err)
	}

	// Test should fail.
	patch = NewPatch(TestOp("/name", "Jane"))
	_, err = patch.Apply([]byte(doc))
	if err == nil {
		t.Error("test should fail for wrong value")
	}
}

func TestPatch_NestedAdd(t *testing.T) {
	doc := `{"user":{"name":"John"}}`
	patch := NewPatch(AddOp("/user/age", 30))

	result, err := patch.Apply([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	json.Unmarshal(result, &m)
	user := m["user"].(map[string]interface{})
	if user["age"] != float64(30) {
		t.Errorf("nested age: got %v", user["age"])
	}
}

func TestPatch_MultipleOps(t *testing.T) {
	doc := `{"name":"John","age":30}`
	patch := NewPatch(
		ReplaceOp("/name", "Jane"),
		ReplaceOp("/age", 25),
		AddOp("/city", "Jakarta"),
	)

	result, err := patch.Apply([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	json.Unmarshal(result, &m)
	if m["name"] != "Jane" {
		t.Errorf("name: got %v", m["name"])
	}
	if m["age"] != float64(25) {
		t.Errorf("age: got %v", m["age"])
	}
	if m["city"] != "Jakarta" {
		t.Errorf("city: got %v", m["city"])
	}
}

func TestPatch_ArrayAdd(t *testing.T) {
	doc := `{"items":["a","b","c"]}`
	patch := NewPatch(AddOp("/items/1", "x"))

	result, err := patch.Apply([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	json.Unmarshal(result, &m)
	items := m["items"].([]interface{})
	if len(items) != 4 {
		t.Errorf("should have 4 items: got %d", len(items))
	}
	if items[1] != "x" {
		t.Errorf("items[1]: got %v", items[1])
	}
}

func TestPatch_ArrayAppend(t *testing.T) {
	doc := `{"items":["a","b"]}`
	patch := NewPatch(AddOp("/items/-", "c"))

	result, err := patch.Apply([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	json.Unmarshal(result, &m)
	items := m["items"].([]interface{})
	if len(items) != 3 || items[2] != "c" {
		t.Errorf("append: got %v", items)
	}
}

func TestPatch_ArrayRemove(t *testing.T) {
	doc := `{"items":["a","b","c"]}`
	patch := NewPatch(RemoveOp("/items/1"))

	result, err := patch.Apply([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	json.Unmarshal(result, &m)
	items := m["items"].([]interface{})
	if len(items) != 2 {
		t.Errorf("should have 2 items: got %d", len(items))
	}
}

func TestParse(t *testing.T) {
	raw := `[{"op":"add","path":"/name","value":"John"},{"op":"remove","path":"/age"}]`
	patch, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(patch) != 2 {
		t.Errorf("ops: got %d", len(patch))
	}
}

func TestParse_Invalid(t *testing.T) {
	_, err := Parse([]byte("not json"))
	if err == nil {
		t.Error("should fail on invalid JSON")
	}
}

func TestValidate_MissingPath(t *testing.T) {
	patch := NewPatch(Operation{Op: OpAdd, Value: []byte(`"val"`)})
	if err := patch.Validate(); err == nil {
		t.Error("should fail without path")
	}
}

func TestValidate_MissingValue(t *testing.T) {
	patch := NewPatch(Operation{Op: OpAdd, Path: "/name"})
	if err := patch.Validate(); err == nil {
		t.Error("should fail without value for add")
	}
}

func TestValidate_UnknownOp(t *testing.T) {
	patch := NewPatch(Operation{Op: "invalid", Path: "/x"})
	if err := patch.Validate(); err == nil {
		t.Error("should fail on unknown op")
	}
}

func TestPatch_RemoveNonExistent(t *testing.T) {
	doc := `{"name":"John"}`
	patch := NewPatch(RemoveOp("/missing"))

	_, err := patch.Apply([]byte(doc))
	if err == nil {
		t.Error("should fail removing non-existent key")
	}
}

func TestPatch_InvalidDoc(t *testing.T) {
	patch := NewPatch(AddOp("/name", "John"))
	_, err := patch.Apply([]byte("not json"))
	if err == nil {
		t.Error("should fail on invalid document")
	}
}
