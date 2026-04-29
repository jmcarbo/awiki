package synth

import (
	"path/filepath"

	"awiki/internal/adapters"
	"awiki/internal/config"
)

// Runner is the per-invocation context shared by every synth verb.
type Runner struct {
	RepoRoot   string
	ContentDir string
	PluginDir  string
	Config     map[string]string
	Qmd        adapters.Qmd
	PostHook   adapters.PostHook
	Today      string // injected for tests; defaults to time.Now date
}

// AllowPostHooks reports whether plugin post-hooks may run.
// Mirrors the bash check on .awiki/config: ALLOW_PLUGIN_POST_HOOKS=1.
func (r *Runner) AllowPostHooks() bool {
	if r == nil || r.Config == nil {
		return false
	}
	return r.Config["ALLOW_PLUGIN_POST_HOOKS"] == "1"
}

// LoadConfig reads .awiki/config under r.RepoRoot. Missing file is
// non-fatal (mirrors bash `source -f`).
func (r *Runner) LoadConfig() error {
	if r.RepoRoot == "" {
		return nil
	}
	cfg, err := config.Load(filepath.Join(r.RepoRoot, ".awiki", "config"))
	if err != nil {
		return err
	}
	r.Config = cfg
	return nil
}
