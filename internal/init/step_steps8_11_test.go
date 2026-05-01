package initverb

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Step 9a: wire-qmd-mcp gating ---

func TestStepWireQmdMCPSkipsWhenStatusNotOk(t *testing.T) {
	dir := t.TempDir()
	ctx := StepContext{
		RepoRoot: dir,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
	}
	res, err := (stepWireQmdMCP{}).Execute(ctx, &Answers{WireQmdMCP: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusSkipped {
		t.Fatalf("expected skipped, got %+v", res)
	}
}

func TestStepWireQmdMCPSkipsWhenAnswerFalse(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".awiki"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".awiki", "qmd-status"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := StepContext{
		RepoRoot:       dir,
		NonInteractive: true,
		Stdout:         &bytes.Buffer{},
		Stderr:         &bytes.Buffer{},
	}
	res, err := (stepWireQmdMCP{}).Execute(ctx, &Answers{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusSkipped {
		t.Fatalf("expected skipped, got %+v", res)
	}
}

// --- Step 9b: wire-awiki-mcp ---

func TestStepWireAwikiMCPNoConsent(t *testing.T) {
	dir := t.TempDir()
	ctx := StepContext{
		RepoRoot:       dir,
		NonInteractive: true,
		Stdout:         &bytes.Buffer{},
		Stderr:         &bytes.Buffer{},
	}
	res, err := (stepWireAwikiMCP{}).Execute(ctx, &Answers{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusSkipped {
		t.Fatalf("status=%s", res.Status)
	}
}

func TestStepWireAwikiMCPRunsNPMInstall(t *testing.T) {
	dir := t.TempDir()
	npm := &fakeNPM{}
	ctx := StepContext{
		Ctx: context.Background(), RepoRoot: dir,
		NonInteractive: true,
		Stdout:         &bytes.Buffer{},
		Stderr:         &bytes.Buffer{},
		NPM:            npm,
	}
	// We expect this to fail in ops.WireAwikiMCP because there's no
	// MCP server file in the temp dir; but we should still see npm
	// install fired.
	_, _ = (stepWireAwikiMCP{}).Execute(ctx, &Answers{WireAwikiMCP: true})
	if npm.calls != 1 {
		t.Fatalf("npm calls=%d", npm.calls)
	}
	if !strings.HasSuffix(npm.lastDir, filepath.Join("mcp", "awiki-server")) {
		t.Fatalf("npm dir=%q", npm.lastDir)
	}
}

func TestStepWireAwikiMCPHaltsOnNPMError(t *testing.T) {
	dir := t.TempDir()
	npm := &fakeNPM{code: 1, err: errors.New("npm failed")}
	ctx := StepContext{
		Ctx: context.Background(), RepoRoot: dir,
		NonInteractive: true,
		Stdout:         &bytes.Buffer{},
		Stderr:         &bytes.Buffer{},
		NPM:            npm,
	}
	if _, err := (stepWireAwikiMCP{}).Execute(ctx, &Answers{WireAwikiMCP: true}); err == nil {
		t.Fatal("expected error from npm install failure")
	}
}

// --- Step 10: log-init ---

func TestStepLogInitAppendsEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "content"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\ntitle: \"Log\"\ntype: log\ndraft: true\n---\n\n# Log\n"
	if err := os.WriteFile(filepath.Join(dir, "content", "log.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := StepContext{
		RepoRoot: dir,
		Stdout:   &bytes.Buffer{},
		Stderr:   &bytes.Buffer{},
		Now:      time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
	}
	res, err := (stepLogInit{}).Execute(ctx, &Answers{
		WikiName: "rome", Domain: "research",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusApplied {
		t.Fatalf("status=%s", res.Status)
	}
	updated, _ := os.ReadFile(filepath.Join(dir, "content", "log.md"))
	if !strings.Contains(string(updated), "wiki 'rome' initialized for domain 'research'") {
		t.Errorf("log.md missing init entry: %s", updated)
	}
}

// --- Step 11: stage-commit ---

func TestStepStageCommitNoConsentJustStages(t *testing.T) {
	git := &fakeGit{statusOut: "M file.txt\n"}
	stdout := &bytes.Buffer{}
	ctx := StepContext{
		Ctx: context.Background(), RepoRoot: "/tmp/wiki",
		NonInteractive: true,
		Stdout:         stdout,
		Stderr:         &bytes.Buffer{},
		Git:            git,
	}
	res, err := (stepStageCommit{}).Execute(ctx, &Answers{WikiName: "rome"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusSkipped {
		t.Fatalf("status=%s note=%s", res.Status, res.Note)
	}
	if !strings.Contains(stdout.String(), "Files staged") {
		t.Errorf("expected staged-files header: %s", stdout)
	}
}

func TestStepStageCommitConsentCommits(t *testing.T) {
	git := &fakeGit{statusOut: "M file.txt\n"}
	ctx := StepContext{
		Ctx: context.Background(), RepoRoot: "/tmp/wiki",
		NonInteractive: true,
		Stdout:         &bytes.Buffer{},
		Stderr:         &bytes.Buffer{},
		Git:            git,
	}
	res, err := (stepStageCommit{}).Execute(ctx, &Answers{
		WikiName: "rome", StageCommit: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusApplied {
		t.Fatalf("status=%s", res.Status)
	}
	if !strings.Contains(res.Note, "chore: initialize wiki 'rome'") {
		t.Fatalf("note=%q", res.Note)
	}
}
