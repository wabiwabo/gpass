package seeddata

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type failingInserter struct{ n int }

func (f *failingInserter) Insert(_ context.Context, _ string, _ map[string]interface{}) error {
	f.n++
	return errors.New("boom")
}

// TestLoadDir_MissingDirectory pins the os.ReadDir error branch.
func TestLoadDir_MissingDirectory(t *testing.T) {
	l := NewLoader(NewMemoryInserter(), nil)
	_, err := l.LoadDir(context.Background(), "/nonexistent/path/xyz")
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

// TestLoadFile_ReadError pins the os.ReadFile error branch.
func TestLoadFile_ReadError(t *testing.T) {
	l := NewLoader(NewMemoryInserter(), nil)
	_, err := l.LoadFile(context.Background(), "/nonexistent/file.json")
	if err == nil {
		t.Fatal("expected read error")
	}
}

// TestLoadFile_BadJSON pins the json.Unmarshal error branch.
func TestLoadFile_BadJSON(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := NewLoader(NewMemoryInserter(), nil)
	_, err := l.LoadFile(context.Background(), bad)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

// TestLoadDir_HappyPathAndSubdirsAndNonJSON pins:
//   - subdirectories are skipped (entry.IsDir branch)
//   - non-.json files are skipped
//   - JSON files load in sorted order
//   - per-file errors accumulate without aborting the run
//   - inserter errors increment FileResult.Errors but LoadFile still succeeds
func TestLoadDir_HappyPathAndSubdirsAndNonJSON(t *testing.T) {
	dir := t.TempDir()

	// Subdir — must be skipped.
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Non-JSON file — must be skipped.
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Valid seed file (sorted second).
	good := `[{"table":"users","data":{"id":1}},{"table":"users","data":{"id":2}}]`
	if err := os.WriteFile(filepath.Join(dir, "002_users.json"), []byte(good), 0o644); err != nil {
		t.Fatal(err)
	}
	// Malformed file (sorted first) — must be recorded in result.Errors.
	if err := os.WriteFile(filepath.Join(dir, "001_bad.json"), []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}

	mem := NewMemoryInserter()
	l := NewLoader(mem, nil)
	res, err := l.LoadDir(context.Background(), dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if res.TotalRecords != 2 {
		t.Errorf("TotalRecords = %d, want 2", res.TotalRecords)
	}
	if len(res.Errors) != 1 {
		t.Errorf("Errors = %v, want exactly one (the bad file)", res.Errors)
	}
	if mem.Count() != 2 {
		t.Errorf("MemoryInserter.Count = %d, want 2", mem.Count())
	}
}

// TestLoadFile_InserterErrorsCounted pins the result.Errors++ branch when
// the inserter rejects records — LoadFile must still return nil error.
func TestLoadFile_InserterErrorsCounted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.json")
	body := `[{"table":"t","data":{"k":"v"}},{"table":"t","data":{"k":"w"}}]`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	l := NewLoader(&failingInserter{}, nil)
	fr, err := l.LoadFile(context.Background(), path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if fr.Errors != 2 || fr.Inserted != 0 {
		t.Errorf("Errors=%d Inserted=%d, want 2/0", fr.Errors, fr.Inserted)
	}
}
