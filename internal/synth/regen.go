package synth

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"awiki/internal/fsutil"
)

// RegenOptions controls the behaviour of the Regen verb.
type RegenOptions struct {
	Slug  string
	Force bool
	Stage bool
}

// Regen re-generates the prompt for an existing synthesis page:
//  1. Reads the live page; exits 1 if missing.
//  2. Reads the plugin: field; exits 1 if missing or unloadable.
//  3. If !Force && !Stage: runs HandEditCheck; exits 4 on hand-edit.
//  4. Privacy fail-closed probe: all resolved slugs (privacy skipped) are
//     inspected; if any has the "private" tag and the synth page is not
//     private → exits 2.
//  5. Canonical resolve (privacy enforced).
//  6. Computes scope hash.
//  7. If Stage: copies live to staged dir; operates on staged copy.
//  8. Clears the marker region (rewrites BEGIN line, empties body).
//  9. Emits prompt to stdout.
//  10. Writes telemetry line to stderr.
//
// Matches scripts/synth.sh:cmd_regen.
func (r *Runner) Regen(ctx context.Context, opts RegenOptions, stdout, stderr io.Writer) error {
	live := filepath.Join(r.synthDir(), opts.Slug+".md")
	data, err := os.ReadFile(live)
	if err != nil {
		return &ExitError{Code: 1, Msg: "synthesis page not found: " + live}
	}
	body := string(data)

	plugin := readScalar(body, "plugin")
	if plugin == "" {
		return &ExitError{Code: 1, Msg: live + " missing 'plugin:' frontmatter"}
	}
	p, err := LoadPlugin(filepath.Join(r.PluginDir, plugin+".md"))
	if err != nil {
		return &ExitError{Code: 1, Msg: "plugin load failed: " + plugin}
	}

	if !opts.Force && !opts.Stage {
		edited, untracked, herr := r.HandEditCheck(ctx, live, body)
		if herr != nil {
			return herr
		}
		if !untracked && edited {
			return &ExitError{Code: 4, Msg: "hand-edit detected inside marker region; pass --force or --stage"}
		}
	}

	// Privacy fail-closed probe: resolve with privacy filter disabled so we
	// see all pages that would be in scope. If any source page has tag
	// "private" and the target synthesis page is not private → error.
	targetPrivate := isPrivatePath(live)
	allSlugs, err := r.resolveSlugsWithPrivacy(ctx, live)
	if err != nil {
		return err
	}
	if !targetPrivate {
		for _, s := range allSlugs {
			path := r.slugToPath(s)
			if path == "" {
				continue
			}
			if hasTag(readPageTags(path), "private") {
				return &ExitError{
					Code: 2,
					Msg:  "private source " + s + " now in scope; tag the synthesis page private or pass --allow-private (regen)",
				}
			}
		}
	}

	// Canonical resolve (privacy enforced).
	slugs, err := r.resolveSlugs(ctx, live)
	if err != nil {
		return err
	}
	hash := ScopeHash(slugs)

	// Determine scope description from the parsed scope.
	fmText := extractFrontmatterText(body)
	scope := ParseScopeFromFrontmatter(fmText)
	scopeDesc := scopeDescription(scope)

	// Choose target file: live or staged copy.
	target := live
	if opts.Stage {
		stagedDir := filepath.Join(r.synthDir(), ".staged")
		if err := os.MkdirAll(stagedDir, 0o755); err != nil {
			return err
		}
		target = filepath.Join(stagedDir, opts.Slug+".md")
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}

	// Read current target content (may be live or just-written staged copy).
	cur, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	cleared, err := ClearRegion(string(cur), plugin, hash)
	if err != nil {
		return err
	}
	if err := fsutil.AtomicWrite(target, []byte(cleared)); err != nil {
		return err
	}

	// Render feedback from the target (which is either the live page or the
	// staged copy — either way feedback is outside the markers and survives
	// ClearRegion untouched).
	feedback := renderFeedbackBlock(cur)

	if err := r.EmitPrompt(stdout, p, slugs, scopeDesc, feedback); err != nil {
		return err
	}

	stageFlag := 0
	if opts.Stage {
		stageFlag = 1
	}
	fmt.Fprintf(stderr, "SYNTH-REGEN|target=%s|stage=%d|scope_hash=%s\n", target, stageFlag, hash)
	return nil
}
