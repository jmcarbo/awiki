package initverb

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRunNonInteractiveE2E exercises the full orchestrator against a
// fixture wiki, using fakes for every adapter that touches the host.
// It verifies:
//   - state file is created with the expected answers stub
//   - every step runs to completion (applied or skipped)
//   - Step 7 patches WIKI.md / hugo.toml / content/_index.md
//   - Step 10 appends to content/log.md
func TestRunNonInteractiveE2E(t *testing.T) {
	dir := setupE2EFixture(t)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	deps := StepContext{
		Stdin:  strings.NewReader(""),
		Stdout: stdout,
		Stderr: stderr,
		Git:    &fakeGit{revParse: "deadbeefcafef00d"},
		Bash:   &fakeBash{},
		NPM:    &fakeNPM{},
		Agent:  &fakeAgent{out: "fixture landing"},
	}
	rc, err := Run(Options{
		RepoRoot:       dir,
		NonInteractive: true,
		Agent:          "claude",
		// Skip the dep-check (host-dependent) and install-qmd (network)
		// + wire-mcp + stage-commit (no real git here).
		SkipSteps: []string{
			"dep-check", "install-qmd", "wire-qmd-mcp",
			"wire-awiki-mcp", "stage-commit",
		},
		Now: time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
	}, deps)
	if err != nil {
		t.Fatalf("run: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}

	// Verify state file.
	st, err := LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st == nil {
		t.Fatal("state file missing after run")
	}
	if st.Answers.WikiName == "" {
		t.Errorf("wiki_name should fall back to dir basename, got %q", st.Answers.WikiName)
	}
	if st.Answers.Domain != "other" {
		t.Errorf("domain should default to 'other', got %q", st.Answers.Domain)
	}
	for _, want := range []string{
		"domain", "wiki-name", "privacy", "track-processed",
		"theme", "publish-log", "patch-identity", "log-init",
		"template-init", "smoke-test",
	} {
		rec := st.FindStep(want)
		if rec == nil {
			t.Errorf("step %s missing from state", want)
			continue
		}
		if rec.Status != StatusApplied && rec.Status != StatusSkipped {
			t.Errorf("step %s status=%s note=%s", want, rec.Status, rec.Note)
		}
	}

	// Verify Step 7 patched the identity files.
	wiki, _ := os.ReadFile(filepath.Join(dir, "WIKI.md"))
	if !strings.Contains(string(wiki), "`"+filepath.Base(dir)+"`") {
		t.Errorf("WIKI.md not patched with wiki_name: %s", wiki)
	}
	idx, _ := os.ReadFile(filepath.Join(dir, "content", "_index.md"))
	if !strings.Contains(string(idx), "fixture landing") {
		t.Errorf("content/_index.md missing agent landing: %s", idx)
	}

	// Verify Step 10 appended to the log.
	log, _ := os.ReadFile(filepath.Join(dir, "content", "log.md"))
	if !strings.Contains(string(log), "initialized for domain") {
		t.Errorf("content/log.md missing init entry: %s", log)
	}

	// Verify Step 12 wrote template.json.
	if _, err := os.Stat(filepath.Join(dir, ".awiki", "template.json")); err != nil {
		t.Errorf("template.json: %v", err)
	}
}

// TestRunContinueResumesAfterFailure simulates a failing privacy step
// (Bash adapter returns non-zero), then re-runs with --continue and a
// healthy bash. The second run resumes from the failed step.
func TestRunContinueResumesAfterFailure(t *testing.T) {
	dir := setupE2EFixture(t)
	failingBash := &fakeBash{code: 1, err: errFakeBash}
	deps := StepContext{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Git:    &fakeGit{},
		Bash:   failingBash,
		NPM:    &fakeNPM{},
	}
	// Force privacy=git-crypt so the Bash call is exercised.
	cfg := filepath.Join(dir, "init.yaml")
	if err := os.WriteFile(cfg, []byte("privacy: git-crypt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(Options{
		RepoRoot:       dir,
		NonInteractive: true,
		ConfigPath:     cfg,
		SkipSteps:      []string{"dep-check"},
		Now:            time.Now(),
	}, deps); err == nil {
		t.Fatal("expected first run to fail at privacy step")
	}
	st, _ := LoadState(dir)
	if rec := st.FindStep("privacy"); rec == nil || rec.Status != StatusFailed {
		t.Fatalf("privacy should be failed, got %+v", rec)
	}

	// Re-run with healthy bash + --continue.
	deps.Bash = &fakeBash{}
	deps.Stdout = &bytes.Buffer{}
	deps.Stderr = &bytes.Buffer{}
	if _, err := Run(Options{
		RepoRoot:       dir,
		NonInteractive: true,
		ConfigPath:     cfg,
		Continue:       true,
		SkipSteps: []string{
			"dep-check", "install-qmd", "wire-qmd-mcp",
			"wire-awiki-mcp", "stage-commit",
		},
		Now: time.Now(),
	}, deps); err != nil {
		t.Fatalf("resume: %v", err)
	}
	st2, _ := LoadState(dir)
	if rec := st2.FindStep("privacy"); rec == nil || rec.Status != StatusApplied {
		t.Fatalf("privacy after resume: %+v", rec)
	}
}

// TestRunWithConfigYAML exercises --config <yaml>.
func TestRunWithConfigYAML(t *testing.T) {
	dir := setupE2EFixture(t)
	cfg := filepath.Join(dir, "init.yaml")
	body := `domain: research
wiki_name: my-wiki
purpose: A wiki about X
privacy: none
track_processed: false
theme: hugo-book
publish_log: true
wire_qmd_mcp: false
wire_awiki_mcp: false
stage_commit: false
`
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := StepContext{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Git:    &fakeGit{},
		Bash:   &fakeBash{},
		NPM:    &fakeNPM{},
	}
	if _, err := Run(Options{
		RepoRoot:       dir,
		NonInteractive: true,
		ConfigPath:     cfg,
		SkipSteps: []string{
			"dep-check", "install-qmd", "wire-qmd-mcp",
			"wire-awiki-mcp", "stage-commit",
		},
		Now: time.Now(),
	}, deps); err != nil {
		t.Fatal(err)
	}
	st, _ := LoadState(dir)
	if st.Answers.WikiName != "my-wiki" {
		t.Errorf("wiki_name=%q", st.Answers.WikiName)
	}
	if st.Answers.Domain != "research" {
		t.Errorf("domain=%q", st.Answers.Domain)
	}
	if !st.Answers.PublishLog {
		t.Error("publish_log should be true")
	}
}

// errFakeBash is sentinel returned by the failing fakeBash above.
var errFakeBash = errFakeBashSentinel{}

type errFakeBashSentinel struct{}

func (errFakeBashSentinel) Error() string { return "fake bash error" }

// setupE2EFixture writes the minimal set of files the steps mutate.
func setupE2EFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "content"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	wiki := `# WIKI Schema

## 1. Identity
- **wiki_name:** ` + "`<unset>`" + `
- **domain:** ` + "`<unset>`" + `
- **purpose:** ` + "`<unset>`" + `

## 2. Conventions
`
	must := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must("WIKI.md", wiki)
	must("hugo.toml", "baseURL = 'https://example.com/'\ntitle = 'awiki'\ntheme = 'hugo-book'\n")
	must("content/_index.md", "---\ntitle: \"awiki\"\ntype: section-index\n---\n\n# Welcome\n")
	must("content/log.md", "---\ntitle: \"Log\"\ntype: log\ndraft: true\n---\n\n# Log\n")
	must(".gitignore", "raw/processed/*\n!raw/processed/.gitkeep\n")
	must("BOOTSTRAP.md", `# BOOTSTRAP

### Step 0
<!-- bootstrap-step: dep-check -->

dep-check body
`)
	must("template.manifest.toml", `schema_version = 1
template_version = "1.0.0"

[strategies]
overwrite = ["**"]

[new_file_default]
strategy = "prompt"

[bootstrap]
ordered_steps = ["dep-check"]

[bootstrap.dangerous]
ids = []
`)
	must("scripts/encrypt-init.sh", "#!/usr/bin/env bash\nexit 0\n")
	// Provide an empty mcp/awiki-server/index.js so the wire step can find it
	// (we won't actually run it because we skip wire-awiki-mcp).
	if err := os.MkdirAll(filepath.Join(dir, "mcp", "awiki-server"), 0o755); err != nil {
		t.Fatal(err)
	}
	must("mcp/awiki-server/index.js", "// stub\n")
	_ = context.Background()
	return dir
}
