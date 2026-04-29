package synth

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"awiki/internal/fsutil"
)

// Finalize stamps a synthesis page with the current scope hash, source list,
// and last_generated timestamp, then optionally runs the plugin post-hook.
// It mirrors scripts/synth.sh:cmd_finalize.
//
// Target resolution:
//  1. If <synthDir>/.staged/<slug>.md exists, operate on the staged copy.
//  2. Otherwise operate on <synthDir>/<slug>.md (live).
//  3. Either way, exit 1 if the chosen target does not exist.
//
// Steps:
//  1. CheckMarkers → exit 5 on failure.
//  2. Lint (--only=synth) → exit 6 on non-zero.
//  3. Re-resolve scope from target's frontmatter.
//  4. Compute scope hash; stamp BEGIN line, sources:, last_generated:.
//  5. AtomicWrite updated content.
//  6. Plugin post-hook (gated on ALLOW_PLUGIN_POST_HOOKS=1) → exit 6 on failure.
//  7. Append log entry.
//  8. Bump .awiki/ingest-count.
//  9. Emit SYNTH-FINALIZE telemetry to stderr.
func (r *Runner) Finalize(ctx context.Context, slug string, stderr io.Writer) error {
	live := filepath.Join(r.synthDir(), slug+".md")
	staged := filepath.Join(r.synthDir(), ".staged", slug+".md")
	target := live
	if _, err := os.Stat(staged); err == nil {
		target = staged
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return &ExitError{Code: 1, Msg: "synthesis page not found: " + target}
	}
	body := string(data)

	if err := CheckMarkers(body); err != nil {
		return &ExitError{Code: 5, Msg: err.Error()}
	}

	if _, code, _ := r.Lint.Run(ctx, r.RepoRoot, "--only=synth", "--file="+target); code != 0 {
		return &ExitError{Code: 6, Msg: "scoped lint failed for " + target}
	}

	// Re-resolve scope canonically from the target's frontmatter.
	slugs, err := r.resolveSlugs(ctx, target)
	if err != nil {
		return err
	}
	hash := ScopeHash(slugs)

	// Stamp BEGIN line, sources:, last_generated:.
	body = SetScopeHashInMarker(body, hash)
	body = FmSetSources(body, slugs)
	body = FmSetScalar(body, "last_generated", time.Now().UTC().Format("2006-01-02T15:04:05Z"))

	if err := fsutil.AtomicWrite(target, []byte(body)); err != nil {
		return err
	}

	pluginName := readScalar(body, "plugin")
	if pluginName != "" {
		p, perr := LoadPlugin(filepath.Join(r.PluginDir, pluginName+".md"))
		if perr == nil && p.PostHook != "" {
			if hookErr := r.RunPluginPostHook(ctx, p, target, stderr); hookErr != nil {
				return &ExitError{Code: 6, Msg: "post-hook failed for " + target}
			}
		}
	}

	r.LogAppend(ctx, "synth", pluginName+" "+slug)

	// Bump .awiki/ingest-count.
	cntPath := filepath.Join(r.RepoRoot, ".awiki", "ingest-count")
	cur := 0
	if b, e := os.ReadFile(cntPath); e == nil {
		cur, _ = strconv.Atoi(strings.TrimSpace(string(b)))
	}
	_ = os.MkdirAll(filepath.Dir(cntPath), 0o755)
	_ = os.WriteFile(cntPath, []byte(strconv.Itoa(cur+1)+"\n"), 0o644)

	srcCount := 0
	for _, s := range slugs {
		if s != "" {
			srcCount++
		}
	}
	fmt.Fprintf(stderr, "SYNTH-FINALIZE|target=%s|sources=%d|scope_hash=%s\n", target, srcCount, hash)
	return nil
}
