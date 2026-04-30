package formats

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"awiki/internal/ingest"
	"awiki/internal/testutil"
)

// xlsxFixtureRoot resolves the repo-relative path to the slice 6
// fixture tree. Mirrors pdfFixtureRoot / audioFixtureRoot.
func xlsxFixtureRoot(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", "..", ".."))
	return filepath.Join(repoRoot, "tests", "fixtures", "ingest", "xlsx")
}

// fakeManifestForSample returns a minimal one-sheet manifest matching
// the slice-6 fixture. Repo-relative paths mirror what the real
// xlsx-extract.py would write into the manifest after passing through
// out-dir / csv-dir.
func fakeManifestForSample() string {
	type item struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		RowsTotal   int    `json:"rows_total"`
		RowsPreview int    `json:"rows_preview"`
		CSV         string `json:"csv"`
		MD          string `json:"md"`
	}
	type m struct {
		WorkbookSlug string `json:"workbook_slug"`
		Sheets       []item `json:"sheets"`
	}
	out := m{
		WorkbookSlug: "sample",
		Sheets: []item{{
			Name:        "Sheet1",
			Slug:        "sheet-1",
			RowsTotal:   10,
			RowsPreview: 10,
			CSV:         "raw/processed/_originals/sample/sample-sheet-1.csv",
			MD:          "raw/inbox/sample-sheet-1.md",
		}},
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// canonicalSheetMD is the exact markdown body the fake writes for
// `sample-sheet-1.md`. Pinned in expected_files/.../sample-sheet-1.md.
const canonicalSheetMD = `---
title: "sample — Sheet1"
date: 2026-04-30
last_updated: 2026-04-30
type: source
tags: [xlsx]
aliases: []
sources: []
workbook: "sample.xlsx"
sheet: "Sheet1"
rows_total: 10
rows_preview: 10
columns: ["a","b"]
csv: raw/processed/_originals/sample/sample-sheet-1.csv
original: raw/processed/_originals/sample/sample.xlsx
draft: false
---

## Preview

(canned per-sheet markdown produced by FakeXLSXExtract)
`

const canonicalSheetCSV = `a,b
1,2
3,4
5,6
7,8
9,10
11,12
13,14
15,16
17,18
19,20
`

// TestIngestXLSXFixture drives formats.IngestXLSX against the slice-6
// golden fixture. The fixture pins the XLSX-SHEET / XLSX-CONVERTED /
// XLSX-NEXT records in bash order, the canned per-sheet markdown,
// and the stashed original.
func TestIngestXLSXFixture(t *testing.T) {
	fixture := xlsxFixtureRoot(t)

	tmp := t.TempDir()
	testutil.CopyTree(t, filepath.Join(fixture, "input"), tmp)

	argsBytes, err := os.ReadFile(filepath.Join(fixture, "args"))
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	args := strings.Fields(strings.TrimSpace(string(argsBytes)))
	if len(args) < 2 || args[0] != "ingest-xlsx" {
		t.Fatalf("args fixture must start with `ingest-xlsx`, got %q", argsBytes)
	}
	srcArg := args[1]
	// The fixture's args also pin the --preview-rows flag. We do not
	// need to parse it here; the canned manifest is fixed and the
	// flag's only effect is to forward to the helper.

	r := &ingest.Runner{
		RepoRoot: tmp,
		XLSXExtract: &testutil.FakeXLSXExtract{
			ManifestJSON: fakeManifestForSample(),
			Files: map[string][]byte{
				"sample-sheet-1.md":  []byte(canonicalSheetMD),
				"sample-sheet-1.csv": []byte(canonicalSheetCSV),
			},
		},
	}

	var stdout, stderr bytes.Buffer
	res, err := IngestXLSX(context.Background(), r, XLSXOptions{
		SourcePath:  srcArg,
		PreviewRows: 25,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("IngestXLSX: %v\nstderr: %s", err, stderr.String())
	}
	if res == nil {
		t.Fatal("IngestXLSX returned nil result")
	}

	wantStdout, err := os.ReadFile(filepath.Join(fixture, "expected_stdout"))
	if err != nil {
		t.Fatalf("read expected_stdout: %v", err)
	}
	if got := stdout.String(); got != string(wantStdout) {
		t.Errorf("stdout mismatch:\n got: %q\nwant: %q", got, string(wantStdout))
	}

	if _, err := os.Stat(filepath.Join(tmp, srcArg)); !os.IsNotExist(err) {
		t.Errorf("expected source xlsx to be removed, stat err = %v", err)
	}

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
// Behavioral tests covering the bash-compat guards. Mirrors the cases
// in scripts/ingest-xlsx.sh's error paths so the bats oracle (none yet,
// but anticipated) can be deleted in slice 10 without coverage loss.
// --------------------------------------------------------------------

// newXLSXRunner returns a Runner rooted at a fresh tempdir with a
// raw/inbox/ tree pre-created and a FakeXLSXExtract injected.
func newXLSXRunner(t *testing.T, fake *testutil.FakeXLSXExtract) (*ingest.Runner, string) {
	t.Helper()
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "raw", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &ingest.Runner{
		RepoRoot:    tmp,
		XLSXExtract: fake,
	}
	return r, tmp
}

func TestIngestXLSXRejectsBadPath(t *testing.T) {
	r, _ := newXLSXRunner(t, &testutil.FakeXLSXExtract{})
	var stdout, stderr bytes.Buffer
	_, err := IngestXLSX(context.Background(), r, XLSXOptions{SourcePath: "elsewhere/x.xlsx"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for path outside raw/inbox/, got nil")
	}
	var ee *ingest.ExitError
	if !errors.As(err, &ee) || ee.Code != 2 {
		t.Fatalf("expected ExitError code 2, got %v", err)
	}
	if !strings.Contains(stderr.String(), "XLSX-ERROR|reason=bad-path") {
		t.Errorf("stderr missing bad-path: %q", stderr.String())
	}
}

func TestIngestXLSXRejectsTraversalSegment(t *testing.T) {
	r, tmp := newXLSXRunner(t, &testutil.FakeXLSXExtract{})
	if err := os.MkdirAll(filepath.Join(tmp, "raw", "inbox", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	_, err := IngestXLSX(context.Background(), r, XLSXOptions{SourcePath: "raw/inbox/sub/../../etc/passwd.xlsx"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for traversal segment, got nil")
	}
	var ee *ingest.ExitError
	if !errors.As(err, &ee) || ee.Code != 2 {
		t.Fatalf("expected ExitError code 2, got %v", err)
	}
	if !strings.Contains(stderr.String(), "XLSX-ERROR|reason=bad-path") {
		t.Errorf("stderr missing bad-path: %q", stderr.String())
	}
}

func TestIngestXLSXRejectsMissingFile(t *testing.T) {
	r, _ := newXLSXRunner(t, &testutil.FakeXLSXExtract{})
	var stdout, stderr bytes.Buffer
	_, err := IngestXLSX(context.Background(), r, XLSXOptions{SourcePath: "raw/inbox/missing.xlsx"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	var ee *ingest.ExitError
	if !errors.As(err, &ee) || ee.Code != 1 {
		t.Fatalf("expected ExitError code 1, got %v", err)
	}
	if !strings.Contains(stderr.String(), "XLSX-ERROR|reason=not-found") {
		t.Errorf("stderr missing not-found: %q", stderr.String())
	}
}

func TestIngestXLSXRejectsBadExtension(t *testing.T) {
	r, tmp := newXLSXRunner(t, &testutil.FakeXLSXExtract{})
	if err := os.WriteFile(filepath.Join(tmp, "raw", "inbox", "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	_, err := IngestXLSX(context.Background(), r, XLSXOptions{SourcePath: "raw/inbox/note.txt"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for bad extension, got nil")
	}
	var ee *ingest.ExitError
	if !errors.As(err, &ee) || ee.Code != 2 {
		t.Fatalf("expected ExitError code 2, got %v", err)
	}
	if !strings.Contains(stderr.String(), "XLSX-ERROR|reason=bad-ext") {
		t.Errorf("stderr missing bad-ext: %q", stderr.String())
	}
}

func TestIngestXLSXMissingDep(t *testing.T) {
	r, tmp := newXLSXRunner(t, &testutil.FakeXLSXExtract{DepMissing: true})
	if err := os.WriteFile(filepath.Join(tmp, "raw", "inbox", "wb.xlsx"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	_, err := IngestXLSX(context.Background(), r, XLSXOptions{SourcePath: "raw/inbox/wb.xlsx"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected missing-dep error, got nil")
	}
	var ee *ingest.ExitError
	if !errors.As(err, &ee) || ee.Code != 5 {
		t.Fatalf("expected ExitError code 5, got %v", err)
	}
	if !strings.Contains(stderr.String(), "XLSX-ERROR|reason=missing-dep|dep=python-calamine") {
		t.Errorf("stderr missing missing-dep: %q", stderr.String())
	}
	// install hint also on stderr
	if !strings.Contains(stderr.String(), "install: pip3 install python-calamine") {
		t.Errorf("stderr missing install hint: %q", stderr.String())
	}
}

func TestIngestXLSXBadSlug(t *testing.T) {
	// Stem of "---" slugifies to empty in the fake (mirrors the helper).
	r, tmp := newXLSXRunner(t, &testutil.FakeXLSXExtract{})
	if err := os.WriteFile(filepath.Join(tmp, "raw", "inbox", "---.xlsx"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	_, err := IngestXLSX(context.Background(), r, XLSXOptions{SourcePath: "raw/inbox/---.xlsx"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected bad-slug error, got nil")
	}
	var ee *ingest.ExitError
	if !errors.As(err, &ee) || ee.Code != 2 {
		t.Fatalf("expected ExitError code 2, got %v", err)
	}
	if !strings.Contains(stderr.String(), "XLSX-ERROR|reason=bad-slug") {
		t.Errorf("stderr missing bad-slug: %q", stderr.String())
	}
}

func TestIngestXLSXEmptyWorkbook(t *testing.T) {
	r, tmp := newXLSXRunner(t, &testutil.FakeXLSXExtract{
		ManifestJSON: `{"workbook_slug":"wb","sheets":[]}`,
	})
	if err := os.WriteFile(filepath.Join(tmp, "raw", "inbox", "wb.xlsx"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	_, err := IngestXLSX(context.Background(), r, XLSXOptions{SourcePath: "raw/inbox/wb.xlsx"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected empty-workbook error, got nil")
	}
	var ee *ingest.ExitError
	if !errors.As(err, &ee) || ee.Code != 6 {
		t.Fatalf("expected ExitError code 6, got %v", err)
	}
	if !strings.Contains(stdout.String(), "XLSX-EMPTY|workbook=wb|src=raw/inbox/wb.xlsx") {
		t.Errorf("stdout missing XLSX-EMPTY: %q", stdout.String())
	}
}

func TestIngestXLSXBadManifest(t *testing.T) {
	r, tmp := newXLSXRunner(t, &testutil.FakeXLSXExtract{
		ManifestJSON: `not-json{`,
	})
	if err := os.WriteFile(filepath.Join(tmp, "raw", "inbox", "wb.xlsx"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	_, err := IngestXLSX(context.Background(), r, XLSXOptions{SourcePath: "raw/inbox/wb.xlsx"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected bad-manifest error, got nil")
	}
	var ee *ingest.ExitError
	if !errors.As(err, &ee) || ee.Code != 1 {
		t.Fatalf("expected ExitError code 1, got %v", err)
	}
	if !strings.Contains(stderr.String(), "XLSX-ERROR|reason=bad-manifest") {
		t.Errorf("stderr missing bad-manifest: %q", stderr.String())
	}
}

func TestIngestXLSXExtractFailure(t *testing.T) {
	r, tmp := newXLSXRunner(t, &testutil.FakeXLSXExtract{
		Code: 4,
		Err:  errors.New("forced extract failure"),
	})
	if err := os.WriteFile(filepath.Join(tmp, "raw", "inbox", "wb.xlsx"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	_, err := IngestXLSX(context.Background(), r, XLSXOptions{SourcePath: "raw/inbox/wb.xlsx"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected extract failure, got nil")
	}
	var ee *ingest.ExitError
	if !errors.As(err, &ee) || ee.Code != 4 {
		t.Fatalf("expected ExitError code 4 (helper rc), got %v", err)
	}
	if !strings.Contains(stderr.String(), "XLSX-ERROR|reason=extract|src=raw/inbox/wb.xlsx|rc=4") {
		t.Errorf("stderr missing extract failure record: %q", stderr.String())
	}
}
