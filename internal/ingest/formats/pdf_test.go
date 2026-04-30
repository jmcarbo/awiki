package formats

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"awiki/internal/ingest"
	"awiki/internal/testutil"
)

// pdfFixedTime is the deterministic clock the slice-4 fixture-driven
// test injects. Format matches the bash `date '+%Y-%m-%d'` form used
// for the frontmatter `date` and `last_updated` fields.
var pdfFixedTime = time.Date(2026, 4, 30, 12, 34, 0, 0, time.UTC)

// pdfFixtureRoot resolves the repo-relative path to the slice 4
// fixture tree. Mirrors the helpers in capture_test.go and
// list_test.go.
func pdfFixtureRoot(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	// internal/ingest/formats/pdf_test.go -> repo root is three levels up.
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", "..", ".."))
	return filepath.Join(repoRoot, "tests", "fixtures", "ingest", "pdf")
}

// TestIngestPDFFixture drives formats.IngestPDF against the slice-4
// golden fixture. The fixture pins:
//   - input/raw/inbox/source.pdf  — placeholder bytes (the fake skips parsing).
//   - input/canned-text.txt       — what the FakePDFToText returns.
//   - args                        — `ingest-pdf <relative-path>`.
//   - expected_stdout             — byte-for-byte stdout (PDF-CONVERTED + Now run).
//   - expected_files/             — every file produced (markdown + original copy).
func TestIngestPDFFixture(t *testing.T) {
	fixture := pdfFixtureRoot(t)

	tmp := t.TempDir()
	testutil.CopyTree(t, filepath.Join(fixture, "input"), tmp)

	argsBytes, err := os.ReadFile(filepath.Join(fixture, "args"))
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	args := strings.Fields(strings.TrimSpace(string(argsBytes)))
	if len(args) < 2 || args[0] != "ingest-pdf" {
		t.Fatalf("args fixture must start with `ingest-pdf`, got %q", argsBytes)
	}
	srcArg := args[1]

	r := &ingest.Runner{
		RepoRoot: tmp,
		Today:    pdfFixedTime.Format("2006-01-02"),
		PDFToText: &testutil.FakePDFToText{
			TextPath: filepath.Join(tmp, "canned-text.txt"),
		},
	}

	var stdout, stderr bytes.Buffer
	res, err := IngestPDF(context.Background(), r, PDFOptions{SourcePath: srcArg}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("IngestPDF: %v\nstderr: %s", err, stderr.String())
	}
	if res == nil {
		t.Fatal("IngestPDF returned nil result")
	}

	wantStdout, err := os.ReadFile(filepath.Join(fixture, "expected_stdout"))
	if err != nil {
		t.Fatalf("read expected_stdout: %v", err)
	}
	if got := stdout.String(); got != string(wantStdout) {
		t.Errorf("stdout mismatch:\n got: %q\nwant: %q", got, string(wantStdout))
	}

	// Source PDF must be removed (bash: rm "$PDF").
	if _, err := os.Stat(filepath.Join(tmp, srcArg)); !os.IsNotExist(err) {
		t.Errorf("expected source pdf to be removed, stat err = %v", err)
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

// --------------------------------------------------------------------
// Behavioral tests covering the bash-compat guards. These run in
// addition to the fixture so each branch in pdf.go is pinned. Mirrors
// the cases in tests/ingest_pdf_test.bats so the bats oracle can be
// deleted in slice 10 without coverage loss.
// --------------------------------------------------------------------

// newPDFRunner returns a Runner rooted at a fresh tempdir with a
// raw/inbox/ tree pre-created and a FakePDFToText injected.
func newPDFRunner(t *testing.T, cannedText string) (*ingest.Runner, string) {
	t.Helper()
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "raw", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &ingest.Runner{
		RepoRoot:  tmp,
		Today:     pdfFixedTime.Format("2006-01-02"),
		PDFToText: &testutil.FakePDFToText{Text: cannedText},
	}
	return r, tmp
}

func TestIngestPDFRejectsNonPDFExtension(t *testing.T) {
	r, tmp := newPDFRunner(t, "x")
	if err := os.WriteFile(filepath.Join(tmp, "raw", "inbox", "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	_, err := IngestPDF(context.Background(), r, PDFOptions{SourcePath: "raw/inbox/note.txt"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for non-pdf extension, got nil")
	}
	var ee *ingest.ExitError
	if !errors.As(err, &ee) || ee.Code != 1 {
		t.Fatalf("expected ExitError code 1, got %v", err)
	}
	if !strings.Contains(stderr.String(), "not a pdf") {
		t.Errorf("stderr missing 'not a pdf': %q", stderr.String())
	}
}

func TestIngestPDFRejectsMissingFile(t *testing.T) {
	r, _ := newPDFRunner(t, "x")
	var stdout, stderr bytes.Buffer
	_, err := IngestPDF(context.Background(), r, PDFOptions{SourcePath: "raw/inbox/missing.pdf"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	var ee *ingest.ExitError
	if !errors.As(err, &ee) || ee.Code != 1 {
		t.Fatalf("expected ExitError code 1, got %v", err)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("stderr missing 'not found': %q", stderr.String())
	}
}

func TestIngestPDFRejectsPathOutsideInbox(t *testing.T) {
	r, tmp := newPDFRunner(t, "x")
	// Place a pdf outside raw/inbox/.
	if err := os.MkdirAll(filepath.Join(tmp, "elsewhere"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "elsewhere", "x.pdf"), []byte("%PDF-1.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	_, err := IngestPDF(context.Background(), r, PDFOptions{SourcePath: "elsewhere/x.pdf"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for path outside raw/inbox/, got nil")
	}
	var ee *ingest.ExitError
	if !errors.As(err, &ee) || ee.Code != 1 {
		t.Fatalf("expected ExitError code 1, got %v", err)
	}
	if !strings.Contains(stderr.String(), "must be under raw/inbox/") {
		t.Errorf("stderr missing inbox guard: %q", stderr.String())
	}
}

func TestIngestPDFAdapterFailureExitCode2(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "raw", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "raw", "inbox", "src.pdf"), []byte("%PDF-1.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &ingest.Runner{
		RepoRoot:  tmp,
		Today:     pdfFixedTime.Format("2006-01-02"),
		PDFToText: &testutil.FakePDFToText{Code: 1, Err: errors.New("boom")},
	}
	var stdout, stderr bytes.Buffer
	_, err := IngestPDF(context.Background(), r, PDFOptions{SourcePath: "raw/inbox/src.pdf"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected adapter failure, got nil")
	}
	var ee *ingest.ExitError
	if !errors.As(err, &ee) || ee.Code != 2 {
		t.Fatalf("expected ExitError code 2, got %v", err)
	}
	// Source must be left in place on extractor failure (no rm in
	// bash either — `set -e` aborts before the rm).
	if _, err := os.Stat(filepath.Join(tmp, "raw", "inbox", "src.pdf")); err != nil {
		t.Errorf("source pdf should remain on adapter failure: %v", err)
	}
}
