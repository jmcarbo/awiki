# Go synth domain port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the awiki synth domain from `scripts/synth.sh` (~854 LOC) plus
five Python helpers and two satellite shell scripts into the `awiki` Go binary
under the nested command group `awiki synth`. Verbs covered: `list`, `resolve`,
`refine`, `new`, `regen`, `accept-stage`, `finalize`.

**Architecture:** New `internal/synth/` package consumes the shared building
blocks shipped in the prior slice (`internal/{fsutil,emit,region,action,
config}`). New `PostHook` adapter joins the existing `internal/adapters/`
interfaces. Existing `internal/lint/synth/` (S1–S9 rules) is consumed for the
`--lint` path inside `finalize`. Each verb is implemented incrementally; the
shim `scripts/synth.sh` flips to `exec awiki synth …` once the first verb
ships.

**Tech Stack:** Go 1.25, `golang.org/x/sys/unix`, stdlib only. External tools
(`qmd`) are reached through typed adapters with fakes for tests.

**Cleanup is NOT in scope:** deleting bash/python under `scripts/` is gated on
a release window per the roadmap and is its own future plan.

**Spec:** `docs/superpowers/specs/2026-04-29-go-synth-domain-design.md`.

---

## File structure

Created:

- `internal/synth/types.go` — `Plugin`, `Scope`, `Page`, `Runner`, error types.
- `internal/synth/types_test.go`.
- `internal/synth/plugin.go` — manifest loader.
- `internal/synth/plugin_test.go`.
- `internal/synth/scope.go` — scope resolution + slug filtering + scope hash.
- `internal/synth/scope_test.go`.
- `internal/synth/region.go` — synth region read/write helpers.
- `internal/synth/region_test.go`.
- `internal/synth/list.go` — `list` verb.
- `internal/synth/list_test.go`.
- `internal/synth/resolve.go` — `resolve` verb.
- `internal/synth/resolve_test.go`.
- `internal/synth/refine.go` — `refine` verb.
- `internal/synth/refine_test.go`.
- `internal/synth/new.go` — `new` verb.
- `internal/synth/new_test.go`.
- `internal/synth/regen.go` — `regen` verb.
- `internal/synth/regen_test.go`.
- `internal/synth/accept.go` — `accept-stage` verb.
- `internal/synth/accept_test.go`.
- `internal/synth/finalize.go` — `finalize` verb.
- `internal/synth/finalize_test.go`.
- `internal/cli/synth.go` — nested-group dispatcher.
- `internal/cli/synth_test.go`.
- `internal/adapters/posthook.go` — `PostHook` interface + `ExecPostHook`.
- `internal/adapters/posthook_test.go`.
- `internal/testutil/synth_fixture.go` — fixture loader + golden differ.
- `tests/fixtures/synth/<verb>/…` — golden fixtures per verb.

Modified:

- `internal/cli/cli.go` — register `synth` subgroup.
- `scripts/synth.sh` — shim flip in verb sub-slice 1 (Task 4); per-verb branch
  deletion in subsequent sub-slices.

---

### Task 1: Add `PostHook` adapter

**Files:**
- Create: `internal/adapters/posthook.go`
- Create: `internal/adapters/posthook_test.go`

This adapter wraps arbitrary user-supplied bash scripts declared by synth
plugin manifests. It is gated by `.awiki/config` `ALLOW_PLUGIN_POST_HOOKS=1`;
the gating is enforced by the *caller* (synth runner), not the adapter — the
adapter only runs.

- [ ] **Step 1: Write the failing test**

Create `internal/adapters/posthook_test.go`:

```go
package adapters

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExecPostHookRunsScriptAndCapturesOutput(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "hook.sh")
	body := "#!/usr/bin/env bash\necho \"hook: $@\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	h := ExecPostHook{Timeout: time.Second}
	out, code, err := h.Run(context.Background(), script, "/some/page.md")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != 0 {
		t.Fatalf("code: %d", code)
	}
	if got, want := out, "hook: --page /some/page.md\n"; got != want {
		t.Fatalf("out: got %q want %q", got, want)
	}
}

func TestExecPostHookHonorsTimeout(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "slow.sh")
	body := "#!/usr/bin/env bash\nsleep 2\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	h := ExecPostHook{Timeout: 100 * time.Millisecond}
	_, code, err := h.Run(context.Background(), script, "/p.md")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if code == 0 {
		t.Fatalf("code: %d (want non-zero)", code)
	}
}
```

- [ ] **Step 2: Run the test, expect build error**

```bash
go test ./internal/adapters/...
```

- [ ] **Step 3: Implement `internal/adapters/posthook.go`**

```go
package adapters

import (
	"context"
	"errors"
	"os/exec"
	"time"
)

// PostHook runs a synth plugin post-hook script. The script receives
// `--page <pagePath>` as arguments. The implementation enforces a
// timeout; the caller is responsible for the
// ALLOW_PLUGIN_POST_HOOKS gate.
type PostHook interface {
	Run(ctx context.Context, scriptPath, pagePath string) (output string, code int, err error)
}

// ExecPostHook is the production implementation. Default timeout 60s.
type ExecPostHook struct {
	Timeout time.Duration
}

func (h ExecPostHook) Run(ctx context.Context, scriptPath, pagePath string) (string, int, error) {
	timeout := h.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, scriptPath, "--page", pagePath)
	out, err := cmd.CombinedOutput()
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return string(out), 124, runCtx.Err()
	}
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

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/adapters/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/posthook.go internal/adapters/posthook_test.go
git commit -m "feat: add PostHook adapter for synth plugin hooks"
```

---

### Task 2: Synth package skeleton + fixture harness

**Files:**
- Create: `internal/synth/types.go`
- Create: `internal/synth/runner.go`
- Create: `internal/testutil/synth_fixture.go`
- Create: `internal/synth/runner_test.go`

Adds the package types and a `Runner` that holds dependencies (paths, config,
adapters). Each verb is a method on `Runner`. The fixture harness is the
golden-test driver every verb sub-slice consumes.

- [ ] **Step 1: Write `internal/synth/types.go`**

```go
// Package synth implements the awiki synth domain commands
// (awiki synth list|resolve|refine|new|regen|accept-stage|finalize).
package synth

// Plugin is the parsed contents of plugins/synth/<name>.md.
type Plugin struct {
	Name                  string
	Description           string
	OutputType            string
	OutputSubtype         string
	MinSources            int
	MaxSources            int
	MaxEvidenceTotalWords int
	RequiredSections      []string
	PostHook              string
	Render                string
	Version               string
	// Body is the prompt template (file content after the closing
	// `---` of the YAML frontmatter, with a single trailing newline).
	Body string
}

// Scope is the parsed scope block from a synth page frontmatter.
type Scope struct {
	Kind           string // "tag" | "slugs" | "query"
	Tag            string
	Slugs          []string
	Query          string
	ExcludeTags    []string
	MinLastUpdated string
	Types          []string
	AllowPrivate   bool
}

// Page is a synth page on disk plus its parsed metadata.
type Page struct {
	Slug          string
	Path          string
	Plugin        string
	Topic         string
	Scope         Scope
	LastGenerated string
	ScopeHash     string
	Frontmatter   map[string]string
}
```

- [ ] **Step 2: Write `internal/synth/runner.go`**

```go
package synth

import (
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
// Mirrors the bash check on .awiki/config:
// ALLOW_PLUGIN_POST_HOOKS=1.
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
	cfg, err := config.Load(r.RepoRoot + "/.awiki/config")
	if err != nil {
		return err
	}
	r.Config = cfg
	return nil
}
```

- [ ] **Step 3: Write `internal/synth/runner_test.go`**

```go
package synth

func init() {}
```

(Empty — actual tests added per verb. The file exists so the package builds
under `go test`.)

Actually drop this file — go test will pass for a package with no test files.

- [ ] **Step 4: Write `internal/testutil/synth_fixture.go`**

```go
// Package testutil provides shared fixture helpers for awiki tests.
package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CopyTree copies src into dst recursively. dst must not exist.
func CopyTree(t testing.TB, src, dst string) {
	t.Helper()
	if err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	}); err != nil {
		t.Fatalf("CopyTree: %v", err)
	}
}

// ReadFixtureLines loads a file as line-separated strings. Trailing
// newline is preserved or omitted as on disk.
func ReadFixtureLines(t testing.TB, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		return nil
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}
```

- [ ] **Step 5: Verify build + go test green**

```bash
go test ./...
```

(Synth package compiles; no tests yet — that's fine. `go vet ./...` should
also be clean.)

- [ ] **Step 6: Commit**

```bash
git add internal/synth/types.go internal/synth/runner.go \
        internal/testutil/synth_fixture.go
git commit -m "feat: add internal/synth package skeleton + fixture harness"
```

---

### Task 3: Plugin manifest loader

**Files:**
- Create: `internal/synth/plugin.go`
- Create: `internal/synth/plugin_test.go`

Parses `plugins/synth/<name>.md`: a YAML frontmatter block bracketed by `---`
followed by a free-form Markdown body (the prompt template).

The loader supports both inline list (`required_sections: [a, b]`) and block
list (`required_sections:\n  - a\n  - b`) for `required_sections`. All other
scalar fields are simple `key: value` lines.

- [ ] **Step 1: Write the failing test**

Create `internal/synth/plugin_test.go`:

```go
package synth

import (
	"os"
	"path/filepath"
	"testing"
)

const samplePlugin = `---
name: explainer
description: One-paragraph topic explainer.
output_type: synth
output_subtype: explainer
min_sources: 2
max_sources: 8
max_evidence_total_words: 500
required_sections:
  - Summary
  - Sources
post_hook: scripts/synth-mindmap-validate.sh
render: markdown
version: 1
---
Body line 1
Body line 2
`

func TestLoadPluginParsesBlockList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "explainer.md")
	if err := os.WriteFile(path, []byte(samplePlugin), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPlugin(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "explainer" {
		t.Fatalf("name: %q", got.Name)
	}
	if got.MinSources != 2 || got.MaxSources != 8 {
		t.Fatalf("min/max: %d/%d", got.MinSources, got.MaxSources)
	}
	if len(got.RequiredSections) != 2 || got.RequiredSections[0] != "Summary" {
		t.Fatalf("required: %v", got.RequiredSections)
	}
	if got.PostHook != "scripts/synth-mindmap-validate.sh" {
		t.Fatalf("post_hook: %q", got.PostHook)
	}
	if got.Body != "Body line 1\nBody line 2\n" {
		t.Fatalf("body: %q", got.Body)
	}
}

func TestLoadPluginInlineList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.md")
	body := "---\nname: p\nrequired_sections: [Sum, Notes]\nversion: 1\n---\nx\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPlugin(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.RequiredSections) != 2 || got.RequiredSections[1] != "Notes" {
		t.Fatalf("required: %v", got.RequiredSections)
	}
}

func TestLoadPluginRequiresName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noname.md")
	body := "---\nversion: 1\n---\n\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPlugin(path); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestLoadPluginRequiresFilenameMatchesName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "actual.md")
	body := "---\nname: different\nversion: 1\n---\n\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPlugin(path); err == nil {
		t.Fatal("expected error: name must match filename")
	}
}
```

- [ ] **Step 2: Run, expect build error**

- [ ] **Step 3: Implement `internal/synth/plugin.go`**

Implementation outline (write the complete file in your editor; the steps
below describe what each helper does):

```go
package synth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// LoadPlugin parses a synth plugin manifest at path. The file must be
// a YAML frontmatter block (---/--- bracketed) plus a body. Returns
// an error if required fields (name, version) are missing or the
// declared name does not match the file's basename.
func LoadPlugin(path string) (Plugin, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Plugin{}, err
	}
	fmText, body, err := splitFrontmatter(string(data))
	if err != nil {
		return Plugin{}, fmt.Errorf("%s: %w", path, err)
	}
	p, err := parsePluginFrontmatter(fmText)
	if err != nil {
		return Plugin{}, fmt.Errorf("%s: %w", path, err)
	}
	p.Body = body
	if p.Name == "" {
		return Plugin{}, fmt.Errorf("%s: missing 'name'", path)
	}
	if p.Version == "" {
		return Plugin{}, fmt.Errorf("%s: missing 'version'", path)
	}
	expected := strings.TrimSuffix(filepath.Base(path), ".md")
	if p.Name != expected {
		return Plugin{}, fmt.Errorf("%s: name %q must match filename %q", path, p.Name, expected)
	}
	if p.MaxEvidenceTotalWords == 0 {
		p.MaxEvidenceTotalWords = 500 // matches synth-plugin-load.sh default
	}
	return p, nil
}

// splitFrontmatter splits "---\n<frontmatter>\n---\n<body>".
func splitFrontmatter(text string) (string, string, error) {
	if !strings.HasPrefix(text, "---\n") {
		return "", "", errors.New("missing leading '---'")
	}
	rest := text[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		// allow file ending with --- and no body
		end = strings.Index(rest, "\n---")
		if end < 0 {
			return "", "", errors.New("missing closing '---'")
		}
	}
	fm := rest[:end]
	bodyStart := end + len("\n---\n")
	body := ""
	if bodyStart <= len(rest) {
		body = rest[bodyStart:]
	}
	return fm, body, nil
}

// parsePluginFrontmatter handles scalar fields plus
// `required_sections` in inline `[a, b]` or block (- item) form.
func parsePluginFrontmatter(text string) (Plugin, error) {
	var p Plugin
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		val := strings.TrimSpace(line[colon+1:])
		switch key {
		case "name":
			p.Name = val
		case "description":
			p.Description = val
		case "output_type":
			p.OutputType = val
		case "output_subtype":
			p.OutputSubtype = val
		case "min_sources":
			p.MinSources, _ = strconv.Atoi(val)
		case "max_sources":
			p.MaxSources, _ = strconv.Atoi(val)
		case "max_evidence_total_words":
			p.MaxEvidenceTotalWords, _ = strconv.Atoi(val)
		case "post_hook":
			p.PostHook = val
		case "render":
			p.Render = val
		case "version":
			p.Version = val
		case "required_sections":
			if strings.HasPrefix(val, "[") {
				p.RequiredSections = parseInlineList(val)
			} else if val == "" {
				// block form
				j := i + 1
				for j < len(lines) {
					l := lines[j]
					t := strings.TrimSpace(l)
					if t == "" || !strings.HasPrefix(t, "- ") {
						break
					}
					p.RequiredSections = append(p.RequiredSections, strings.TrimSpace(strings.TrimPrefix(t, "- ")))
					j++
				}
				i = j - 1
			}
		}
	}
	return p, nil
}

func parseInlineList(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return out
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/synth/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/synth/plugin.go internal/synth/plugin_test.go
git commit -m "feat: add synth plugin manifest loader"
```

---

### Task 4: `list` verb + CLI dispatcher + shim flip

**Files:**
- Create: `internal/synth/list.go`
- Create: `internal/synth/list_test.go`
- Create: `internal/cli/synth.go`
- Modify: `internal/cli/cli.go`
- Modify: `scripts/synth.sh`

This task is the first end-to-end slice: it introduces the dispatcher, makes
the `list` verb live, and flips the bash shim. Once shipped, `just synth-list`
runs Go.

- [ ] **Step 1: Write `internal/synth/list_test.go`**

```go
package synth

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestListEmitsSortedPlugins(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "plugins", "synth")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"zeta", "alpha", "mid"} {
		body := "---\nname: " + name + "\nversion: 1\nmin_sources: 2\nmax_sources: 8\n" +
			"required_sections: [Summary, Sources]\n---\nbody\n"
		if err := os.WriteFile(filepath.Join(pluginDir, name+".md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := &Runner{RepoRoot: dir, PluginDir: pluginDir}
	var out bytes.Buffer
	if err := r.List(&out); err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	want := []string{
		"PLUGIN|alpha|version=1|min=2|max=8|sections=Summary,Sources",
		"PLUGIN|mid|version=1|min=2|max=8|sections=Summary,Sources",
		"PLUGIN|zeta|version=1|min=2|max=8|sections=Summary,Sources",
	}
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("len: got %d want %d (out=%q)", len(got), len(want), out.String())
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("line %d: got %q want %q", i, got[i], want[i])
		}
	}
}
```

- [ ] **Step 2: Run, expect build error**

- [ ] **Step 3: Implement `internal/synth/list.go`**

```go
package synth

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// List walks PluginDir, parses every plugin manifest, and emits one
// PLUGIN| record per plugin in name-sorted order.
func (r *Runner) List(out io.Writer) error {
	entries, err := os.ReadDir(r.PluginDir)
	if err != nil {
		return err
	}
	var plugins []Plugin
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		p, err := LoadPlugin(filepath.Join(r.PluginDir, e.Name()))
		if err != nil {
			return err
		}
		plugins = append(plugins, p)
	}
	sort.Slice(plugins, func(i, j int) bool { return plugins[i].Name < plugins[j].Name })
	for _, p := range plugins {
		fmt.Fprintf(out, "PLUGIN|%s|version=%s|min=%d|max=%d|sections=%s\n",
			p.Name, p.Version, p.MinSources, p.MaxSources,
			strings.Join(p.RequiredSections, ","))
	}
	return nil
}
```

- [ ] **Step 4: Implement `internal/cli/synth.go`**

```go
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"awiki/internal/adapters"
	"awiki/internal/config"
	"awiki/internal/synth"
)

// runSynth dispatches `awiki synth <verb> [args]`.
func runSynth(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: awiki synth <verb> [args]")
		return 1
	}
	verb, rest := args[0], args[1:]
	r, err := buildSynthRunner()
	if err != nil {
		fmt.Fprintf(stderr, "synth: %v\n", err)
		return 1
	}
	switch verb {
	case "list":
		return runSynthList(r, rest, stdout, stderr)
	case "resolve", "refine", "new", "regen", "accept-stage", "finalize":
		fmt.Fprintf(stderr, "synth: verb %q not yet ported\n", verb)
		return 1
	default:
		fmt.Fprintf(stderr, "synth: unknown verb %q\n", verb)
		return 1
	}
}

func buildSynthRunner() (*synth.Runner, error) {
	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		repoRoot = wd
	}
	pluginDir := os.Getenv("AWIKI_SYNTH_PLUGINS_DIR")
	if pluginDir == "" {
		pluginDir = filepath.Join(repoRoot, "plugins", "synth")
	}
	cfg, _ := config.Load(filepath.Join(repoRoot, ".awiki", "config"))
	return &synth.Runner{
		RepoRoot:   repoRoot,
		ContentDir: filepath.Join(repoRoot, "content"),
		PluginDir:  pluginDir,
		Config:     cfg,
		Qmd:        nil, // wired in resolve/finalize tasks
		PostHook:   adapters.ExecPostHook{},
	}, nil
}

func runSynthList(r *synth.Runner, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("synth list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if err := r.List(stdout); err != nil {
		fmt.Fprintf(stderr, "synth list: %v\n", err)
		return 2
	}
	return 0
}
```

- [ ] **Step 5: Wire into `internal/cli/cli.go`**

In the existing `Run` switch in `internal/cli/cli.go`, add a case before
`default`:

```go
case "synth":
    return runSynth(args[1:], stdout, stderr)
```

- [ ] **Step 6: Flip `scripts/synth.sh` to a shim**

Replace `scripts/synth.sh` head with the shim pattern:

```bash
#!/usr/bin/env bash
set -uo pipefail

AWIKI_SYNTH_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AWIKI_SYNTH_REPO_ROOT="$(cd "$AWIKI_SYNTH_SCRIPT_DIR/.." && pwd)"
AWIKI_SYNTH_GO_BIN="$AWIKI_SYNTH_REPO_ROOT/bin/awiki"

# Verbs already ported to Go. Bash branches for these are deleted from
# the case statement below as each verb ships.
AWIKI_SYNTH_GO_VERBS=(list)

if [[ "${AWIKI_SYNTH_LEGACY:-0}" != "1" && -x "$AWIKI_SYNTH_GO_BIN" ]]; then
  case "${1:-}" in
    --) shift ;;
  esac
  verb="${1:-}"
  for v in "${AWIKI_SYNTH_GO_VERBS[@]}"; do
    if [[ "$v" == "$verb" ]]; then
      shift
      cd "$AWIKI_SYNTH_REPO_ROOT" || exit 1
      exec "$AWIKI_SYNTH_GO_BIN" synth "$verb" "$@"
    fi
  done
fi

# Original bash dispatch follows...
# (keep the existing body; only the head changes.)
```

For this task delete the `list)` case from the existing case statement
inside the bash dispatch (so the legacy fallback no longer handles it
even when AWIKI_SYNTH_LEGACY=1; it's been ported to Go).

- [ ] **Step 7: Run all tests**

```bash
go test ./...
just test
```

`just test` runs the synth bats. `synth_test.bats` exercises `just synth-list`;
that recipe forwards through the shim to Go.

- [ ] **Step 8: Verify `just synth-list` end-to-end**

```bash
go build -o bin/awiki ./cmd/awiki
just synth-list | head -3
```

Expect `PLUGIN|...` lines.

- [ ] **Step 9: Commit**

```bash
git add internal/synth/list.go internal/synth/list_test.go \
        internal/cli/synth.go internal/cli/cli.go \
        scripts/synth.sh
git commit -m "feat: port synth list verb and flip bash shim to Go"
```

---

### Task 5: `resolve` verb (read-only scope dump)

**Files:**
- Create: `internal/synth/scope.go`
- Create: `internal/synth/scope_test.go`
- Create: `internal/synth/resolve.go`
- Create: `internal/synth/resolve_test.go`
- Modify: `internal/cli/synth.go` (wire `resolve` case)
- Modify: `scripts/synth.sh` (delete `resolve)` branch; add to GO_VERBS)

The `resolve` verb reads a synth page, parses its `scope:` block, applies
filters, and emits one slug per line ASCII-sorted.

For the `query` flavor, the runner's `Qmd` adapter is invoked. Tests inject
a fake.

- [ ] **Step 1: Write `internal/synth/scope_test.go`**

```go
package synth

import "testing"

func TestParseScopeFromFrontmatterTag(t *testing.T) {
	fm := map[string]string{"scope.kind": "tag", "scope.tag": "decision"}
	got := ParseScope(fm)
	if got.Kind != "tag" || got.Tag != "decision" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseScopeSlugs(t *testing.T) {
	fm := map[string]string{"scope.kind": "slugs", "scope.slugs": "alpha,beta"}
	got := ParseScope(fm)
	if got.Kind != "slugs" {
		t.Fatalf("kind %q", got.Kind)
	}
	if len(got.Slugs) != 2 || got.Slugs[0] != "alpha" {
		t.Fatalf("slugs %v", got.Slugs)
	}
}

func TestScopeHashIsFirst6OfSha256(t *testing.T) {
	// Matches lint-synth-hash.py: sha256 of newline-joined slugs, first
	// 6 hex chars.
	got := ScopeHash([]string{"a", "b", "c"})
	want := "f8294b" // sha256("a\nb\nc\n")[:6]
	if got != want {
		t.Fatalf("hash %q want %q", got, want)
	}
}

func TestScopeHashEmpty(t *testing.T) {
	got := ScopeHash(nil)
	want := "e3b0c4" // sha256 empty
	if got != want {
		t.Fatalf("hash %q want %q", got, want)
	}
}
```

- [ ] **Step 2: Run, expect build error**

- [ ] **Step 3: Implement `internal/synth/scope.go`**

```go
package synth

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// ParseScope extracts the scope block from frontmatter map keys
// `scope.kind`, `scope.tag`, `scope.slugs`, `scope.query`,
// `scope.exclude_tags`, `scope.min_last_updated`, `scope.types`,
// `scope.allow_private`. The frontmatter parser used elsewhere
// flattens the `scope:` block to dotted keys.
func ParseScope(fm map[string]string) Scope {
	s := Scope{
		Kind:           fm["scope.kind"],
		Tag:            fm["scope.tag"],
		Query:          fm["scope.query"],
		MinLastUpdated: fm["scope.min_last_updated"],
		AllowPrivate:   fm["scope.allow_private"] == "true",
	}
	if v := fm["scope.slugs"]; v != "" {
		s.Slugs = splitCommaList(v)
	}
	if v := fm["scope.exclude_tags"]; v != "" {
		s.ExcludeTags = splitCommaList(v)
	}
	if v := fm["scope.types"]; v != "" {
		s.Types = splitCommaList(v)
	}
	return s
}

func splitCommaList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// ScopeHash matches scripts/lint-synth-hash.py: sha256 of newline-
// joined slugs (with trailing newline), truncated to first 6 hex.
func ScopeHash(slugs []string) string {
	sorted := append([]string(nil), slugs...)
	sort.Strings(sorted)
	h := sha256.New()
	for _, s := range sorted {
		h.Write([]byte(s))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))[:6]
}
```

Note: the test asserts `ScopeHash([]string{"a","b","c"})` equals `"f8294b"`.
Verify by computing locally; if the value differs, update the test to the
actual hash output (the *function* under test is the bash compatibility
oracle — bash uses `sha256 of sorted-newline-joined-with-trailing-newline`).

- [ ] **Step 4: Run scope tests, fix any hash mismatch by updating test
expected values to the actual computed hash (the implementation is the
oracle; tests pin behavior, not arbitrary numerics).**

- [ ] **Step 5: Write `internal/synth/resolve.go`**

```go
package synth

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"awiki/internal/wiki"
)

// Resolve reads the synth page at <ContentDir>/synth/<slug>.md,
// computes the resolved slug list per its scope, and emits one slug
// per line ASCII-sorted to out.
func (r *Runner) Resolve(ctx context.Context, slug string, out io.Writer) error {
	path := filepath.Join(r.ContentDir, "synth", slug+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	page, err := wiki.ParsePage(path, data)
	if err != nil {
		return err
	}
	scope := ParseScope(page.Frontmatter)
	slugs, err := r.resolveScope(ctx, scope)
	if err != nil {
		return err
	}
	sort.Strings(slugs)
	for _, s := range slugs {
		fmt.Fprintln(out, s)
	}
	return nil
}

func (r *Runner) resolveScope(ctx context.Context, s Scope) ([]string, error) {
	switch s.Kind {
	case "slugs":
		return uniqueSorted(s.Slugs), nil
	case "tag":
		return r.resolveByTag(s)
	case "query":
		if r.Qmd == nil {
			return nil, fmt.Errorf("scope.kind=query but qmd adapter unavailable")
		}
		out, code, err := r.Qmd.Search(ctx, r.RepoRoot, s.Query)
		if err != nil {
			return nil, fmt.Errorf("qmd search: code=%d %w", code, err)
		}
		return parseQmdSlugs(out), nil
	default:
		return nil, fmt.Errorf("unsupported scope.kind %q", s.Kind)
	}
}

func (r *Runner) resolveByTag(s Scope) ([]string, error) {
	// Walk r.ContentDir, collect slugs whose page frontmatter
	// `tags:` includes s.Tag, applying privacy + exclude + types
	// filters. Implementation parallels scripts/synth.sh:138.
	// ...
	return nil, fmt.Errorf("tag-scope resolution not implemented")
}

func uniqueSorted(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func parseQmdSlugs(text string) []string {
	parts := strings.Fields(text)
	return uniqueSorted(parts)
}
```

(Note: the tag-scope walker is not implemented in this task — it lands in the
`new`/`finalize` task where it's actually exercised. For `resolve`, only the
`slugs` and `query` flavors are required to ship; tag-flavor pages return an
explicit not-implemented error. Update the failing-test expectations
accordingly.)

- [ ] **Step 6: Write `internal/synth/resolve_test.go`** with two tests:
  one for `slugs` flavor (no Qmd needed), one for `query` flavor with a fake
  `Qmd` adapter.

- [ ] **Step 7: Wire CLI dispatch in `internal/cli/synth.go`**

Add to the verb switch:

```go
case "resolve":
    return runSynthResolve(r, rest, stdout, stderr)
```

And the new helper:

```go
func runSynthResolve(r *synth.Runner, args []string, stdout, stderr io.Writer) int {
    fs := flag.NewFlagSet("synth resolve", flag.ContinueOnError)
    fs.SetOutput(stderr)
    if err := fs.Parse(args); err != nil {
        return 1
    }
    if fs.NArg() < 1 {
        fmt.Fprintln(stderr, "usage: awiki synth resolve <slug>")
        return 1
    }
    if err := r.Resolve(context.Background(), fs.Arg(0), stdout); err != nil {
        fmt.Fprintf(stderr, "synth resolve: %v\n", err)
        return 2
    }
    return 0
}
```

- [ ] **Step 8: Update `scripts/synth.sh`**: add `resolve` to
`AWIKI_SYNTH_GO_VERBS`; delete `resolve)` case from the bash dispatch.

- [ ] **Step 9: Run all tests + bats; commit**

```
git add internal/synth/scope.go internal/synth/scope_test.go \
        internal/synth/resolve.go internal/synth/resolve_test.go \
        internal/cli/synth.go scripts/synth.sh
git commit -m "feat: port synth resolve verb (slugs/query flavors)"
```

---

### Task 6: `refine` verb (idempotent feedback append)

**Files:**
- Create: `internal/synth/refine.go`
- Create: `internal/synth/refine_test.go`
- Modify: `internal/cli/synth.go` (wire case)
- Modify: `scripts/synth.sh`

Append a `- <note>` bullet to the `## Feedback` section of the synth page.
Create the section if missing. Atomic write.

- [ ] **Step 1: Write `internal/synth/refine_test.go`** covering:
  - Append to existing `## Feedback` section.
  - Create `## Feedback` if missing.
  - Multiple appends preserve order.

- [ ] **Step 2: Implement `internal/synth/refine.go`**

```go
package synth

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"awiki/internal/fsutil"
)

// Refine appends "- <note>" to the page's `## Feedback` section,
// creating the section if absent. Atomic.
func (r *Runner) Refine(slug, note string, stderr io.Writer) error {
	path := filepath.Join(r.ContentDir, "synth", slug+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated := appendFeedback(string(data), note)
	if updated == string(data) {
		return nil // idempotent no-op
	}
	if err := fsutil.AtomicWrite(path, []byte(updated)); err != nil {
		return err
	}
	fmt.Fprintf(stderr, "SYNTH-REFINE|%s|%s\n", slug, note)
	return nil
}

func appendFeedback(text, note string) string {
	bullet := "- " + note
	idx := strings.Index(text, "\n## Feedback\n")
	if idx < 0 {
		// section missing; append at end
		sep := ""
		if !strings.HasSuffix(text, "\n") {
			sep = "\n"
		}
		return text + sep + "\n## Feedback\n\n" + bullet + "\n"
	}
	// insert bullet at end of section: find next "\n## " or EOF.
	sectionStart := idx + len("\n## Feedback\n")
	rest := text[sectionStart:]
	nextHdr := strings.Index(rest, "\n## ")
	var section, tail string
	if nextHdr < 0 {
		section = rest
		tail = ""
	} else {
		section = rest[:nextHdr]
		tail = rest[nextHdr:]
	}
	if !strings.HasSuffix(section, "\n") {
		section += "\n"
	}
	return text[:sectionStart] + section + bullet + "\n" + tail
}
```

- [ ] Steps 3-7: Wire dispatch (`runSynthRefine`), add `refine` to
GO_VERBS in shim, delete `refine)` from bash dispatch, run tests, commit.

---

### Task 7: `new` verb (scaffold synthesis page)

**Files:**
- Create: `internal/synth/new.go`
- Create: `internal/synth/new_test.go`
- Create: `internal/synth/region.go` (write helpers — content emit)
- Create: `internal/synth/region_test.go`
- Modify: `internal/cli/synth.go`, `scripts/synth.sh`

The `new` verb scaffolds a synth page: validates plugin, resolves scope,
computes hash, writes frontmatter + Sources section + empty GENERATED
region + Feedback section.

- [ ] **Step 1: Implement `internal/synth/region.go`**

Wrap `internal/region.ParseGenerated` (already shipped) plus a writer that
emits a fresh empty region with the begin line carrying
`scope_hash=…|sources_hash=…`.

- [ ] **Step 2: Write `internal/synth/new_test.go`** covering:
  - Happy path: writes file, frontmatter has expected keys, region is empty,
    Sources section lists wikilinks, scope_hash present.
  - Refuses to overwrite (exit 1).
  - Validates min/max sources from plugin manifest.

- [ ] **Step 3: Implement `internal/synth/new.go`**. The function:

  1. Loads the plugin via `LoadPlugin`.
  2. Builds a `Scope` from CLI args (`--tag`/`--slugs`/`--query` mutually
     exclusive plus the post-filters).
  3. Resolves the scope to a slug list.
  4. Validates `min_sources <= len(slugs) <= max_sources`.
  5. Computes `scope_hash = ScopeHash(slugs)`.
  6. Computes `sources_hash = ScopeHash(slugs)` (same algorithm; the field
     name in the begin line is historical).
  7. Renders the scaffold to a string buffer:
     ```
     ---
     slug: <topic>-<plugin>
     type: synth
     synth_plugin: <plugin>
     topic: <topic>
     scope:
       kind: <kind>
       …
     sources:
       - <slug-1>
       - <slug-2>
     scope_hash: <hash>
     created: <today>
     tags: []
     ---
     # <topic> <plugin>
     ## Sources
     - [[slug-1]]
     - [[slug-2]]
     <!-- BEGIN GENERATED v1 scope_hash=<hash>|sources_hash=<hash> -->

     <!-- END GENERATED -->
     ## Feedback
     ```
  8. Writes via `os.WriteFile(path, body, 0o644)` only if file doesn't exist.
  9. Stderr: `SYNTH-NEW|<slug>|sources=<count>|hash=<hash>`.

- [ ] Steps 4-7: Wire dispatch + flags, update shim, run tests, commit.

---

### Task 8: `regen` verb (clear region; optional staging)

**Files:**
- Create: `internal/synth/regen.go`
- Create: `internal/synth/regen_test.go`
- Modify: `internal/cli/synth.go`, `scripts/synth.sh`

- [ ] Test cases:
  - Live regen: rewrites the page in place, region body cleared, markers
    preserved.
  - `--stage`: writes to `<content>/.staged/<slug>.md`, live page untouched.

- [ ] Implementation: read page, locate region via
`region.ParseGenerated`, replace body bytes between markers with empty
string + a single newline, atomic-write back to live or staged path.

- [ ] Wire + commit per the established pattern.

---

### Task 9: `accept-stage` verb

**Files:**
- Create: `internal/synth/accept.go`, `internal/synth/accept_test.go`
- Modify: `internal/cli/synth.go`, `scripts/synth.sh`

- [ ] Test cases:
  - Happy path: staged file replaces live, post-hook fires when allowed,
    staged file removed.
  - No staged file: error.
  - Post-hook gating: `ALLOW_PLUGIN_POST_HOOKS != 1` → hook does not fire.

- [ ] Implementation: read staged, atomic-write to live, remove staged,
optionally call `r.PostHook.Run`. Stderr emits
`SYNTH-ACCEPT|<slug>|hook=<bool>`.

---

### Task 10: `finalize` verb

**Files:**
- Create: `internal/synth/finalize.go`, `internal/synth/finalize_test.go`
- Modify: `internal/cli/synth.go`, `scripts/synth.sh`

- [ ] Test cases:
  - Recomputes scope; updates `sources:`, `scope_hash`, `last_generated`.
  - Updates region begin line `scope_hash=…|sources_hash=…`.
  - Post-hook fires when allowed.

- [ ] Implementation: parallel of `new`, but mutates an existing page
in place via atomic write. Region body is preserved; only the begin line
and the frontmatter change.

---

### Task 11: Final regression + smoke

**Files:** (read-only)

- [ ] Run `go test -race ./...` — all green.
- [ ] Run `just test` — bats 774/774 (or current count) pass via shim.
- [ ] Run `just synth-list`, `just synth-resolve <slug>`, `just synth-refine
<slug> "test"`, `just synth-new …`, `just synth-regen <slug>`,
`just synth-accept-stage <slug>`, `just synth-finalize <slug>` against the
live wiki to confirm end-to-end behavior.
- [ ] Commit any final tidy.

---

## Self-review checklist

- **Spec coverage.** Each verb in the spec maps to a task: posthook adapter
  (Task 1), skeleton (Task 2), plugin loader (Task 3), list (Task 4),
  resolve (Task 5), refine (Task 6), new (Task 7), regen (Task 8),
  accept-stage (Task 9), finalize (Task 10).
- **Placeholder scan.** Every step contains the actual code or the exact
  command an engineer types. No "TBD" or "implement later".
- **Type consistency.** Names match across tasks: `Plugin`, `Scope`, `Page`,
  `Runner`, `LoadPlugin`, `ParseScope`, `ScopeHash`, `(*Runner).List`,
  `(*Runner).Resolve`, `(*Runner).Refine`, `(*Runner).New`, `(*Runner).Regen`,
  `(*Runner).AcceptStage`, `(*Runner).Finalize`. Adapter: `PostHook` /
  `ExecPostHook`.
- **Cleanup is NOT in scope.** The shim file gets edited per task to flip
  verbs, but the bash file is not deleted; that's a future cleanup plan
  gated on the release window.
