package jsonpatch

import (
	"encoding/json"
	"strings"
	"testing"
)

// raw is a small helper for inline json.RawMessage values.
func raw(v string) json.RawMessage { return json.RawMessage(v) }

// TestApply_RecursiveTraversal pins the recursive descent in
// getValue/addValue/removeValue through nested map/array layers — these
// were the 71-72% lines that the existing top-level tests skipped.
func TestApply_RecursiveTraversal(t *testing.T) {
	// nested replace through array
	doc := []byte(`{"a":{"b":[{"c":"old"}]}}`)
	out, err := Patch{{Op: OpReplace, Path: "/a/b/0/c", Value: raw(`"new"`)}}.Apply(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"new"`) {
		t.Errorf("nested replace lost: %s", out)
	}

	// nested add into object
	out2, err := Patch{{Op: OpAdd, Path: "/a/b/x", Value: raw(`1`)}}.Apply([]byte(`{"a":{"b":{}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out2), `"x":1`) {
		t.Errorf("nested add: %s", out2)
	}

	// nested remove
	out3, err := Patch{{Op: OpRemove, Path: "/a/b"}}.Apply([]byte(`{"a":{"b":1,"c":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out3), `"b"`) {
		t.Errorf("nested remove kept key: %s", out3)
	}
}

// TestApply_TraversalErrors pins each error branch inside the recursive
// helpers: missing key, out-of-range, invalid index, scalar traversal,
// remove missing, add invalid index.
func TestApply_TraversalErrors(t *testing.T) {
	cases := []struct {
		name  string
		doc   string
		patch Patch
		want  string
	}{
		{"missing key replace", `{"a":1}`, Patch{{Op: OpReplace, Path: "/missing", Value: raw(`2`)}}, "not found"},
		{"out of range replace", `{"a":[1,2]}`, Patch{{Op: OpReplace, Path: "/a/9", Value: raw(`3`)}}, "out of range"},
		{"invalid array idx replace", `{"a":[1]}`, Patch{{Op: OpReplace, Path: "/a/foo", Value: raw(`3`)}}, "invalid array index"},
		{"traverse scalar", `{"a":1}`, Patch{{Op: OpReplace, Path: "/a/b", Value: raw(`3`)}}, "cannot remove"},
		{"remove missing key", `{"a":1}`, Patch{{Op: OpRemove, Path: "/missing"}}, "not found"},
		{"remove nested missing", `{"a":{"b":1}}`, Patch{{Op: OpRemove, Path: "/a/missing"}}, "not found"},
		{"remove invalid array idx", `{"a":[1]}`, Patch{{Op: OpRemove, Path: "/a/foo"}}, "invalid array index"},
		{"remove array out of range", `{"a":[1]}`, Patch{{Op: OpRemove, Path: "/a/9"}}, "out of range"},
		{"add nested missing key", `{"a":{}}`, Patch{{Op: OpAdd, Path: "/a/missing/x", Value: raw(`1`)}}, "not found"},
		{"add into scalar", `{"a":1}`, Patch{{Op: OpAdd, Path: "/a/b/c", Value: raw(`1`)}}, "cannot"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.patch.Apply([]byte(tc.doc))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want substring %q", err, tc.want)
			}
		})
	}
}
