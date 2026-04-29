package chart

import (
	"path/filepath"

	"awiki/internal/adapters"
)

// Runner holds the dependencies and root directories for chart operations.
type Runner struct {
	RepoRoot   string
	ContentDir string
	AssetsDir  string
	VLConvert  adapters.VLConvert
	VendorVega adapters.VendorVega
}

// AssetsChartsDir returns the absolute path to the charts assets directory
// (<RepoRoot>/assets/charts).
func (r *Runner) AssetsChartsDir() string {
	if r.AssetsDir != "" {
		return r.AssetsDir
	}
	return filepath.Join(r.RepoRoot, "assets", "charts")
}
