package initverb

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestStateRoundTrip verifies that NewState → SaveState → LoadState
// preserves the document byte-for-byte (modulo trailing newline +
// indent which SaveState normalizes deterministically).
func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	s := NewState(now, "claude")
	s.Answers.Domain = "research"
	s.Answers.WikiName = "my-wiki"
	s.UpsertStep("dep-check", StatusApplied, "ok", now)
	if err := SaveState(dir, s); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadState(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded == nil {
		t.Fatal("loaded nil")
	}
	if loaded.Answers.Domain != "research" {
		t.Fatalf("domain: got %q", loaded.Answers.Domain)
	}
	if loaded.Answers.WikiName != "my-wiki" {
		t.Fatalf("wiki_name: got %q", loaded.Answers.WikiName)
	}
	if !loaded.IsApplied("dep-check") {
		t.Fatal("dep-check should be applied")
	}
}

// TestLoadStateMissing verifies an absent file returns (nil, nil).
func TestLoadStateMissing(t *testing.T) {
	dir := t.TempDir()
	s, err := LoadState(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s != nil {
		t.Fatalf("expected nil state for missing file, got %+v", s)
	}
}

// TestResetState removes the file idempotently.
func TestResetState(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	if err := SaveState(dir, NewState(now, "")); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := ResetState(dir); err != nil {
		t.Fatalf("reset: %v", err)
	}
	// Idempotent re-call.
	if err := ResetState(dir); err != nil {
		t.Fatalf("reset re-call: %v", err)
	}
	if _, err := os.Stat(StatePath(dir)); !os.IsNotExist(err) {
		t.Fatalf("state file should be gone, err=%v", err)
	}
}

// TestLoadConfig parses a minimal config file with quoted strings,
// inline comments, and bool variants.
func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	body := `# init config
domain: research
wiki_name: "my-wiki"
purpose: 'A wiki about X'  # trailing comment
privacy: none
track_processed: true
theme: hugo-book
publish_log: false
wire_qmd_mcp: yes
wire_awiki_mcp: no
stage_commit: 1
agent: claude
`
	cfg := filepath.Join(dir, "init.yaml")
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	ans, agent, err := LoadConfig(cfg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if ans.Domain != "research" {
		t.Errorf("domain: %q", ans.Domain)
	}
	if ans.WikiName != "my-wiki" {
		t.Errorf("wiki_name: %q", ans.WikiName)
	}
	if ans.Purpose != "A wiki about X" {
		t.Errorf("purpose: %q", ans.Purpose)
	}
	if !ans.TrackProcessed {
		t.Error("track_processed: false")
	}
	if ans.PublishLog {
		t.Error("publish_log: true")
	}
	if !ans.WireQmdMCP {
		t.Error("wire_qmd_mcp: false")
	}
	if ans.WireAwikiMCP {
		t.Error("wire_awiki_mcp: true")
	}
	if !ans.StageCommit {
		t.Error("stage_commit: false")
	}
	if agent != "claude" {
		t.Errorf("agent: %q", agent)
	}
}

// TestRunStubHonorsSkipAndContinue verifies the orchestrator wires
// --skip-step + --continue + --reset against the step list. Real
// step impls are exercised in their dedicated tests; here we skip
// every step so the orchestrator's flow is the only thing under test.
func TestRunStubHonorsSkipAndContinue(t *testing.T) {
	dir := t.TempDir()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	deps := StepContext{
		Stdin:  strings.NewReader(""),
		Stdout: stdout,
		Stderr: stderr,
	}
	allSkipped := []string{
		"dep-check", "domain", "wiki-name", "privacy", "track-processed",
		"theme", "publish-log", "patch-identity", "install-qmd",
		"wire-qmd-mcp", "wire-awiki-mcp", "log-init", "stage-commit",
		"template-init", "smoke-test",
	}
	rc, err := Run(Options{
		RepoRoot:       dir,
		NonInteractive: true,
		SkipSteps:      allSkipped,
		Now:            time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
	}, deps)
	if err != nil {
		t.Fatalf("run: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	st, err := LoadState(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	depRec := st.FindStep("dep-check")
	if depRec == nil || depRec.Status != StatusSkipped {
		t.Fatalf("dep-check should be skipped, got %+v", depRec)
	}
	if !strings.Contains(stdout.String(), "INIT|skip|dep-check") {
		t.Errorf("stdout missing skip marker: %s", stdout)
	}
	if !strings.Contains(stdout.String(), "INIT|done") {
		t.Errorf("stdout missing done marker: %s", stdout)
	}

	// --continue on the same state should resume past the prior steps.
	stdout2 := &bytes.Buffer{}
	deps.Stdout = stdout2
	if _, err := Run(Options{
		RepoRoot:       dir,
		NonInteractive: true,
		SkipSteps:      allSkipped,
		Continue:       true,
		Now:            time.Date(2026, 5, 1, 12, 1, 0, 0, time.UTC),
	}, deps); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if !strings.Contains(stdout2.String(), "INIT|done") {
		t.Errorf("rerun stdout missing done marker: %s", stdout2)
	}
}

// TestRunReset wipes state.
func TestRunReset(t *testing.T) {
	dir := t.TempDir()
	stdout := &bytes.Buffer{}
	deps := StepContext{
		Stdin:  strings.NewReader(""),
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
	}
	allSkipped := []string{
		"dep-check", "domain", "wiki-name", "privacy", "track-processed",
		"theme", "publish-log", "patch-identity", "install-qmd",
		"wire-qmd-mcp", "wire-awiki-mcp", "log-init", "stage-commit",
		"template-init", "smoke-test",
	}
	if _, err := Run(Options{RepoRoot: dir, NonInteractive: true, SkipSteps: allSkipped, Now: time.Now()}, deps); err != nil {
		t.Fatalf("first run: %v", err)
	}
	stdout.Reset()
	if _, err := Run(Options{RepoRoot: dir, NonInteractive: true, SkipSteps: allSkipped, Reset: true, Now: time.Now()}, deps); err != nil {
		t.Fatalf("reset run: %v", err)
	}
	if !strings.Contains(stdout.String(), "INIT|reset") {
		t.Errorf("expected reset marker in stdout: %s", stdout)
	}
}
