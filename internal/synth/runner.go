package synth

import (
	"context"
	"os/exec"
	"path/filepath"

	"awiki/internal/adapters"
	"awiki/internal/config"
)

// ExitError is a sentinel error carrying an OS exit code. Synth verbs
// return it when they want the CLI dispatcher to exit with a specific
// code (e.g. exit 4 for hand-edit detected, exit 2 for privacy
// violation).
type ExitError struct {
	Code int
	Msg  string
}

func (e *ExitError) Error() string { return e.Msg }
func (e *ExitError) ExitCode() int  { return e.Code }

// Runner is the per-invocation context shared by every synth verb.
type Runner struct {
	RepoRoot   string
	ContentDir string
	PluginDir  string
	Config     map[string]string
	Qmd        adapters.Qmd
	PostHook   adapters.PostHook
	Git        adapters.SynthGit
	Lint       adapters.SynthLint
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

// SynthDir returns the synthesis pages directory (exported alias of synthDir).
func (r *Runner) SynthDir() string {
	return r.synthDir()
}

// LogAppend shells scripts/log-append.sh to record a journal entry.
// Errors are silently ignored (matches bash behavior).
func (r *Runner) LogAppend(ctx context.Context, topic, msg string) {
	cmd := exec.CommandContext(ctx, "bash",
		filepath.Join(r.RepoRoot, "scripts", "log-append.sh"), topic, "--", msg)
	cmd.Run() //nolint:errcheck
}
