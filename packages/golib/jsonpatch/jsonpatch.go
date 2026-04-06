// Package jsonpatch implements RFC 6902 JSON Patch operations for
// partial resource updates. Supports add, remove, replace, move,
// copy, and test operations.
package jsonpatch

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Op represents a JSON Patch operation type.
type Op string

const (
	OpAdd     Op = "add"
	OpRemove  Op = "remove"
	OpReplace Op = "replace"
	OpMove    Op = "move"
	OpCopy    Op = "copy"
	OpTest    Op = "test"
)

// Operation is a single JSON Patch operation.
type Operation struct {
	Op    Op               `json:"op"`
	Path  string           `json:"path"`
	Value json.RawMessage  `json:"value,omitempty"`
	From  string           `json:"from,omitempty"`
}

// Patch is a list of JSON Patch operations.
type Patch []Operation

// Parse parses a JSON Patch document.
func Parse(data []byte) (Patch, error) {
	var patch Patch
	if err := json.Unmarshal(data, &patch); err != nil {
		return nil, fmt.Errorf("jsonpatch: parse: %w", err)
	}
	return patch, nil
}

// Validate checks that all operations are well-formed.
func (p Patch) Validate() error {
	for i, op := range p {
		if err := op.validate(); err != nil {
			return fmt.Errorf("jsonpatch: operation %d: %w", i, err)
		}
	}
	return nil
}

func (op *Operation) validate() error {
	switch op.Op {
	case OpAdd, OpReplace, OpTest:
		if op.Path == "" {
			return fmt.Errorf("%s requires path", op.Op)
		}
		if op.Value == nil {
			return fmt.Errorf("%s requires value", op.Op)
		}
	case OpRemove:
		if op.Path == "" {
			return fmt.Errorf("remove requires path")
		}
	case OpMove, OpCopy:
		if op.Path == "" {
			return fmt.Errorf("%s requires path", op.Op)
		}
		if op.From == "" {
			return fmt.Errorf("%s requires from", op.Op)
		}
	default:
		return fmt.Errorf("unknown op: %q", op.Op)
	}
	return nil
}

// Apply applies the patch to a JSON document.
func (p Patch) Apply(doc []byte) ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	var target interface{}
	if err := json.Unmarshal(doc, &target); err != nil {
		return nil, fmt.Errorf("jsonpatch: unmarshal: %w", err)
	}

	var err error
	for i, op := range p {
		target, err = applyOp(target, op)
		if err != nil {
			return nil, fmt.Errorf("jsonpatch: op %d (%s %s): %w", i, op.Op, op.Path, err)
		}
	}

	return json.Marshal(target)
}

func applyOp(doc interface{}, op Operation) (interface{}, error) {
	switch op.Op {
	case OpAdd:
		var value interface{}
		json.Unmarshal(op.Value, &value)
		return addValue(doc, parsePath(op.Path), value)
	case OpRemove:
		return removeValue(doc, parsePath(op.Path))
	case OpReplace:
		var value interface{}
		json.Unmarshal(op.Value, &value)
		doc, err := removeValue(doc, parsePath(op.Path))
		if err != nil {
			return nil, err
		}
		return addValue(doc, parsePath(op.Path), value)
	case OpMove:
		val, err := getValue(doc, parsePath(op.From))
		if err != nil {
			return nil, err
		}
		doc, err = removeValue(doc, parsePath(op.From))
		if err != nil {
			return nil, err
		}
		return addValue(doc, parsePath(op.Path), val)
	case OpCopy:
		val, err := getValue(doc, parsePath(op.From))
		if err != nil {
			return nil, err
		}
		return addValue(doc, parsePath(op.Path), val)
	case OpTest:
		var expected interface{}
		json.Unmarshal(op.Value, &expected)
		actual, err := getValue(doc, parsePath(op.Path))
		if err != nil {
			return nil, err
		}
		expectedJSON, _ := json.Marshal(expected)
		actualJSON, _ := json.Marshal(actual)
		if string(expectedJSON) != string(actualJSON) {
			return nil, fmt.Errorf("test failed: expected %s, got %s", expectedJSON, actualJSON)
		}
		return doc, nil
	}
	return nil, fmt.Errorf("unknown op: %s", op.Op)
}

func parsePath(path string) []string {
	if path == "" || path == "/" {
		return nil
	}
	// Remove leading slash.
	if path[0] == '/' {
		path = path[1:]
	}
	parts := strings.Split(path, "/")
	// Unescape RFC 6901 references.
	for i, p := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(p, "~1", "/"), "~0", "~")
	}
	return parts
}

func getValue(doc interface{}, path []string) (interface{}, error) {
	if len(path) == 0 {
		return doc, nil
	}

	switch target := doc.(type) {
	case map[string]interface{}:
		val, ok := target[path[0]]
		if !ok {
			return nil, fmt.Errorf("key %q not found", path[0])
		}
		return getValue(val, path[1:])
	case []interface{}:
		idx, err := strconv.Atoi(path[0])
		if err != nil {
			return nil, fmt.Errorf("invalid array index: %s", path[0])
		}
		if idx < 0 || idx >= len(target) {
			return nil, fmt.Errorf("index %d out of range", idx)
		}
		return getValue(target[idx], path[1:])
	default:
		return nil, fmt.Errorf("cannot traverse %T", doc)
	}
}

func addValue(doc interface{}, path []string, value interface{}) (interface{}, error) {
	if len(path) == 0 {
		return value, nil
	}

	switch target := doc.(type) {
	case map[string]interface{}:
		if len(path) == 1 {
			target[path[0]] = value
			return target, nil
		}
		child, ok := target[path[0]]
		if !ok {
			return nil, fmt.Errorf("key %q not found", path[0])
		}
		newChild, err := addValue(child, path[1:], value)
		if err != nil {
			return nil, err
		}
		target[path[0]] = newChild
		return target, nil
	case []interface{}:
		idx, err := strconv.Atoi(path[0])
		if err != nil {
			if path[0] == "-" {
				// Append.
				return append(target, value), nil
			}
			return nil, fmt.Errorf("invalid array index: %s", path[0])
		}
		if len(path) == 1 {
			if idx > len(target) {
				return nil, fmt.Errorf("index %d out of range", idx)
			}
			// Insert at index.
			result := make([]interface{}, len(target)+1)
			copy(result, target[:idx])
			result[idx] = value
			copy(result[idx+1:], target[idx:])
			return result, nil
		}
		if idx < 0 || idx >= len(target) {
			return nil, fmt.Errorf("index %d out of range", idx)
		}
		newChild, err := addValue(target[idx], path[1:], value)
		if err != nil {
			return nil, err
		}
		target[idx] = newChild
		return target, nil
	default:
		return nil, fmt.Errorf("cannot add to %T", doc)
	}
}

func removeValue(doc interface{}, path []string) (interface{}, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("cannot remove root")
	}

	switch target := doc.(type) {
	case map[string]interface{}:
		if len(path) == 1 {
			if _, ok := target[path[0]]; !ok {
				return nil, fmt.Errorf("key %q not found", path[0])
			}
			delete(target, path[0])
			return target, nil
		}
		child, ok := target[path[0]]
		if !ok {
			return nil, fmt.Errorf("key %q not found", path[0])
		}
		newChild, err := removeValue(child, path[1:])
		if err != nil {
			return nil, err
		}
		target[path[0]] = newChild
		return target, nil
	case []interface{}:
		idx, err := strconv.Atoi(path[0])
		if err != nil {
			return nil, fmt.Errorf("invalid array index: %s", path[0])
		}
		if idx < 0 || idx >= len(target) {
			return nil, fmt.Errorf("index %d out of range", idx)
		}
		if len(path) == 1 {
			return append(target[:idx], target[idx+1:]...), nil
		}
		newChild, err := removeValue(target[idx], path[1:])
		if err != nil {
			return nil, err
		}
		target[idx] = newChild
		return target, nil
	default:
		return nil, fmt.Errorf("cannot remove from %T", doc)
	}
}

// NewPatch creates a patch from individual operations.
func NewPatch(ops ...Operation) Patch {
	return Patch(ops)
}

// AddOp creates an add operation.
func AddOp(path string, value interface{}) Operation {
	v, _ := json.Marshal(value)
	return Operation{Op: OpAdd, Path: path, Value: v}
}

// RemoveOp creates a remove operation.
func RemoveOp(path string) Operation {
	return Operation{Op: OpRemove, Path: path}
}

// ReplaceOp creates a replace operation.
func ReplaceOp(path string, value interface{}) Operation {
	v, _ := json.Marshal(value)
	return Operation{Op: OpReplace, Path: path, Value: v}
}

// TestOp creates a test operation.
func TestOp(path string, value interface{}) Operation {
	v, _ := json.Marshal(value)
	return Operation{Op: OpTest, Path: path, Value: v}
}
