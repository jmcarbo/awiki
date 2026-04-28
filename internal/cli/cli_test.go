package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunRequiresCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing command")
	}
	if stderr.String() == "" {
		t.Fatalf("expected usage text on stderr")
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"nope"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit for unknown command")
	}
	if got := stderr.String(); got == "" || !bytes.Contains([]byte(got), []byte("unknown command")) {
		t.Fatalf("stderr = %q, want unknown command message", got)
	}
}

func TestRunLintAcceptsFlagAfterContentDir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	contentDir := t.TempDir()
	code := Run([]string{"lint", contentDir, "--fix"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if got, want := stdout.String(), "LINT-SUMMARY|errors=0|warnings=0|info=0\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestParseLintOptionsAcceptsFlagAfterContentDir(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseLintOptions([]string{"custom-content", "--fix"}, &stderr)
	if err != nil {
		t.Fatalf("parseLintOptions() error = %v; stderr = %q", err, stderr.String())
	}
	if opts.ContentDir != "custom-content" {
		t.Fatalf("ContentDir = %q, want %q", opts.ContentDir, "custom-content")
	}
	if !opts.Fix {
		t.Fatalf("Fix = false, want true")
	}
}

func TestParseLintOptionsInfersRepoRootFromAbsoluteContentDir(t *testing.T) {
	var stderr bytes.Buffer
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "scripts"), 0o755); err != nil {
		t.Fatalf("MkdirAll(scripts) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "scripts", "lint.sh"), []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(lint.sh) error = %v", err)
	}
	contentDir := filepath.Join(repoRoot, "content")
	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(content) error = %v", err)
	}

	opts, err := parseLintOptions([]string{contentDir, "--only=synth"}, &stderr)
	if err != nil {
		t.Fatalf("parseLintOptions() error = %v; stderr = %q", err, stderr.String())
	}

	if opts.RepoRoot != repoRoot {
		t.Fatalf("RepoRoot = %q, want %q", opts.RepoRoot, repoRoot)
	}
}

func TestRunLintUnknownFlagReturnsOne(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"lint", "--not-a-flag"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run() code = %d, want 1", code)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}
