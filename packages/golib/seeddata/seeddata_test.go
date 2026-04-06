package seeddata

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTestSeedFile(t *testing.T, dir, name string, records []Record) {
	t.Helper()
	data, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLoader_LoadFile(t *testing.T) {
	dir := t.TempDir()
	records := []Record{
		{Table: "users", Data: map[string]interface{}{"id": "u1", "name": "Alice"}},
		{Table: "users", Data: map[string]interface{}{"id": "u2", "name": "Bob"}},
	}
	writeTestSeedFile(t, dir, "001_users.json", records)

	inserter := NewMemoryInserter()
	loader := NewLoader(inserter, nil)

	result, err := loader.LoadFile(context.Background(), filepath.Join(dir, "001_users.json"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Inserted != 2 {
		t.Errorf("inserted: got %d, want 2", result.Inserted)
	}
	if inserter.Count() != 2 {
		t.Errorf("total records: got %d, want 2", inserter.Count())
	}
}

func TestLoader_LoadDir(t *testing.T) {
	dir := t.TempDir()

	writeTestSeedFile(t, dir, "001_users.json", []Record{
		{Table: "users", Data: map[string]interface{}{"id": "u1"}},
	})
	writeTestSeedFile(t, dir, "002_consents.json", []Record{
		{Table: "consents", Data: map[string]interface{}{"id": "c1", "user_id": "u1"}},
		{Table: "consents", Data: map[string]interface{}{"id": "c2", "user_id": "u1"}},
	})

	inserter := NewMemoryInserter()
	loader := NewLoader(inserter, nil)

	result, err := loader.LoadDir(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalRecords != 3 {
		t.Errorf("total records: got %d, want 3", result.TotalRecords)
	}
	if len(result.Files) != 2 {
		t.Errorf("files: got %d, want 2", len(result.Files))
	}
	if len(inserter.Records["users"]) != 1 {
		t.Errorf("users: got %d", len(inserter.Records["users"]))
	}
	if len(inserter.Records["consents"]) != 2 {
		t.Errorf("consents: got %d", len(inserter.Records["consents"]))
	}
}

func TestLoader_LoadDir_Ordering(t *testing.T) {
	dir := t.TempDir()

	// Files should be loaded in alphabetical order.
	writeTestSeedFile(t, dir, "002_second.json", []Record{
		{Table: "second", Data: map[string]interface{}{"order": 2}},
	})
	writeTestSeedFile(t, dir, "001_first.json", []Record{
		{Table: "first", Data: map[string]interface{}{"order": 1}},
	})

	inserter := NewMemoryInserter()
	loader := NewLoader(inserter, nil)

	result, _ := loader.LoadDir(context.Background(), dir)
	if result.Files[0].Filename != "001_first.json" {
		t.Errorf("first file: got %q", result.Files[0].Filename)
	}
	if result.Files[1].Filename != "002_second.json" {
		t.Errorf("second file: got %q", result.Files[1].Filename)
	}
}

func TestLoader_LoadDir_SkipsNonJSON(t *testing.T) {
	dir := t.TempDir()

	writeTestSeedFile(t, dir, "001_data.json", []Record{
		{Table: "data", Data: map[string]interface{}{"id": "1"}},
	})
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("not json"), 0644)

	inserter := NewMemoryInserter()
	loader := NewLoader(inserter, nil)

	result, _ := loader.LoadDir(context.Background(), dir)
	if result.TotalRecords != 1 {
		t.Errorf("should only load JSON: got %d records", result.TotalRecords)
	}
}

func TestLoader_LoadFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.json"), []byte("not json"), 0644)

	loader := NewLoader(NewMemoryInserter(), nil)
	_, err := loader.LoadFile(context.Background(), filepath.Join(dir, "bad.json"))
	if err == nil {
		t.Error("should fail on invalid JSON")
	}
}

func TestLoader_LoadFile_FileNotFound(t *testing.T) {
	loader := NewLoader(NewMemoryInserter(), nil)
	_, err := loader.LoadFile(context.Background(), "/nonexistent/file.json")
	if err == nil {
		t.Error("should fail on missing file")
	}
}

func TestLoader_LoadDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	inserter := NewMemoryInserter()
	loader := NewLoader(inserter, nil)

	result, err := loader.LoadDir(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalRecords != 0 {
		t.Error("empty dir should have 0 records")
	}
}

func TestLoader_LoadDir_NonexistentDir(t *testing.T) {
	loader := NewLoader(NewMemoryInserter(), nil)
	_, err := loader.LoadDir(context.Background(), "/nonexistent/dir")
	if err == nil {
		t.Error("should fail on nonexistent directory")
	}
}

func TestMemoryInserter_MultipleTables(t *testing.T) {
	m := NewMemoryInserter()
	ctx := context.Background()

	m.Insert(ctx, "a", map[string]interface{}{"x": 1})
	m.Insert(ctx, "a", map[string]interface{}{"x": 2})
	m.Insert(ctx, "b", map[string]interface{}{"y": 3})

	if m.Count() != 3 {
		t.Errorf("count: got %d, want 3", m.Count())
	}
	if len(m.Records["a"]) != 2 {
		t.Errorf("table a: got %d", len(m.Records["a"]))
	}
}
