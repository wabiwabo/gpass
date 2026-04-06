package seeddata

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Record represents a single seed data record.
type Record struct {
	Table string                 `json:"table"`
	Data  map[string]interface{} `json:"data"`
}

// SeedFile represents a seed data file with ordering.
type SeedFile struct {
	Order   int      `json:"order"`
	Table   string   `json:"table"`
	Records []Record `json:"records"`
}

// Inserter inserts records into the target store.
type Inserter interface {
	Insert(ctx context.Context, table string, data map[string]interface{}) error
}

// Loader loads and applies seed data from JSON files.
type Loader struct {
	inserter Inserter
	logger   *slog.Logger
}

// NewLoader creates a new seed data loader.
func NewLoader(inserter Inserter, logger *slog.Logger) *Loader {
	if logger == nil {
		logger = slog.Default()
	}
	return &Loader{inserter: inserter, logger: logger}
}

// LoadDir loads all JSON seed files from a directory in order.
// Files are sorted by name (use NNN_ prefix for ordering).
func (l *Loader) LoadDir(ctx context.Context, dir string) (*LoadResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("seeddata: read directory %s: %w", dir, err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".json") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	sort.Strings(files)

	result := &LoadResult{StartedAt: time.Now()}

	for _, file := range files {
		fr, err := l.LoadFile(ctx, file)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", filepath.Base(file), err))
			continue
		}
		result.Files = append(result.Files, *fr)
		result.TotalRecords += fr.Inserted
	}

	result.Duration = time.Since(result.StartedAt)
	return result, nil
}

// LoadFile loads a single JSON seed file.
func (l *Loader) LoadFile(ctx context.Context, path string) (*FileResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var records []Record
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	result := &FileResult{
		Filename: filepath.Base(path),
	}

	for _, rec := range records {
		if err := l.inserter.Insert(ctx, rec.Table, rec.Data); err != nil {
			l.logger.Warn("seeddata: insert failed",
				"file", filepath.Base(path),
				"table", rec.Table,
				"error", err,
			)
			result.Errors++
			continue
		}
		result.Inserted++
	}

	l.logger.Info("seeddata: loaded file",
		"file", filepath.Base(path),
		"inserted", result.Inserted,
		"errors", result.Errors,
	)

	return result, nil
}

// LoadResult summarizes a seed data loading operation.
type LoadResult struct {
	StartedAt    time.Time    `json:"started_at"`
	Duration     time.Duration `json:"duration"`
	TotalRecords int          `json:"total_records"`
	Files        []FileResult `json:"files"`
	Errors       []string     `json:"errors,omitempty"`
}

// FileResult summarizes loading a single file.
type FileResult struct {
	Filename string `json:"filename"`
	Inserted int    `json:"inserted"`
	Errors   int    `json:"errors"`
}

// MemoryInserter is an in-memory inserter for testing.
type MemoryInserter struct {
	Records map[string][]map[string]interface{}
}

// NewMemoryInserter creates a new in-memory inserter.
func NewMemoryInserter() *MemoryInserter {
	return &MemoryInserter{
		Records: make(map[string][]map[string]interface{}),
	}
}

// Insert stores a record in the memory map.
func (m *MemoryInserter) Insert(_ context.Context, table string, data map[string]interface{}) error {
	m.Records[table] = append(m.Records[table], data)
	return nil
}

// Count returns total records across all tables.
func (m *MemoryInserter) Count() int {
	total := 0
	for _, records := range m.Records {
		total += len(records)
	}
	return total
}
