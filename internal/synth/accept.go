package synth

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"awiki/internal/fsutil"
)

// AcceptStage promotes a staged synthesis file to the live location.
// It mirrors scripts/synth.sh:cmd_accept_stage.
//
//  1. Staged file must exist at <synthDir>/.staged/<slug>.md → exit 7 if missing.
//  2. Scoped lint is run against the staged file → exit 6 on failure.
//  3. The staged file is moved to <synthDir>/<slug>.md (atomic if live already exists).
//  4. If the plugin declares a post_hook and post-hooks are allowed, it is run.
//  5. A synth log entry is appended.
//  6. SYNTH-ACCEPT-STAGE|target=<live> is emitted to stderr.
func (r *Runner) AcceptStage(ctx context.Context, slug string, stderr io.Writer) error {
	stagedDir := filepath.Join(r.synthDir(), ".staged")
	staged := filepath.Join(stagedDir, slug+".md")
	live := filepath.Join(r.synthDir(), slug+".md")

	if _, err := os.Stat(staged); err != nil {
		return &ExitError{Code: 7, Msg: "no staged file at " + staged}
	}

	if _, code, _ := r.Lint.Run(ctx, r.RepoRoot, "--only=synth", "--file="+staged); code != 0 {
		return &ExitError{Code: 6, Msg: "scoped lint failed for staged file"}
	}

	data, err := os.ReadFile(staged)
	if err != nil {
		return err
	}

	// Promote staged → live. AtomicWrite requires an existing target;
	// if live does not yet exist use WriteFile instead.
	if _, statErr := os.Stat(live); statErr == nil {
		if err := fsutil.AtomicWrite(live, data); err != nil {
			return err
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(live), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(live, data, 0o644); err != nil {
			return err
		}
	}

	if err := os.Remove(staged); err != nil {
		return err
	}

	pluginName := readScalar(string(data), "plugin")
	if pluginName != "" {
		p, perr := LoadPlugin(filepath.Join(r.PluginDir, pluginName+".md"))
		if perr == nil && p.PostHook != "" {
			if hookErr := r.RunPluginPostHook(ctx, p, live, stderr); hookErr != nil {
				return &ExitError{Code: 6, Msg: "post-hook failed for " + live}
			}
		}
	}

	r.LogAppend(ctx, "synth", pluginName+" "+slug+" accept-stage")
	fmt.Fprintf(stderr, "SYNTH-ACCEPT-STAGE|target=%s\n", live)
	return nil
}
