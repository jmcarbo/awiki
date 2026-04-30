package cli

import (
	"bytes"
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
func TestRunIngestVerbsNotYetPorted(t *testing.T) {
	verbs := []string{
		"ingest",
		"ingest-xlsx",
		"ingest-git",
		"ingest-audio",
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
