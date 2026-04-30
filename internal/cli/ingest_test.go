package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunIngestVerbsNotYetPorted is retired as of slice 9: every flat
// ingest verb the slice plan listed is now Go-native. The historical
// not-yet-ported list (capture/listers/pdf/audio/xlsx/ingest/git/
// watchdog) is fully drained. The placeholder name is preserved as a
// breadcrumb in the commit log.
//
// Per-verb coverage:
//   - capture            internal/ingest/capture_test.go        (slice 2)
//   - ingest-batch-list  internal/ingest/list_test.go           (slice 3)
//   - ingest-git-list    internal/ingest/list_test.go           (slice 3)
//   - ingest-pdf         internal/ingest/formats/pdf_test.go    (slice 4)
//   - ingest-audio       internal/ingest/formats/audio_test.go  (slice 5)
//   - ingest-xlsx        internal/ingest/formats/xlsx_test.go   (slice 6)
//   - ingest             internal/ingest/bookkeep_test.go       (slice 7)
//   - ingest-git         internal/ingest/git/run_test.go        (slice 8)
//   - watchdog           internal/ingest/watchdog_test.go       (slice 9)
//
// The test is removed; this comment documents the audit trail.

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
