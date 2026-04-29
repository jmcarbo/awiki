package query

import (
	"path/filepath"

	"awiki/internal/adapters"
)

// Runner holds the runtime context for query operations.
type Runner struct {
	RepoRoot   string
	ContentDir string
	DataDir    string
	DuckDB     adapters.DuckDB
	Today      string
}

// QueryDir returns the canonical queries sub-directory under ContentDir.
func (r *Runner) QueryDir() string {
	return filepath.Join(r.ContentDir, "queries")
}

// DatasetDir returns the canonical datasets sub-directory under ContentDir.
func (r *Runner) DatasetDir() string {
	return filepath.Join(r.ContentDir, "datasets")
}
