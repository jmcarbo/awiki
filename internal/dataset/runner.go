package dataset

import "path/filepath"

// Runner holds the runtime context for dataset operations.
type Runner struct {
	RepoRoot   string
	ContentDir string
	DataDir    string
	Config     map[string]string
	Today      string
}

// DatasetDir returns the canonical datasets sub-directory under ContentDir.
func (r *Runner) DatasetDir() string {
	return filepath.Join(r.ContentDir, "datasets")
}
