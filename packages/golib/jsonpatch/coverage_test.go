package jsonpatch

import (
	"encoding/json"
	"strings"
	"testing"
)

// applyJSON is a tiny helper that round-trips a doc through Apply and
// returns the result as a typed value for assertion.
func applyJSON(t *testing.T, doc string, p Patch) (interface{}, error) {
	t.Helper()
	out, err := p.Apply([]byte(doc))
	if err != nil {
		return nil, err
	}
	var v interface{}
	if jerr := json.Unmarshal(out, &v); jerr != nil {
		t.Fatalf("post-apply unmarshal: %v (out=%s)", jerr, out)
	}
	return v, nil
}

// TestValidate_RejectsMissingFields enumerates the validation error
// branches that were uncovered (op-specific required fields).
func TestValidate_RejectsMissingFields(t *testing.T) {
	cases := []struct {
		name string
		op   Operation
		want string
	}{
		{"add without path", Operation{Op: OpAdd, Value: json.RawMessage(`1`)}, "add requires path"},
		{"add without value", Operation{Op: OpAdd, Path: "/x"}, "add requires value"},
		{"replace without value", Operation{Op: OpReplace, Path: "/x"}, "replace requires value"},
		{"test without value", Operation{Op: OpTest, Path: "/x"}, "test requires value"},
		{"remove without path", Operation{Op: OpRemove}, "remove requires path"},
		{"move without from", Operation{Op: OpMove, Path: "/x"}, "move requires from"},
		{"move without path", Operation{Op: OpMove, From: "/x"}, "move requires path"},
		{"copy without from", Operation{Op: OpCopy, Path: "/x"}, "copy requires from"},
		{"unknown op", Operation{Op: "frobnicate", Path: "/x"}, `unknown op: "frobnicate"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Patch{tc.op}.Validate()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

// TestApply_ArrayOps covers add (insert + append), remove, and replace
// against arrays — the previously-uncovered side of addValue/removeValue.
func TestApply_ArrayOps(t *testing.T) {
	doc := `{"items":[1,2,3]}`

	// Append via "-" sentinel.
	got, err := applyJSON(t, doc, Patch{AddOp("/items/-", 4)})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	items := got.(map[string]interface{})["items"].([]interface{})
	if len(items) != 4 || items[3].(float64) != 4 {
		t.Errorf("append result: %v", items)
	}

	// Insert at index 1.
	got, err = applyJSON(t, doc, Patch{AddOp("/items/1", 99)})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	items = got.(map[string]interface{})["items"].([]interface{})
	if len(items) != 4 || items[1].(float64) != 99 || items[2].(float64) != 2 {
		t.Errorf("insert result: %v", items)
	}

	// Remove index 0.
	got, err = applyJSON(t, doc, Patch{RemoveOp("/items/0")})
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	items = got.(map[string]interface{})["items"].([]interface{})
	if len(items) != 2 || items[0].(float64) != 2 {
		t.Errorf("remove result: %v", items)
	}
}

// TestApply_MoveCopyTest covers Move/Copy/Test paths in applyOp.
func TestApply_MoveCopyTest(t *testing.T) {
	doc := `{"a":{"x":1},"b":{}}`

	// Move /a/x → /b/y
	got, err := applyJSON(t, doc, Patch{{Op: OpMove, From: "/a/x", Path: "/b/y"}})
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	root := got.(map[string]interface{})
	if _, present := root["a"].(map[string]interface{})["x"]; present {
		t.Error("move did not delete source")
	}
	if root["b"].(map[string]interface{})["y"].(float64) != 1 {
		t.Errorf("move did not place value at dest: %v", root)
	}

	// Copy /a → /c
	got, err = applyJSON(t, doc, Patch{{Op: OpCopy, From: "/a", Path: "/c"}})
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	root = got.(map[string]interface{})
	if root["c"] == nil {
		t.Error("copy did not create dest")
	}

	// Test passes when value matches.
	if _, err := applyJSON(t, doc, Patch{TestOp("/a/x", 1)}); err != nil {
		t.Errorf("test should have passed: %v", err)
	}
	// Test fails when value differs.
	_, err = applyJSON(t, doc, Patch{TestOp("/a/x", 999)})
	if err == nil || !strings.Contains(err.Error(), "test failed") {
		t.Errorf("test should have failed, got %v", err)
	}
}

// TestApply_PathErrors covers the error branches in getValue/addValue/
// removeValue: missing key, invalid index, out-of-range, traverse non-container.
func TestApply_PathErrors(t *testing.T) {
	cases := []struct {
		name  string
		doc   string
		patch Patch
		want  string
	}{
		{"remove missing key", `{}`, Patch{RemoveOp("/missing")}, "not found"},
		{"remove from scalar", `{"a":1}`, Patch{RemoveOp("/a/sub")}, "cannot remove from"},
		{"remove root", `{}`, Patch{RemoveOp("/")}, "cannot remove root"},
		{"add invalid array index", `{"a":[1,2]}`, Patch{AddOp("/a/notnum", 9)}, "invalid array index"},
		{"add out of range insert", `{"a":[1,2]}`, Patch{AddOp("/a/99", 9)}, "out of range"},
		{"copy missing source", `{}`, Patch{{Op: OpCopy, From: "/nope", Path: "/x"}}, "not found"},
		{"test missing path", `{}`, Patch{TestOp("/nope", 1)}, "not found"},
		{"get array bad index", `{"a":[1]}`, Patch{TestOp("/a/x", 1)}, "invalid array index"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.patch.Apply([]byte(tc.doc))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

// TestApply_RFC6901Escapes covers parsePath unescaping of ~1 (slash) and
// ~0 (tilde) per RFC 6901.
func TestApply_RFC6901Escapes(t *testing.T) {
	doc := `{"a/b":1,"c~d":2}`
	// /a~1b → key "a/b"
	got, err := applyJSON(t, doc, Patch{TestOp("/a~1b", 1)})
	if err != nil {
		t.Errorf("a/b escape: %v", err)
	}
	_ = got
	// /c~0d → key "c~d"
	if _, err := applyJSON(t, doc, Patch{TestOp("/c~0d", 2)}); err != nil {
		t.Errorf("c~d escape: %v", err)
	}
}
