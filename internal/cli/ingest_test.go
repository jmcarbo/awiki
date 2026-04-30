package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunIngestVerbsNotYetPorted asserts every flat ingest verb wired
// in slice 1 returns the "verb not yet ported" sentinel on stderr.
// Subsequent slices replace each case as they land. `capture` is removed
// from this list as of slice 2 (it is now ported and has dedicated
// coverage in internal/ingest/capture_test.go). `ingest-batch-list` and
// `ingest-git-list` are removed as of slice 3 (leaf listers — coverage
// in internal/ingest/list_test.go). `ingest-pdf` is removed as of
// slice 4 (coverage in internal/ingest/formats/pdf_test.go).
// `ingest-audio` is removed as of slice 5 (coverage in
// internal/ingest/formats/audio_test.go). `ingest-xlsx` is removed as
// of slice 6 (coverage in internal/ingest/formats/xlsx_test.go).
// `ingest` is removed as of slice 7 (coverage in
// internal/ingest/bookkeep_test.go). `ingest-git` is removed as of
// slice 8 (coverage in internal/ingest/git/run_test.go).
func TestRunIngestVerbsNotYetPorted(t *testing.T) {
	verbs := []string{
		"watchdog",
	}
	t.Setenv("AWIKI_REPO_ROOT", t.TempDir())
	for _, v := range verbs {
		t.Run(v, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run([]string{v}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("Run(%q) code = 0, want non-zero", v)
			}
			want := "ingest: verb not yet ported: " + v
			if !strings.Contains(stderr.String(), want) {
				t.Fatalf("stderr = %q, want substring %q", stderr.String(), want)
			}
		})
	}
}

// TestRunIngestGitNoArgs asserts the ingest-git verb returns the bash-
// compat usage banner on missing positional arg.
func TestRunIngestGitNoArgs(t *testing.T) {
	t.Setenv("AWIKI_REPO_ROOT", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := Run([]string{"ingest-git"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("Run code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: ingest-git.sh") {
		t.Errorf("stderr missing usage banner: %q", stderr.String())
	}
}

// TestRunIngestGitUnknownFlag asserts the dispatcher rejects unknown
// flags with the bash-compat ERROR banner. Uses a missing repo dir so
// we never attempt the actual git work.
func TestRunIngestGitUnknownFlag(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AWIKI_REPO_ROOT", tmp)
	var stdout, stderr bytes.Buffer
	// Create an empty src dir so RepoKey resolves the local path branch.
	src := filepath.Join(tmp, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	code := Run([]string{"ingest-git", src, "--bogus"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("Run code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "unknown flag: --bogus") {
		t.Errorf("stderr missing unknown-flag error: %q", stderr.String())
	}
}

func TestRunIngestUnknownVerbInDispatcher(t *testing.T) {
	// The dispatcher rejects an unknown top-level command with
	// "unknown command", not the ingest-domain sentinel.
	var stdout, stderr bytes.Buffer
	code := Run([]string{"ingest-bogus"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("Run(\"ingest-bogus\") code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q, want \"unknown command\"", stderr.String())
	}
}
