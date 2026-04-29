package query

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"awiki/internal/adapters"
)

// mockDuckDB is a simple mock for the DuckDB adapter.
type mockDuckDB struct {
	output string
	code   int
	err    error
}

func (m mockDuckDB) Run(_ context.Context, _, _ string) (string, int, error) {
	return m.output, m.code, m.err
}

var _ adapters.DuckDB = mockDuckDB{}

func TestRun_SimpleSelect(t *testing.T) {
	tmp := t.TempDir()
	runner := &Runner{
		RepoRoot:   tmp,
		ContentDir: filepath.Join(tmp, "content"),
		DataDir:    filepath.Join(tmp, "data"),
		DuckDB:     mockDuckDB{output: "name,age\nalice,30\n", code: 0},
		Today:      "2024-01-01",
	}
	os.MkdirAll(runner.DatasetDir(), 0755)

	var buf bytes.Buffer
	err := runner.Run(context.Background(), RunOptions{SQL: "SELECT 1"}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "| name |") {
		t.Errorf("expected table output, got: %q", out)
	}
}

func TestRun_WithOut(t *testing.T) {
	tmp := t.TempDir()
	runner := &Runner{
		RepoRoot:   tmp,
		ContentDir: filepath.Join(tmp, "content"),
		DataDir:    filepath.Join(tmp, "data"),
		DuckDB:     mockDuckDB{output: "col\nval\n", code: 0},
		Today:      "2024-01-01",
	}
	os.MkdirAll(runner.DatasetDir(), 0755)

	var buf bytes.Buffer
	err := runner.Run(context.Background(), RunOptions{SQL: "SELECT 1 AS col", Out: "my-out"}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "QUERY|MATERIALIZED") {
		t.Errorf("expected MATERIALIZED message, got: %q", out)
	}

	// Check the file was created.
	target := filepath.Join(runner.DatasetDir(), "my-out.md")
	if _, err := os.Stat(target); os.IsNotExist(err) {
		t.Errorf("expected dataset page to be created at %s", target)
	}
}

func TestRun_MissingSQL(t *testing.T) {
	tmp := t.TempDir()
	runner := &Runner{
		RepoRoot:   tmp,
		ContentDir: filepath.Join(tmp, "content"),
		DataDir:    filepath.Join(tmp, "data"),
		DuckDB:     mockDuckDB{},
		Today:      "2024-01-01",
	}
	var buf bytes.Buffer
	err := runner.Run(context.Background(), RunOptions{SQL: ""}, &buf)
	if err == nil {
		t.Error("expected error for missing SQL")
	}
}

func TestRun_DuckDBError(t *testing.T) {
	tmp := t.TempDir()
	runner := &Runner{
		RepoRoot:   tmp,
		ContentDir: filepath.Join(tmp, "content"),
		DataDir:    filepath.Join(tmp, "data"),
		DuckDB:     mockDuckDB{output: "error msg", code: 1},
		Today:      "2024-01-01",
	}
	var buf bytes.Buffer
	err := runner.Run(context.Background(), RunOptions{SQL: "SELECT broken"}, &buf)
	if err == nil {
		t.Error("expected error when DuckDB exits with non-zero code")
	}
}
