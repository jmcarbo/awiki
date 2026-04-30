package ingest

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"awiki/internal/testutil"
)

// bookkeepFixtureRoot resolves the repo-relative path to the slice 7
// fixture tree. Mirrors the helpers in capture_test.go and
// formats/pdf_test.go.
func bookkeepFixtureRoot(t *testing.T, name string) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
	return filepath.Join(repoRoot, "tests", "fixtures", "ingest", "ingest", name)
}

// disableLookPath swaps LookPathFn for the duration of the test so the
// auto-qmd-reindex gate and agent-CLI presence check run deterministic
// regardless of what is on the developer's PATH.
func disableLookPath(t *testing.T) {
	t.Helper()
	prev := LookPathFn
	LookPathFn = func(string) (string, error) {
		return "", errors.New("not found (test stub)")
	}
	t.Cleanup(func() { LookPathFn = prev })
}

// TestIngestBookkeepFixtureBatch drives IngestBookkeep against the
// slice 7 batch-mode fixture. The fixture pins:
//   - input/raw/inbox/batch/note.md  — seed source.
//   - args                           — `ingest <relative-path>`.
//   - expected_stdout                — INGEST-OK + AGENT-PROMPT
//                                       (count=1 < threshold=5; no AUTO-LINT).
//   - expected_files/raw/processed/batch/note.md — moved source.
//   - expected_files/.awiki/ingest-count          — counter at 1.
func TestIngestBookkeepFixtureBatch(t *testing.T) {
	disableLookPath(t)
	fixture := bookkeepFixtureRoot(t, "batch")

	tmp := t.TempDir()
	testutil.CopyTree(t, filepath.Join(fixture, "input"), tmp)

	argsBytes, err := os.ReadFile(filepath.Join(fixture, "args"))
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	args := strings.Fields(strings.TrimSpace(string(argsBytes)))
	if len(args) < 2 || args[0] != "ingest" {
		t.Fatalf("args fixture must start with `ingest`, got %q", argsBytes)
	}

	r := &Runner{
		RepoRoot:  tmp,
		Lint:      &testutil.FakeIngestLint{},
		Agent:     &testutil.FakeAgent{},
		Qmd:       &testutil.FakeQmd{},
		LogAppend: &testutil.FakeLogAppend{},
	}

	var stdout, stderr bytes.Buffer
	err = IngestBookkeep(context.Background(), r, BookkeepOptions{SourcePath: args[1]}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("IngestBookkeep: %v\nstderr: %s", err, stderr.String())
	}

	wantStdout, err := os.ReadFile(filepath.Join(fixture, "expected_stdout"))
	if err != nil {
		t.Fatalf("read expected_stdout: %v", err)
	}
	if got := stdout.String(); got != string(wantStdout) {
		t.Errorf("stdout mismatch:\n got: %q\nwant: %q", got, string(wantStdout))
	}

	// Source must be removed from the inbox after the rename.
	if _, err := os.Stat(filepath.Join(tmp, args[1])); !os.IsNotExist(err) {
		t.Errorf("expected source to be removed, stat err = %v", err)
	}

	// Walk expected_files/ and compare each file byte-for-byte.
	expectedRoot := filepath.Join(fixture, "expected_files")
	if err := filepath.Walk(expectedRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(expectedRoot, path)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(filepath.Join(tmp, rel))
		if err != nil {
			t.Errorf("missing expected file %s: %v", rel, err)
			return nil
		}
		want, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			t.Errorf("file %s mismatch\n got: %q\nwant: %q", rel, got, want)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk expected_files: %v", err)
	}
}

// TestIngestBookkeepFixtureBatchWithAgent drives the --agent variant.
// The fixture pins the AGENT-INVOKE record byte-format on top of
// INGEST-OK and AGENT-PROMPT.
func TestIngestBookkeepFixtureBatchWithAgent(t *testing.T) {
	// LookPathFn returns success for "claude" so the agent-invoke branch fires.
	prev := LookPathFn
	LookPathFn = func(name string) (string, error) {
		if name == "claude" {
			return "/fake/claude", nil
		}
		return "", errors.New("not found (test stub)")
	}
	t.Cleanup(func() { LookPathFn = prev })

	fixture := bookkeepFixtureRoot(t, "batch-with-agent")
	tmp := t.TempDir()
	testutil.CopyTree(t, filepath.Join(fixture, "input"), tmp)

	argsBytes, err := os.ReadFile(filepath.Join(fixture, "args"))
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	args := strings.Fields(strings.TrimSpace(string(argsBytes)))
	// args = ["ingest", "--agent", "claude", "raw/inbox/batch/agent-test.md"]
	if len(args) < 4 || args[0] != "ingest" || args[1] != "--agent" {
		t.Fatalf("args fixture must start with `ingest --agent`, got %q", argsBytes)
	}

	fakeAgent := &testutil.FakeAgent{}
	r := &Runner{
		RepoRoot:  tmp,
		Lint:      &testutil.FakeIngestLint{},
		Agent:     fakeAgent,
		Qmd:       &testutil.FakeQmd{},
		LogAppend: &testutil.FakeLogAppend{},
	}

	var stdout, stderr bytes.Buffer
	err = IngestBookkeep(context.Background(), r, BookkeepOptions{
		SourcePath: args[3],
		AgentCLI:   args[2],
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("IngestBookkeep: %v\nstderr: %s", err, stderr.String())
	}

	wantStdout, err := os.ReadFile(filepath.Join(fixture, "expected_stdout"))
	if err != nil {
		t.Fatalf("read expected_stdout: %v", err)
	}
	if got := stdout.String(); got != string(wantStdout) {
		t.Errorf("stdout mismatch:\n got: %q\nwant: %q", got, string(wantStdout))
	}
	if fakeAgent.Calls != 1 {
		t.Errorf("expected exactly 1 agent invocation, got %d", fakeAgent.Calls)
	}
	if fakeAgent.LastCLI != "claude" {
		t.Errorf("agent.LastCLI = %q, want claude", fakeAgent.LastCLI)
	}
}

// --------------------------------------------------------------------
// Behavioral tests covering the bash-compat guards. These run in
// addition to the fixture so each branch in bookkeep.go is pinned.
// They mirror the exit-code contract at scripts/ingest.sh:142-145 so
// the bash oracle can be deleted in slice 10 without coverage loss.
// --------------------------------------------------------------------

// newBookkeepRunner returns a Runner rooted at a fresh tempdir with the
// raw/inbox/<mode>/ tree pre-created and a single source file in place.
// The returned src argument is repo-relative.
func newBookkeepRunner(t *testing.T, mode, name string) (*Runner, string, string) {
	t.Helper()
	disableLookPath(t)
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "raw", "inbox", mode)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.ToSlash(filepath.Join("raw", "inbox", mode, name))
	if err := os.WriteFile(filepath.Join(tmp, src), []byte("body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &Runner{
		RepoRoot:  tmp,
		Lint:      &testutil.FakeIngestLint{},
		Agent:     &testutil.FakeAgent{},
		Qmd:       &testutil.FakeQmd{},
		LogAppend: &testutil.FakeLogAppend{},
	}
	return r, tmp, src
}

func TestIngestBookkeepRejectsNonInboxPath(t *testing.T) {
	disableLookPath(t)
	tmp := t.TempDir()
	r := &Runner{RepoRoot: tmp}
	var stdout, stderr bytes.Buffer
	err := IngestBookkeep(context.Background(), r, BookkeepOptions{SourcePath: "elsewhere/x.md"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 2 {
		t.Fatalf("expected ExitError code 2, got %v", err)
	}
	if !strings.Contains(stderr.String(), "must be under raw/inbox/") {
		t.Errorf("stderr missing inbox guard: %q", stderr.String())
	}
}

func TestIngestBookkeepRejectsMissingSource(t *testing.T) {
	disableLookPath(t)
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "raw", "inbox", "batch"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &Runner{RepoRoot: tmp}
	var stdout, stderr bytes.Buffer
	err := IngestBookkeep(context.Background(), r, BookkeepOptions{SourcePath: "raw/inbox/batch/missing.md"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 1 {
		t.Fatalf("expected ExitError code 1, got %v", err)
	}
	if !strings.Contains(stderr.String(), "source not found") {
		t.Errorf("stderr missing source-not-found: %q", stderr.String())
	}
}

func TestIngestBookkeepRejectsAlreadyProcessed(t *testing.T) {
	r, tmp, src := newBookkeepRunner(t, "batch", "dup.md")
	// Pre-create the destination so the guard fires.
	dst := filepath.Join(tmp, "raw", "processed", "batch", "dup.md")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("prior\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := IngestBookkeep(context.Background(), r, BookkeepOptions{SourcePath: src}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 3 {
		t.Fatalf("expected ExitError code 3, got %v", err)
	}
	if !strings.Contains(stderr.String(), "already processed") {
		t.Errorf("stderr missing already-processed guard: %q", stderr.String())
	}
}

func TestIngestBookkeepAgentSkipNonBatch(t *testing.T) {
	r, _, src := newBookkeepRunner(t, "interactive", "n.md")
	// Stub LookPathFn so claude is "found" — we still expect AGENT-SKIP
	// because mode != batch.
	prev := LookPathFn
	LookPathFn = func(string) (string, error) { return "/fake/claude", nil }
	t.Cleanup(func() { LookPathFn = prev })
	fakeAgent := &testutil.FakeAgent{}
	r.Agent = fakeAgent

	var stdout, stderr bytes.Buffer
	err := IngestBookkeep(context.Background(), r, BookkeepOptions{
		SourcePath: src,
		AgentCLI:   "claude",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("IngestBookkeep: %v", err)
	}
	if !strings.Contains(stderr.String(), "AGENT-SKIP|reason=mode-needs-human|mode=interactive|cli=claude") {
		t.Errorf("stderr missing AGENT-SKIP|reason=mode-needs-human: %q", stderr.String())
	}
	if fakeAgent.Calls != 0 {
		t.Errorf("agent must not be invoked on non-batch mode, calls=%d", fakeAgent.Calls)
	}
}

func TestIngestBookkeepAgentSkipCLINotFound(t *testing.T) {
	r, _, src := newBookkeepRunner(t, "batch", "n.md")
	// LookPathFn already disabled by newBookkeepRunner — claude is "missing".
	fakeAgent := &testutil.FakeAgent{}
	r.Agent = fakeAgent

	var stdout, stderr bytes.Buffer
	err := IngestBookkeep(context.Background(), r, BookkeepOptions{
		SourcePath: src,
		AgentCLI:   "claude",
	}, &stdout, &stderr)
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 6 {
		t.Fatalf("expected ExitError code 6, got %v", err)
	}
	if !strings.Contains(stderr.String(), "AGENT-SKIP|reason=cli-not-found|cli=claude") {
		t.Errorf("stderr missing AGENT-SKIP|reason=cli-not-found: %q", stderr.String())
	}
	if fakeAgent.Calls != 0 {
		t.Errorf("agent must not be invoked when CLI missing, calls=%d", fakeAgent.Calls)
	}
}

func TestIngestBookkeepAgentInvokeBatchSuccess(t *testing.T) {
	r, _, src := newBookkeepRunner(t, "batch", "n.md")
	prev := LookPathFn
	LookPathFn = func(string) (string, error) { return "/fake/claude", nil }
	t.Cleanup(func() { LookPathFn = prev })
	fakeAgent := &testutil.FakeAgent{Code: 0}
	r.Agent = fakeAgent

	var stdout, stderr bytes.Buffer
	err := IngestBookkeep(context.Background(), r, BookkeepOptions{
		SourcePath: src,
		AgentCLI:   "claude",
		AgentFlags: "--permission-mode acceptEdits",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("IngestBookkeep: %v\nstderr: %s", err, stderr.String())
	}
	if fakeAgent.Calls != 1 {
		t.Errorf("expected 1 agent call, got %d", fakeAgent.Calls)
	}
	if fakeAgent.LastCLI != "claude" {
		t.Errorf("LastCLI = %q, want claude", fakeAgent.LastCLI)
	}
	if fakeAgent.LastFlags != "--permission-mode acceptEdits" {
		t.Errorf("LastFlags = %q", fakeAgent.LastFlags)
	}
	if !strings.Contains(fakeAgent.LastPrompt, "raw/processed/batch/n.md") {
		t.Errorf("LastPrompt missing dest path: %q", fakeAgent.LastPrompt)
	}
	if !strings.Contains(stdout.String(), "AGENT-INVOKE|cli=claude|mode=batch") {
		t.Errorf("stdout missing AGENT-INVOKE: %q", stdout.String())
	}
}

func TestIngestBookkeepAgentInvokeBatchFailure(t *testing.T) {
	r, _, src := newBookkeepRunner(t, "batch", "n.md")
	prev := LookPathFn
	LookPathFn = func(string) (string, error) { return "/fake/claude", nil }
	t.Cleanup(func() { LookPathFn = prev })
	fakeAgent := &testutil.FakeAgent{Code: 1, Err: errors.New("agent died")}
	r.Agent = fakeAgent

	var stdout, stderr bytes.Buffer
	err := IngestBookkeep(context.Background(), r, BookkeepOptions{
		SourcePath: src,
		AgentCLI:   "claude",
	}, &stdout, &stderr)
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 6 {
		t.Fatalf("expected ExitError code 6, got %v", err)
	}
}

func TestIngestBookkeepCounterIncrementsAndResetsOnAutoLint(t *testing.T) {
	r, tmp, src := newBookkeepRunner(t, "batch", "first.md")
	r.Config = map[string]string{"AWIKI_LINT_AFTER_N": "1"}
	fakeLint := &testutil.FakeIngestLint{Code: 0}
	r.Lint = fakeLint

	var stdout, stderr bytes.Buffer
	if err := IngestBookkeep(context.Background(), r, BookkeepOptions{SourcePath: src}, &stdout, &stderr); err != nil {
		t.Fatalf("IngestBookkeep: %v\nstderr: %s", err, stderr.String())
	}
	if fakeLint.Calls != 1 {
		t.Errorf("expected 1 lint call, got %d", fakeLint.Calls)
	}
	if !strings.Contains(stdout.String(), "AUTO-LINT|threshold=1|count=1") {
		t.Errorf("stdout missing AUTO-LINT: %q", stdout.String())
	}
	// After auto-lint the counter resets to 0.
	count, err := os.ReadFile(filepath.Join(tmp, ".awiki", "ingest-count"))
	if err != nil {
		t.Fatalf("read counter: %v", err)
	}
	if strings.TrimSpace(string(count)) != "0" {
		t.Errorf("counter not reset: %q", count)
	}
}

func TestIngestBookkeepLintExit2YieldsCode4(t *testing.T) {
	r, _, src := newBookkeepRunner(t, "batch", "n.md")
	r.Config = map[string]string{"AWIKI_LINT_AFTER_N": "1"}
	r.Lint = &testutil.FakeIngestLint{Code: 2, Err: errors.New("lint errors")}

	var stdout, stderr bytes.Buffer
	err := IngestBookkeep(context.Background(), r, BookkeepOptions{SourcePath: src}, &stdout, &stderr)
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 4 {
		t.Fatalf("expected ExitError code 4, got %v\nstderr: %s", err, stderr.String())
	}
}

func TestIngestBookkeepQmdFailureYieldsCode5(t *testing.T) {
	r, _, src := newBookkeepRunner(t, "batch", "n.md")
	prev := LookPathFn
	LookPathFn = func(name string) (string, error) {
		if name == "qmd" {
			return "/fake/qmd", nil
		}
		return "", errors.New("not found")
	}
	t.Cleanup(func() { LookPathFn = prev })
	r.Qmd = &testutil.FakeQmd{ReindexCode: 1, ReindexErr: errors.New("qmd failed")}

	var stdout, stderr bytes.Buffer
	err := IngestBookkeep(context.Background(), r, BookkeepOptions{SourcePath: src}, &stdout, &stderr)
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 5 {
		t.Fatalf("expected ExitError code 5, got %v\nstderr: %s", err, stderr.String())
	}
}

func TestIngestBookkeepExitPrecedenceLintOverQmdOverAgent(t *testing.T) {
	r, _, src := newBookkeepRunner(t, "batch", "n.md")
	r.Config = map[string]string{"AWIKI_LINT_AFTER_N": "1"}
	prev := LookPathFn
	LookPathFn = func(string) (string, error) { return "/fake/x", nil }
	t.Cleanup(func() { LookPathFn = prev })

	r.Lint = &testutil.FakeIngestLint{Code: 2}                            // -> LINT_RC=4
	r.Qmd = &testutil.FakeQmd{ReindexCode: 1, ReindexErr: errors.New("x")} // -> QMD_RC=5
	r.Agent = &testutil.FakeAgent{Code: 1, Err: errors.New("x")}          // -> AGENT_RC=6

	var stdout, stderr bytes.Buffer
	err := IngestBookkeep(context.Background(), r, BookkeepOptions{
		SourcePath: src,
		AgentCLI:   "claude",
	}, &stdout, &stderr)
	var ee *ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("expected ExitError, got %v", err)
	}
	// Lint error must dominate.
	if ee.Code != 4 {
		t.Fatalf("expected exit 4 (lint precedence), got %d", ee.Code)
	}
}

func TestIngestBookkeepAgentPromptByteFormat(t *testing.T) {
	r, _, src := newBookkeepRunner(t, "batch", "p.md")
	var stdout, stderr bytes.Buffer
	if err := IngestBookkeep(context.Background(), r, BookkeepOptions{SourcePath: src}, &stdout, &stderr); err != nil {
		t.Fatalf("IngestBookkeep: %v", err)
	}
	want := "AGENT-PROMPT|Process the source at raw/processed/batch/p.md per WIKI.md §4.1 ingest workflow steps 3-9 (mode=batch). Read it, write content/sources/<slug>.md, update affected entity/concept/topic pages, update content/catalog.md, update section indexes if section purpose changed. Run `just lint` afterward.\n"
	if !strings.Contains(stdout.String(), want) {
		t.Errorf("AGENT-PROMPT mismatch:\n got: %q\nwant substring: %q", stdout.String(), want)
	}
	// Backtick-escaped `just lint` must be literally present.
	if !strings.Contains(stdout.String(), "`just lint`") {
		t.Errorf("AGENT-PROMPT missing literal `just lint` backticks: %q", stdout.String())
	}
}

func TestIngestBookkeepUsageWhenSrcEmpty(t *testing.T) {
	r := &Runner{RepoRoot: t.TempDir()}
	var stdout, stderr bytes.Buffer
	err := IngestBookkeep(context.Background(), r, BookkeepOptions{}, &stdout, &stderr)
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 1 {
		t.Fatalf("expected ExitError code 1, got %v", err)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Errorf("stderr missing usage: %q", stderr.String())
	}
}
