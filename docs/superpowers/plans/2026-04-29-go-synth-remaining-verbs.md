# Go synth remaining verbs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement task-by-task.

**Goal:** Port `awiki synth {new, regen, accept-stage, finalize}` plus
shared helpers (`ClearRegion`, `CheckMarkers`, frontmatter setters,
`HandEditCheck`, `EmitPrompt`, `WriteScaffold`, `RunPluginPostHook`)
plus two new exec adapters (`ExecGit`, `ExecLint`).

**Architecture:** Add helpers + adapters first (Task 1), then the
four verbs in dependency order: regen (Task 2), accept-stage (Task 3),
finalize (Task 4), new (Task 5). Each verb sub-slice flips its branch
in `scripts/synth.sh`'s `AWIKI_SYNTH_GO_VERBS` array.

**Spec:** `docs/superpowers/specs/2026-04-29-go-synth-remaining-verbs-design.md`.

**Bash oracle:** `scripts/synth.sh` `cmd_new`, `cmd_regen`,
`cmd_accept_stage`, `cmd_finalize` plus the helpers
`synth_clear_region`, `synth_check_markers`, `synth_handedit_check`,
`synth_fm_set_scalar`, `synth_fm_set_sources`,
`synth_fm_set_scope_hash_in_marker`, `synth_emit_prompt`,
`synth_pages_block`, `synth_render_feedback_block`,
`synth_write_scaffold`, `synth_post_hooks_allowed`.

**Branch:** `feat/synth-remaining-verbs` in worktree
`.worktrees/synth-remaining-verbs`.

---

## Task 1: shared helpers + adapters

**Files (all in worktree under `internal/`):**
- Create: `adapters/git.go`, `adapters/git_test.go`
- Create: `adapters/lint.go`, `adapters/lint_test.go`
- Create: `synth/region.go`, `synth/region_test.go`
- Create: `synth/frontmatter.go`, `synth/frontmatter_test.go`
- Create: `synth/handedit.go`, `synth/handedit_test.go`
- Create: `synth/prompt.go`, `synth/prompt_test.go`
- Create: `synth/scaffold.go`, `synth/scaffold_test.go`
- Create: `synth/posthook.go`, `synth/posthook_test.go`
- Modify: `synth/runner.go` — add `Git`, `Lint` fields
- Modify: `cli/synth.go` — wire `Git` + `Lint` in `buildSynthRunner`

### `adapters/git.go`

```go
package adapters

import (
	"context"
	"errors"
	"os/exec"
)

// Git is the minimal git CLI wrapper synth verbs use. ExecGit
// implements it via os/exec; tests inject fakes.
type Git interface {
	LsFiles(ctx context.Context, repoRoot, path string) (tracked bool, err error)
	ShowHead(ctx context.Context, repoRoot, path string) (content string, ok bool, err error)
}

type ExecGit struct{}

func (ExecGit) LsFiles(ctx context.Context, repoRoot, path string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-files", "--error-unmatch", "--", path)
	cmd.Dir = repoRoot
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (ExecGit) ShowHead(ctx context.Context, repoRoot, path string) (string, bool, error) {
	if err := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "HEAD").Run(); err != nil {
		return "", false, nil
	}
	cmd := exec.CommandContext(ctx, "git", "show", "HEAD:"+path)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", false, nil
		}
		return "", false, err
	}
	return string(out), true, nil
}
```

`adapters/git_test.go`: compile-time assertion `var _ Git = (*ExecGit)(nil)`.

### `adapters/lint.go`

```go
package adapters

import (
	"context"
	"errors"
	"os/exec"
)

// Lint shells `bash <repoRoot>/scripts/lint.sh <args...>`.
type Lint interface {
	Run(ctx context.Context, repoRoot string, args ...string) (output string, code int, err error)
}

type ExecLint struct{}

func (ExecLint) Run(ctx context.Context, repoRoot string, args ...string) (string, int, error) {
	all := append([]string{"scripts/lint.sh"}, args...)
	cmd := exec.CommandContext(ctx, "bash", all...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode(), err
	}
	return string(out), 127, err
}
```

`adapters/lint_test.go`: compile-time assertion only.

### `synth/region.go`

```go
package synth

import (
	"regexp"
	"strings"

	"awiki/internal/region"
)

// ClearRegion replaces the body of the BEGIN GENERATED region in
// `text` with a single blank line, and rewrites the BEGIN line to
// declare the given plugin and scope_hash. Returns the new text.
// If markers are missing/duplicate/inverted, returns the input
// unchanged plus an error.
func ClearRegion(text, plugin, scopeHash string) (string, error) {
	gen, diags := region.ParseGenerated(text)
	if len(diags) > 0 {
		return text, &MarkerError{Diagnostics: diags}
	}
	beginLine := "<!-- BEGIN GENERATED plugin=" + plugin +
		" scope_hash=" + scopeHash + " -->"
	prefix := text[:gen.BeginOffset]
	// gen.Body starts after the BEGIN line's trailing \n; gen.EndOffset
	// is the start of the END marker.
	endStart := gen.EndOffset
	suffix := text[endStart:]
	// New region: BEGIN line + \n + blank line + \n? Bash output:
	//   <BEGIN line>\n
	//   \n
	//   <END marker>
	body := beginLine + "\n\n"
	return prefix + body + suffix, nil
}

// CheckMarkers verifies BEGIN GENERATED marker count and order.
// Returns nil on valid markers; *MarkerError otherwise. Matches
// scripts/synth.sh:synth_check_markers — exactly one BEGIN and one
// END, BEGIN before END.
func CheckMarkers(text string) error {
	begins := strings.Count(text, "<!-- BEGIN GENERATED ")
	ends := strings.Count(text, "<!-- END GENERATED -->")
	if begins != 1 || ends != 1 {
		return &MarkerError{Message: "marker integrity failure: BEGIN=" + itoa(begins) +
			" END=" + itoa(ends)}
	}
	bIdx := strings.Index(text, "<!-- BEGIN GENERATED ")
	eIdx := strings.Index(text, "<!-- END GENERATED -->")
	if bIdx > eIdx {
		return &MarkerError{Message: "BEGIN marker after END marker"}
	}
	return nil
}

var scopeHashRE = regexp.MustCompile(`scope_hash=[0-9a-f]+`)

// SetScopeHashInMarker replaces the scope_hash=… token in the BEGIN
// GENERATED line. No-op if the line has no scope_hash.
func SetScopeHashInMarker(text, hash string) string {
	gen, diags := region.ParseGenerated(text)
	if len(diags) > 0 {
		return text
	}
	begin := text[gen.BeginOffset : gen.BeginOffset+len(gen.BeginLine)]
	updated := scopeHashRE.ReplaceAllString(begin, "scope_hash="+hash)
	return text[:gen.BeginOffset] + updated + text[gen.BeginOffset+len(gen.BeginLine):]
}

// MarkerError is returned by ClearRegion / CheckMarkers when the
// BEGIN/END markers are missing, duplicated, or out of order.
type MarkerError struct {
	Message     string
	Diagnostics []region.Diagnostic
}

func (e *MarkerError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if len(e.Diagnostics) > 0 {
		return e.Diagnostics[0].Message
	}
	return "marker error"
}

func itoa(n int) string {
	// avoids strconv import (already pulled in elsewhere; harmless if duplicated)
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
```

`region_test.go`: cover happy path (ClearRegion preserves markers, body cleared), duplicate-BEGIN error, missing-END error, BEGIN-after-END, scope_hash replacement. Use small fixture strings.

### `synth/frontmatter.go`

```go
package synth

import (
	"strings"
)

// FmSetScalar replaces the `<key>: ...` line in the YAML frontmatter
// at the start of body. If the key is absent, the input is returned
// unchanged. Matches scripts/synth.sh:synth_fm_set_scalar (which only
// rewrites; never inserts).
func FmSetScalar(body, key, value string) string {
	prefix := "---\n"
	if !strings.HasPrefix(body, prefix) {
		return body
	}
	rest := body[len(prefix):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return body
	}
	fm := rest[:end]
	tail := rest[end:]
	lines := strings.Split(fm, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, key+":") {
			lines[i] = key + ": " + value
			return prefix + strings.Join(lines, "\n") + tail
		}
	}
	return body
}

// FmSetSources rewrites the `sources:` line in frontmatter to
// inline-list form: ["[[s1]]", "[[s2]]"]. Slugs already sorted.
// Matches scripts/synth.sh:synth_fm_set_sources.
func FmSetSources(body string, slugs []string) string {
	parts := make([]string, 0, len(slugs))
	for _, s := range slugs {
		if s == "" {
			continue
		}
		parts = append(parts, `"[[`+s+`]]"`)
	}
	value := "[" + strings.Join(parts, ", ") + "]"
	return FmSetScalar(body, "sources", value)
}
```

`frontmatter_test.go`: scalar replace happy path, missing-key no-op, sources empty list, sources multiple slugs.

### `synth/handedit.go`

```go
package synth

import (
	"context"
	"strings"
)

// HandEditCheck reports whether the BEGIN..END region of the live
// page has diverged from its committed (HEAD) version. Returns
// (handEdited, untracked, err).
//
// Matches scripts/synth.sh:synth_handedit_check semantics:
//   - untracked file → untracked=true (caller treats as fresh)
//   - no HEAD yet → untracked=true
//   - regions equal → handEdited=false
//   - regions differ → handEdited=true
func (r *Runner) HandEditCheck(ctx context.Context, pagePath, workingText string) (handEdited bool, untracked bool, err error) {
	tracked, err := r.Git.LsFiles(ctx, r.RepoRoot, pagePath)
	if err != nil {
		return false, false, err
	}
	if !tracked {
		return false, true, nil
	}
	head, ok, err := r.Git.ShowHead(ctx, r.RepoRoot, pagePath)
	if err != nil {
		return false, false, err
	}
	if !ok {
		return false, true, nil
	}
	return extractRegion(head) != extractRegion(workingText), false, nil
}

// extractRegion returns lines from BEGIN GENERATED through END
// GENERATED inclusive (both as substrings). Mirrors the bash awk
// extractor.
func extractRegion(text string) string {
	beginIdx := strings.Index(text, "<!-- BEGIN GENERATED ")
	endIdx := strings.Index(text, "<!-- END GENERATED -->")
	if beginIdx < 0 || endIdx < 0 || beginIdx > endIdx {
		return ""
	}
	endLineEnd := endIdx + len("<!-- END GENERATED -->")
	if endLineEnd < len(text) && text[endLineEnd] == '\n' {
		endLineEnd++
	}
	return text[beginIdx:endLineEnd]
}
```

`handedit_test.go`: untracked, no HEAD, equal regions, divergent regions, missing markers.

### `synth/prompt.go`

```go
package synth

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// EmitPrompt writes the rendered plugin prompt bundle to out.
// scopeDesc replaces {{scope_description}}. The {{#pages}}...
// {{/pages}} block is replaced with one row per slug. The
// {{#feedback}}...{{/feedback}} block is dropped if feedback is
// empty; otherwise the {{feedback}} placeholder inside is replaced
// with the feedback string and the {{#feedback}}/{{/feedback}}
// delimiter lines are stripped.
//
// Matches scripts/synth.sh:synth_emit_prompt.
func (r *Runner) EmitPrompt(out io.Writer, plugin Plugin, slugs []string, scopeDesc, feedback string) error {
	body := plugin.Body
	body = strings.ReplaceAll(body, "{{scope_description}}", scopeDesc)
	body = expandPagesBlock(body, r.pagesBlock(slugs))
	body = expandFeedbackBlock(body, feedback)
	_, err := io.WriteString(out, body)
	return err
}

// pagesBlock returns the row-per-slug content used to fill the
// {{#pages}}...{{/pages}} block. Each row:
//   - [[<slug>]] (<type>) — <lead-up-to-200-chars>
func (r *Runner) pagesBlock(slugs []string) string {
	var b strings.Builder
	for _, slug := range slugs {
		path := r.slugToPath(slug)
		if path == "" {
			continue
		}
		fm, body, ok := readFrontmatterAndBody(path)
		if !ok {
			continue
		}
		title := strings.Trim(fm["title"], `"`)
		_ = title // bash sets but row format does not include title
		typ := fm["type"]
		lead := firstNonBlankLine(body)
		if len(lead) > 200 {
			lead = lead[:200]
		}
		fmt.Fprintf(&b, "- [[%s]] (%s) — %s\n", slug, typ, lead)
	}
	return strings.TrimRight(b.String(), "\n")
}

func expandPagesBlock(body, rows string) string {
	const open = "{{#pages}}"
	const close = "{{/pages}}"
	startIdx := strings.Index(body, open)
	if startIdx < 0 {
		return body
	}
	endIdx := strings.Index(body[startIdx:], close)
	if endIdx < 0 {
		return body
	}
	endIdx += startIdx + len(close)
	// Bash skips the BEGIN delimiter line and replaces the END line
	// with the rows: net effect is to remove both delimiter lines and
	// substitute rows in place. Bash uses awk per-line; here we slice.
	prefix := trimTrailingNewline(body[:startIdx])
	suffix := stripLeadingNewline(body[endIdx:])
	return prefix + rows + "\n" + suffix
}

func expandFeedbackBlock(body, feedback string) string {
	const open = "{{#feedback}}"
	const close = "{{/feedback}}"
	startIdx := strings.Index(body, open)
	if startIdx < 0 {
		return body
	}
	endIdx := strings.Index(body[startIdx:], close)
	if endIdx < 0 {
		return body
	}
	endIdx += startIdx + len(close)
	if feedback == "" {
		// drop the entire wrapped region
		prefix := trimTrailingNewline(body[:startIdx])
		suffix := stripLeadingNewline(body[endIdx:])
		return prefix + suffix
	}
	// substitute {{feedback}} inside the inner region; strip the
	// delimiter lines.
	inner := body[startIdx+len(open) : endIdx-len(close)]
	inner = strings.ReplaceAll(inner, "{{feedback}}", feedback)
	inner = strings.TrimPrefix(inner, "\n")
	inner = strings.TrimSuffix(inner, "\n")
	prefix := trimTrailingNewline(body[:startIdx])
	suffix := stripLeadingNewline(body[endIdx:])
	return prefix + "\n" + inner + "\n" + suffix
}

// renderFeedbackBlock builds the fenced text block that
// {{#feedback}}...{{/feedback}} consumes when the regen path has
// non-empty feedback. Matches scripts/synth.sh:synth_render_feedback_block.
func renderFeedbackBlock(pageBytes []byte) string {
	lines := strings.Split(string(pageBytes), "\n")
	inBlock := false
	var bullets []string
	for _, l := range lines {
		t := strings.TrimRight(l, "\r")
		if t == "## Feedback" {
			inBlock = true
			continue
		}
		if inBlock {
			if strings.HasPrefix(t, "## ") {
				inBlock = false
				continue
			}
			if strings.HasPrefix(t, "<!-- BEGIN GENERATED") {
				inBlock = false
				continue
			}
			if strings.HasPrefix(t, "- ") {
				bullets = append(bullets, strings.TrimPrefix(t, "- "))
			}
		}
	}
	if len(bullets) == 0 {
		return ""
	}
	return "```text\n" + strings.Join(bullets, "\n") + "\n```"
}

func trimTrailingNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s[:len(s)-1]
	}
	return s
}

func stripLeadingNewline(s string) string {
	if strings.HasPrefix(s, "\n") {
		return s[1:]
	}
	return s
}

func firstNonBlankLine(body string) string {
	for _, l := range strings.Split(body, "\n") {
		if strings.TrimSpace(l) != "" {
			return l
		}
	}
	return ""
}

func readFrontmatterAndBody(path string) (map[string]string, string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", false
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		return nil, text, false
	}
	rest := text[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return nil, text, false
	}
	fm := map[string]string{}
	for _, l := range strings.Split(rest[:end], "\n") {
		if i := strings.Index(l, ":"); i > 0 {
			fm[strings.TrimSpace(l[:i])] = strings.TrimSpace(l[i+1:])
		}
	}
	body := rest[end+len("\n---\n"):]
	return fm, body, true
}
```

`prompt_test.go`: cover scope_description literal substitution,
pages block expansion (single + multi slug), feedback block dropped
when empty, feedback block rendered when present.

### `synth/scaffold.go`

```go
package synth

import (
	"strings"
)

// ScaffoldInput holds the inputs for WriteScaffold.
type ScaffoldInput struct {
	Topic       string
	Plugin      string
	Today       string // date YYYY-MM-DD
	ScopeBlock  string // full scope: ... block, ending with newline
	SourcesYAML string // e.g. "sources: []\n"
	ScopeHash   string
}

// WriteScaffold returns the page bytes the bash
// synth_write_scaffold heredoc emits, byte-equivalent except for
// time-dependent fields the caller supplies.
func WriteScaffold(in ScaffoldInput) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(`title: "` + in.Topic + ` — ` + in.Plugin + `"` + "\n")
	b.WriteString("date: " + in.Today + "\n")
	b.WriteString("last_updated: " + in.Today + "\n")
	b.WriteString("last_generated:\n")
	b.WriteString("type: synthesis\n")
	b.WriteString("plugin: " + in.Plugin + "\n")
	b.WriteString(in.ScopeBlock)
	b.WriteString("tags: [" + in.Topic + ", " + in.Plugin + "]\n")
	b.WriteString("aliases: []\n")
	b.WriteString(in.SourcesYAML)
	b.WriteString("draft: false\n")
	b.WriteString("---\n")
	b.WriteString("\n")
	b.WriteString("Lead paragraph — written once by user/agent, NOT regenerated.\n")
	b.WriteString("\n")
	b.WriteString("## Notes\n")
	b.WriteString("\n")
	b.WriteString("<!-- user notes; survives regen -->\n")
	b.WriteString("\n")
	b.WriteString("<!-- BEGIN GENERATED plugin=" + in.Plugin +
		" scope_hash=" + in.ScopeHash + " -->\n")
	b.WriteString("\n")
	b.WriteString("<!-- END GENERATED -->\n")
	return []byte(b.String())
}
```

`scaffold_test.go`: golden byte comparison against the bash heredoc
output for one fixture (`topic="t1"`, `plugin="briefing"`, simple
scope block).

### `synth/posthook.go`

```go
package synth

import (
	"context"
	"fmt"
	"io"
	"os"
)

// RunPluginPostHook runs the plugin's post-hook script if declared
// and gated by ALLOW_PLUGIN_POST_HOOKS=1 in .awiki/config. Stderr
// emits status records mirroring scripts/synth.sh.
func (r *Runner) RunPluginPostHook(ctx context.Context, plugin Plugin, pagePath string, stderr io.Writer) error {
	if plugin.PostHook == "" {
		return nil
	}
	if !r.AllowPostHooks() {
		fmt.Fprintln(stderr, "SYNTH|post-hook-blocked|ALLOW_PLUGIN_POST_HOOKS=0; set to 1 in .awiki/config to enable")
		return nil
	}
	script := plugin.PostHook
	info, err := os.Stat(script)
	if err != nil || (info.Mode()&0o111 == 0 && !info.Mode().IsRegular()) {
		// matches bash: only run if executable or a regular file
		return nil
	}
	out, code, err := r.PostHook.Run(ctx, script, pagePath)
	if err != nil || code != 0 {
		fmt.Fprintf(stderr, "SYNTH|post-hook-failed|%s exit non-zero\n", script)
		_ = out
		return fmt.Errorf("post-hook failed: %s", script)
	}
	return nil
}
```

`posthook_test.go`: gated-blocked path emits the stderr line and
returns nil; ungated success returns nil; ungated failure emits the
failure line and returns error.

### Runner update

Add to `runner.go`:

```go
import "awiki/internal/adapters"

type Runner struct {
	// existing fields...
	Git  adapters.Git
	Lint adapters.Lint
}
```

In `cli/synth.go::buildSynthRunner`, set:

```go
Git:  adapters.ExecGit{},
Lint: adapters.ExecLint{},
```

### Verification

- `go build ./...` clean.
- `go test ./...` clean (helpers tested; verbs not yet implemented).
- Commit:

```
git add internal/adapters/git.go internal/adapters/git_test.go \
        internal/adapters/lint.go internal/adapters/lint_test.go \
        internal/synth/region.go internal/synth/region_test.go \
        internal/synth/frontmatter.go internal/synth/frontmatter_test.go \
        internal/synth/handedit.go internal/synth/handedit_test.go \
        internal/synth/prompt.go internal/synth/prompt_test.go \
        internal/synth/scaffold.go internal/synth/scaffold_test.go \
        internal/synth/posthook.go internal/synth/posthook_test.go \
        internal/synth/runner.go internal/cli/synth.go
git commit -m "feat: add synth helpers + git/lint adapters for upcoming verbs"
```

---

## Task 2: regen verb

**Files:**
- Create: `internal/synth/regen.go`, `internal/synth/regen_test.go`
- Modify: `internal/cli/synth.go` (wire `regen` case)
- Modify: `scripts/synth.sh` (add `regen` to GO_VERBS array)

Implementation outline (see spec §`regen`):

```go
package synth

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"awiki/internal/fsutil"
)

// Regen rewrites the GENERATED region of the synth page (or copies
// to staged dir first when stage=true), then emits the prompt bundle
// to stdout. See spec.
type RegenOptions struct {
	Slug  string
	Force bool
	Stage bool
}

func (r *Runner) Regen(ctx context.Context, opts RegenOptions, stdout, stderr io.Writer) error {
	live := filepath.Join(r.SynthDir(), opts.Slug+".md")
	data, err := os.ReadFile(live)
	if err != nil {
		return fmt.Errorf("synthesis page not found: %s", live)
	}
	body := string(data)

	plugin := readScalar(body, "plugin")
	if plugin == "" {
		return fmt.Errorf("%s missing 'plugin:' frontmatter", live)
	}
	p, err := LoadPlugin(filepath.Join(r.PluginDir, plugin+".md"))
	if err != nil {
		return fmt.Errorf("plugin load failed: %s", plugin)
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

	scope := ParseScopeFromFrontmatter(extractFrontmatter(body))
	scopeDesc := scopeDescription(scope)

	// Privacy fail-closed re-check
	scope.AllowPrivate = true
	allSlugs, err := r.resolveSlugs(ctx, scope, live)
	if err != nil {
		return err
	}
	targetPrivate := isPrivatePath(live)
	if !targetPrivate {
		for _, s := range allSlugs {
			tags := readPageTags(r.slugToPath(s))
			if hasTag(tags, "private") {
				return &ExitError{Code: 2,
					Msg: "private source " + s + " now in scope; tag the synthesis page private or pass --allow-private (regen)"}
			}
		}
	}

	// Canonical resolve (with privacy)
	scope.AllowPrivate = targetPrivate
	slugs, err := r.resolveSlugs(ctx, scope, live)
	if err != nil {
		return err
	}
	sort.Strings(slugs)
	hash := ScopeHash(slugs)

	target := live
	if opts.Stage {
		stagedDir := filepath.Join(r.ContentDir, ".synth-staged")
		if err := os.MkdirAll(stagedDir, 0o755); err != nil {
			return err
		}
		target = filepath.Join(stagedDir, opts.Slug+".md")
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}

	// Read CURRENT target bytes for ClearRegion + feedback render
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

// helpers (define if not already present elsewhere):
//   readScalar(body, key)            // single scalar from frontmatter
//   extractFrontmatter(body)         // content between leading and second `---`
//   isPrivatePath(path)              // path contains "/private/"
//   readPageTags(path)               // []string, empty if missing
//   hasTag(tags, t)                  // membership
//   scopeDescription(s Scope)        // mirrors bash strings:
//                                    //   "pages tagged 'X'"
//                                    //   "explicit slug list"
//                                    //   "qmd query: <q>"
//   r.resolveSlugs(ctx, scope, livePath) ([]string, error)
//                                    // wraps existing resolve logic
```

Tests (`regen_test.go`):
- Live regen (no stage): region cleared, BEGIN line carries new
  hash, body byte-equivalent except region body and last_updated
  not changed (bash regen does not bump last_updated).
- `--stage`: live untouched; staged file under `.synth-staged/`.
- `--force`: hand-edit detection skipped.
- Hand-edit exit 4 (mock `Git` adapter returns divergent HEAD).
- Untracked file: hand-edit check returns no error, regen proceeds.
- Privacy fail-closed: scope returns a slug whose page has tag
  `private`; target not private → exit 2.

CLI wire: replace the `case "regen":` placeholder with a real
`runSynthRegen` that flag-parses `--force`/`--stage`/`--`, validates
slug, calls `r.Regen`. Map `*ExitError` to its `.Code`; non-typed
errors → exit 2 + stderr.

`scripts/synth.sh`: add `regen` to `AWIKI_SYNTH_GO_VERBS`.

Smoke verify:

```bash
slug="$(ls content/synthesis/*.md | head -1 | xargs -I{} basename {} .md)"
[ -n "$slug" ] && {
  cp "content/synthesis/$slug.md" /tmp/orig.md
  AWIKI_SYNTH_LEGACY=1 bash scripts/synth.sh regen "$slug" --force >/tmp/bash.stdout 2>/tmp/bash.stderr
  cp "content/synthesis/$slug.md" /tmp/bash.md
  cp /tmp/orig.md "content/synthesis/$slug.md"
  ./bin/awiki synth regen "$slug" --force >/tmp/go.stdout 2>/tmp/go.stderr
  cp "content/synthesis/$slug.md" /tmp/go.md
  cp /tmp/orig.md "content/synthesis/$slug.md"
  diff /tmp/bash.md /tmp/go.md && echo "PAGE: equal"
  diff /tmp/bash.stderr /tmp/go.stderr && echo "STDERR: equal"
  diff /tmp/bash.stdout /tmp/go.stdout && echo "PROMPT: equal"
}
```

Must show all three "equal".

Bats 774 still pass.

Commit: `feat: port synth regen verb`.

---

## Task 3: accept-stage verb

**Files:**
- Create: `internal/synth/accept.go`, `internal/synth/accept_test.go`
- Modify: `internal/cli/synth.go` (wire `accept-stage` case)
- Modify: `scripts/synth.sh` (add `accept-stage` to GO_VERBS)

```go
package synth

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"awiki/internal/fsutil"
)

func (r *Runner) AcceptStage(ctx context.Context, slug string, stderr io.Writer) error {
	staged := filepath.Join(r.ContentDir, ".synth-staged", slug+".md")
	live := filepath.Join(r.SynthDir(), slug+".md")
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
	if err := fsutil.AtomicWrite(live, data); err != nil {
		return err
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
```

`LogAppend` is a small `Runner` method that shells
`bash <repoRoot>/scripts/log-append.sh <topic> -- <message>`.
Add it to `runner.go` if not yet present.

Tests:
- Missing staged → exit 7.
- Lint fails → exit 6, live untouched.
- Lint passes, no post-hook → live updated, staged removed,
  stderr record emitted.
- Lint passes, post-hook fails (gated; mock PostHook adapter) →
  exit 6.
- Lint passes, post-hook blocked (config disabled) → exit 0,
  stderr blocked record.

Smoke (against a real staged file): create one, accept, diff
against bash equivalent.

Commit: `feat: port synth accept-stage verb`.

---

## Task 4: finalize verb

**Files:**
- Create: `internal/synth/finalize.go`, `internal/synth/finalize_test.go`
- Modify: `internal/cli/synth.go` (wire `finalize` case)
- Modify: `scripts/synth.sh` (add `finalize` to GO_VERBS)

```go
package synth

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"awiki/internal/fsutil"
)

func (r *Runner) Finalize(ctx context.Context, slug string, stderr io.Writer) error {
	live := filepath.Join(r.SynthDir(), slug+".md")
	staged := filepath.Join(r.ContentDir, ".synth-staged", slug+".md")
	target := live
	if _, err := os.Stat(staged); err == nil {
		target = staged
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("synthesis page not found: %s", target)
	}
	body := string(data)

	if err := CheckMarkers(body); err != nil {
		return &ExitError{Code: 5, Msg: err.Error()}
	}
	if _, code, _ := r.Lint.Run(ctx, r.RepoRoot, "--only=synth", "--file="+target); code != 0 {
		return &ExitError{Code: 6, Msg: "scoped lint failed for " + target}
	}

	scope := ParseScopeFromFrontmatter(extractFrontmatter(body))
	scope.AllowPrivate = isPrivatePath(target)
	slugs, err := r.resolveSlugs(ctx, scope, target)
	if err != nil {
		return err
	}
	hash := ScopeHash(slugs)

	body = SetScopeHashInMarker(body, hash)
	body = FmSetSources(body, slugs)
	body = FmSetScalar(body, "last_generated", time.Now().UTC().Format(time.RFC3339))
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

	// bump ingest counter
	cntPath := filepath.Join(r.RepoRoot, ".awiki", "ingest-count")
	cur := 0
	if b, e := os.ReadFile(cntPath); e == nil {
		cur, _ = strconv.Atoi(string(b))
	}
	_ = os.MkdirAll(filepath.Dir(cntPath), 0o755)
	_ = os.WriteFile(cntPath, []byte(strconv.Itoa(cur+1)), 0o644)

	srcCount := 0
	for _, s := range slugs {
		if s != "" {
			srcCount++
		}
	}
	fmt.Fprintf(stderr, "SYNTH-FINALIZE|target=%s|sources=%d|scope_hash=%s\n", target, srcCount, hash)
	return nil
}
```

Tests:
- Missing target → exit 1.
- Marker integrity failure → exit 5.
- Lint failure → exit 6.
- Happy path: scope_hash updated in BEGIN line; sources rewritten;
  last_generated set to RFC3339 UTC; ingest-count bumped.
- Post-hook gated/failed paths.

Smoke test mirrors regen smoke.

Commit: `feat: port synth finalize verb`.

---

## Task 5: new verb

**Files:**
- Create: `internal/synth/new.go`, `internal/synth/new_test.go`
- Modify: `internal/cli/synth.go` (wire `new` case)
- Modify: `scripts/synth.sh` (add `new` to GO_VERBS)

This is the largest verb. Core logic:

```go
package synth

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type NewOptions struct {
	Plugin         string
	Topic          string
	ScopeKind      string // "tag"|"slugs"|"query"
	ScopeValue     string
	ExcludeTags    string
	MinLastUpdated string
	Types          string
	AllowPrivate   bool
}

func (r *Runner) New(ctx context.Context, opts NewOptions, stdout, stderr io.Writer) error {
	if !synthSlugRegexpMatch(opts.Topic) {
		return &ExitError{Code: 1, Msg: "invalid topic-slug: " + opts.Topic}
	}
	p, err := LoadPlugin(filepath.Join(r.PluginDir, opts.Plugin+".md"))
	if err != nil {
		return &ExitError{Code: 1, Msg: "plugin load failed: " + opts.Plugin}
	}

	target := filepath.Join(r.SynthDir(), opts.Topic+"-"+opts.Plugin+".md")
	targetPrivate := isPrivatePath(target)

	scope := Scope{
		Kind:           opts.ScopeKind,
		ExcludeTags:    splitCommaList(opts.ExcludeTags),
		MinLastUpdated: opts.MinLastUpdated,
		Types:          splitCommaList(opts.Types),
		AllowPrivate:   true,
	}
	switch opts.ScopeKind {
	case "tag":
		scope.Tag = opts.ScopeValue
	case "slugs":
		scope.Slugs = splitCommaList(opts.ScopeValue)
	case "query":
		scope.Query = opts.ScopeValue
	default:
		return &ExitError{Code: 1, Msg: "must pass --tag, --slugs or --query"}
	}

	// First pass: privacy detection.
	rawSlugs, err := r.resolveSlugs(ctx, scope, target)
	if err != nil {
		return err
	}
	hasPrivate := false
	for _, s := range rawSlugs {
		tags := readPageTags(r.slugToPath(s))
		if hasTag(tags, "private") {
			hasPrivate = true
			break
		}
	}
	if hasPrivate && !targetPrivate && !opts.AllowPrivate {
		return &ExitError{Code: 2,
			Msg: "private source in scope; either tag the synthesis page private and place under content/private/, or pass --allow-private"}
	}

	// Second pass: canonical resolution.
	scope.AllowPrivate = targetPrivate || opts.AllowPrivate
	slugs, err := r.resolveSlugs(ctx, scope, target)
	if err != nil {
		return err
	}

	if p.MinSources > 0 && len(slugs) < p.MinSources {
		return &ExitError{Code: 2,
			Msg: fmt.Sprintf("scope resolves to %d sources; plugin requires min_sources=%d", len(slugs), p.MinSources)}
	}
	if p.MaxSources > 0 && len(slugs) > p.MaxSources {
		return &ExitError{Code: 2,
			Msg: fmt.Sprintf("scope resolves to %d sources; plugin allows max_sources=%d", len(slugs), p.MaxSources)}
	}

	if _, err := os.Stat(target); err == nil {
		return &ExitError{Code: 3, Msg: "target already exists: " + target + " — use 'synth.sh regen' to refresh"}
	}

	hash := ScopeHash(slugs)

	scopeBlock := buildScopeBlock(opts.ScopeKind, opts.ScopeValue,
		opts.ExcludeTags, opts.MinLastUpdated, opts.Types)

	scaffold := WriteScaffold(ScaffoldInput{
		Topic:       opts.Topic,
		Plugin:      opts.Plugin,
		Today:       r.today(),
		ScopeBlock:  scopeBlock,
		SourcesYAML: "sources: []\n",
		ScopeHash:   hash,
	})
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(target, scaffold, 0o644); err != nil {
		return err
	}

	scopeDesc := newScopeDescription(opts.ScopeKind, opts.ScopeValue, len(slugs))
	if err := r.EmitPrompt(stdout, p, slugs, scopeDesc, ""); err != nil {
		return err
	}

	r.LogAppend(ctx, "synth-scaffold", opts.Plugin+" "+opts.Topic)
	if hasPrivate && opts.AllowPrivate {
		r.LogAppend(ctx, "synth-declassify", opts.Topic+" sources="+strconv.Itoa(len(slugs)))
	}

	fmt.Fprintf(stderr, "SYNTH-NEW|target=%s|plugin=%s|sources=%d|scope_hash=%s\n",
		target, opts.Plugin, len(slugs), hash)
	return nil
}

func buildScopeBlock(kind, val, excludeTags, minLU, types string) string {
	var b strings.Builder
	b.WriteString("scope:\n")
	switch kind {
	case "tag":
		b.WriteString("  tag: " + val + "\n")
	case "slugs":
		b.WriteString("  slugs: [" + val + "]\n")
	case "query":
		b.WriteString("  query: \"" + val + "\"\n")
	}
	if excludeTags != "" {
		b.WriteString("  exclude_tags: [" + excludeTags + "]\n")
	}
	if minLU != "" {
		b.WriteString("  min_last_updated: " + minLU + "\n")
	}
	if types != "" {
		b.WriteString("  types: [" + types + "]\n")
	}
	return b.String()
}

func newScopeDescription(kind, val string, n int) string {
	switch kind {
	case "tag":
		return "pages tagged '" + val + "'"
	case "slugs":
		return fmt.Sprintf("explicit slug list (%d pages)", n)
	case "query":
		return "qmd query: " + val
	}
	return ""
}
```

Tests (`new_test.go`):
- Happy path: file written, scaffold byte-equivalent to bash output
  for the same inputs (golden fixture using a test plugin).
- Missing plugin → exit 1.
- Refuses to overwrite → exit 3.
- Below min_sources → exit 2.
- Above max_sources → exit 2.
- Private leakage without `--allow-private` → exit 2.
- `--allow-private` allows private sources; declassify log emitted.

CLI wire: implement `runSynthNew` to flag-parse `--tag=`, `--slugs=`,
`--query=`, `--exclude-tags=`, `--min-last-updated=`, `--types=`,
`--allow-private`. Two positionals: plugin, topic. Mutual exclusion
on the three scope flags.

`scripts/synth.sh`: add `new` to `AWIKI_SYNTH_GO_VERBS`.

Smoke (use a small plugin in `synthesis-plugins/`):

```bash
./bin/awiki synth new briefing test-topic --slugs=foo,bar
diff against AWIKI_SYNTH_LEGACY=1 bash version.
```

Commit: `feat: port synth new verb`.

---

## Final task

After all four verbs ship:
- `go test -race ./...` clean.
- `just test` 774/774 pass.
- Smoke each verb against a real synth page, compare bash vs Go.
- Merge `feat/synth-remaining-verbs` to main with `--no-ff` and the
  same template as the prior synth merge.

## Self-review

- **Spec coverage.** Every verb in the spec maps to a task: helpers
  Task 1, regen Task 2, accept-stage Task 3, finalize Task 4, new
  Task 5.
- **Placeholder scan.** Several `// helpers (define if not already
  present elsewhere)` comments in Task 2 reference small predicates
  (`readScalar`, `extractFrontmatter`, `isPrivatePath`,
  `readPageTags`, `hasTag`, `scopeDescription`,
  `r.resolveSlugs`). The implementer adds them where they don't
  already exist; most live in `prompt.go` (`readFrontmatterAndBody`)
  or `resolve.go` (slug-to-path lookup). Treat each as TDD: if a
  helper is missing, add it with a unit test in the same file as
  the verb it supports.
- **Type consistency.** `ExitError`, `ScaffoldInput`, `Scope`,
  `Plugin`, `Runner.{Git,Lint,PostHook,Qmd}` cross-reference
  consistently.
- **Out of scope (correctly).** Cleanup slice; deletion of
  `scripts/synth.sh` and python helpers; MCP synth handlers.
