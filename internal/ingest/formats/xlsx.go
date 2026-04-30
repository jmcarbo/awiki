package formats

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"awiki/internal/adapters"
	"awiki/internal/ingest"
)

// XLSXOptions describes one `awiki ingest-xlsx <path> [flags]` invocation.
//
// SourcePath is the user-supplied path to the workbook. It must resolve
// to a repo-relative path under raw/inbox/ ending in .xlsx, .xls, or
// .ods (mirrors scripts/ingest-xlsx.sh:30,49-59). PreviewRows controls
// the number of rows shown in the per-sheet markdown preview table;
// the bash default is 50, overridable by --preview-rows or by
// AWIKI_XLSX_PREVIEW_ROWS.
type XLSXOptions struct {
	SourcePath  string
	PreviewRows int
}

// XLSXResult is what the verb returns to the caller. WorkbookSlug is
// the kebab-cased stem produced by the helper's --slugify mode.
// OriginalPath is the repo-relative path to the archived workbook
// copy. Sheets enumerates per-sheet md/csv pairs in manifest order.
type XLSXResult struct {
	SourcePath   string
	WorkbookSlug string
	OriginalPath string
	Sheets       []XLSXSheetResult
}

// XLSXSheetResult mirrors one entry of the helper's manifest "sheets"
// array. Slug is namespaced as "<workbook-slug>--<sheet-slug>"; bash
// emits this as the `slug=` field in the XLSX-SHEET record.
type XLSXSheetResult struct {
	Name      string
	Slug      string
	RowsTotal int
	MDPath    string // repo-relative
	CSVPath   string // repo-relative
}

// xlsxManifest is the JSON shape the python helper prints to stdout
// (see scripts/lib/xlsx-extract.py:290-291).
type xlsxManifest struct {
	WorkbookSlug string             `json:"workbook_slug"`
	Sheets       []xlsxManifestItem `json:"sheets"`
}

type xlsxManifestItem struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	RowsTotal   int    `json:"rows_total"`
	RowsPreview int    `json:"rows_preview"`
	CSV         string `json:"csv"`
	MD          string `json:"md"`
}

// IngestXLSX ports `scripts/ingest-xlsx.sh` to Go. The flow mirrors the
// bash byte-for-byte:
//
//  1. Validate the source has a .xlsx/.xls/.ods extension, exists,
//     lives under raw/inbox/, and contains no `/../` traversal segment.
//  2. Preflight the python_calamine dep (honour AWIKI_FAKE_MISSING for
//     tests; emit XLSX-ERROR|reason=missing-dep on miss, exit 5).
//  3. Slugify the basename stem via the XLSXExtract adapter.
//  4. Stash the original workbook to raw/processed/_originals/<slug>/.
//  5. Drive XLSXExtract.Extract; parse the manifest JSON.
//  6. Per-sheet: emit XLSX-SHEET|slug=...|rows=...|md=...|csv=... .
//  7. Remove the source workbook (bash: `rm "$SRC"`).
//  8. Emit XLSX-CONVERTED|in=...|workbook=...|sheets=N|orig=... .
//  9. Per-sheet: emit XLSX-NEXT|<md-rel> .
//
// Exit codes match the bash oracle:
//   - 0 success
//   - 1 not-found
//   - 2 usage / bad-path / bad-ext / bad-slug / bad-manifest
//   - 5 missing-dep (python-calamine)
//   - 6 empty workbook (zero sheets)
//   - <rc> when the helper returns non-zero on extraction
//
// The bash log-append calls (scripts/ingest-xlsx.sh:93,102,107,131) are
// best-effort with `2>/dev/null || true`. v1 simply skips them; the
// ops/log domain port (slice 7+) will reintroduce them via a Logger
// adapter once the byte format is pinned.
func IngestXLSX(ctx context.Context, r *ingest.Runner, opts XLSXOptions, stdout, stderr io.Writer) (*XLSXResult, error) {
	src := opts.SourcePath
	if src == "" {
		fmt.Fprintln(stderr, "XLSX-ERROR|reason=usage")
		return nil, &ingest.ExitError{Code: 2, Msg: "usage"}
	}

	// Normalize to repo-relative form. Mirrors scripts/ingest-xlsx.sh:33-47.
	if filepath.IsAbs(src) {
		rel, err := filepath.Rel(r.RepoRoot, src)
		if err != nil || strings.HasPrefix(rel, "..") {
			fmt.Fprintf(stderr, "XLSX-ERROR|reason=bad-path|src=%s\n", src)
			return nil, &ingest.ExitError{Code: 2, Msg: "bad-path"}
		}
		src = filepath.ToSlash(rel)
	} else {
		src = strings.TrimPrefix(src, "./")
	}

	// Reject any traversal segment so `raw/inbox/../etc/passwd` does
	// not satisfy the textual `raw/inbox/*` gate. Bash uses a slash-
	// padded match (`*/../*`); we mirror it on the raw input rather
	// than on filepath.Clean to keep byte-parity with the error path.
	if strings.Contains("/"+src+"/", "/../") {
		fmt.Fprintf(stderr, "XLSX-ERROR|reason=bad-path|src=%s\n", src)
		return nil, &ingest.ExitError{Code: 2, Msg: "bad-path"}
	}

	if !strings.HasPrefix(src, "raw/inbox/") {
		fmt.Fprintf(stderr, "XLSX-ERROR|reason=bad-path|src=%s\n", src)
		return nil, &ingest.ExitError{Code: 2, Msg: "bad-path"}
	}

	abs := filepath.Join(r.RepoRoot, src)
	if info, err := os.Stat(abs); err != nil || info.IsDir() {
		fmt.Fprintf(stderr, "XLSX-ERROR|reason=not-found|src=%s\n", src)
		return nil, &ingest.ExitError{Code: 1, Msg: "not-found"}
	}

	lowered := strings.ToLower(src)
	if !(strings.HasSuffix(lowered, ".xlsx") ||
		strings.HasSuffix(lowered, ".xls") ||
		strings.HasSuffix(lowered, ".ods")) {
		fmt.Fprintf(stderr, "XLSX-ERROR|reason=bad-ext|src=%s\n", src)
		return nil, &ingest.ExitError{Code: 2, Msg: "bad-ext"}
	}

	if r.XLSXExtract == nil {
		return nil, fmt.Errorf("ingest-xlsx: XLSXExtract adapter is nil")
	}

	// Preflight: python_calamine. Honour AWIKI_FAKE_MISSING for tests.
	// Mirrors scripts/ingest-xlsx.sh:62-67.
	if ok, _ := r.XLSXExtract.CheckDep(ctx); !ok {
		fmt.Fprintln(stderr, "XLSX-ERROR|reason=missing-dep|dep=python-calamine")
		fmt.Fprintln(stderr, "  install: pip3 install python-calamine")
		return nil, &ingest.ExitError{Code: 5, Msg: "missing-dep"}
	}

	base := filepath.Base(src)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	slug, code, err := r.XLSXExtract.Slugify(ctx, stem)
	if err != nil {
		fmt.Fprintf(stderr, "ingest-xlsx: slugify failed (exit %d): %v\n", code, err)
		return nil, &ingest.ExitError{Code: 2, Msg: "slugify-failed"}
	}
	if slug == "" {
		fmt.Fprintf(stderr, "XLSX-ERROR|reason=bad-slug|src=%s|stem=%s\n", src, stem)
		return nil, &ingest.ExitError{Code: 2, Msg: "bad-slug"}
	}

	origDir := "raw/processed/_originals/" + slug
	origDirAbs := filepath.Join(r.RepoRoot, origDir)
	if err := os.MkdirAll(origDirAbs, 0o755); err != nil {
		return nil, fmt.Errorf("ingest-xlsx: mkdir originals: %w", err)
	}
	origRel := origDir + "/" + base
	origAbs := filepath.Join(r.RepoRoot, origRel)
	if err := copyFile(abs, origAbs); err != nil {
		return nil, fmt.Errorf("ingest-xlsx: stash original: %w", err)
	}

	outDirRel := filepath.ToSlash(filepath.Dir(src)) // e.g. raw/inbox
	csvDirRel := origDir

	previewRows := opts.PreviewRows
	if previewRows <= 0 {
		previewRows = 50
	}

	// Pass absolute paths into the adapter. Bash runs the helper with
	// cwd = RepoRoot so its repo-relative paths resolve correctly; we
	// avoid the chdir by absolutizing here. The CSVRel and OriginalRel
	// fields stay repo-relative because they are written verbatim into
	// the per-sheet frontmatter.
	manifestStr, code, err := r.XLSXExtract.Extract(ctx, adapters.XLSXExtractOptions{
		In:          abs,
		OutDir:      filepath.Join(r.RepoRoot, outDirRel),
		CSVDir:      filepath.Join(r.RepoRoot, csvDirRel),
		CSVRel:      origDir,
		OriginalRel: origRel,
		SlugPrefix:  slug,
		PreviewRows: previewRows,
	})
	if err != nil || code != 0 {
		fmt.Fprintf(stderr, "XLSX-ERROR|reason=extract|src=%s|rc=%d\n", src, code)
		// Bash uses `exit "$EXTRACT_RC"`. Mirror by surfacing rc.
		exitCode := code
		if exitCode == 0 {
			exitCode = 1
		}
		return nil, &ingest.ExitError{Code: exitCode, Msg: "extract"}
	}

	manifest := xlsxManifest{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(manifestStr)), &manifest); err != nil {
		fmt.Fprintf(stderr, "XLSX-ERROR|reason=bad-manifest|src=%s|rc=1\n", src)
		return nil, &ingest.ExitError{Code: 1, Msg: "bad-manifest"}
	}

	if len(manifest.Sheets) == 0 {
		fmt.Fprintf(stdout, "XLSX-EMPTY|workbook=%s|src=%s\n", slug, src)
		return nil, &ingest.ExitError{Code: 6, Msg: "empty"}
	}

	// Per-sheet XLSX-SHEET emission BEFORE rm. Mirrors
	// scripts/ingest-xlsx.sh:112-119.
	sheets := make([]XLSXSheetResult, 0, len(manifest.Sheets))
	for _, s := range manifest.Sheets {
		mdRel := relpathFromRoot(r.RepoRoot, s.MD)
		csvRel := relpathFromRoot(r.RepoRoot, s.CSV)
		fmt.Fprintf(stdout, "XLSX-SHEET|slug=%s|rows=%d|md=%s|csv=%s\n",
			s.Slug, s.RowsTotal, mdRel, csvRel)
		sheets = append(sheets, XLSXSheetResult{
			Name:      s.Name,
			Slug:      s.Slug,
			RowsTotal: s.RowsTotal,
			MDPath:    mdRel,
			CSVPath:   csvRel,
		})
	}

	if err := os.Remove(abs); err != nil {
		return nil, fmt.Errorf("ingest-xlsx: remove source: %w", err)
	}

	// XLSX-CONVERTED AFTER rm. Mirrors scripts/ingest-xlsx.sh:123.
	fmt.Fprintf(stdout, "XLSX-CONVERTED|in=%s|workbook=%s|sheets=%d|orig=%s\n",
		src, slug, len(manifest.Sheets), origRel)

	// Per-sheet XLSX-NEXT lines. Mirrors scripts/ingest-xlsx.sh:124-129.
	for _, s := range sheets {
		fmt.Fprintf(stdout, "XLSX-NEXT|%s\n", s.MDPath)
	}

	// TODO(slice 7+ ops port): reintroduce log-append calls via a
	// Logger adapter. The bash form swallows errors with
	// `2>/dev/null || true`, so absent log lines are not a parity
	// violation today.

	return &XLSXResult{
		SourcePath:   src,
		WorkbookSlug: slug,
		OriginalPath: origRel,
		Sheets:       sheets,
	}, nil
}

// relpathFromRoot mirrors python's os.path.relpath when the bash cwd
// is the repo root. Manifest paths are usually already repo-relative
// (the helper writes them as <out_dir>/<sheet_slug>.md and bash passes
// repo-relative out_dir), so this normalizes any absolute path that
// tests may pass and otherwise returns the input cleaned.
func relpathFromRoot(repoRoot, p string) string {
	if filepath.IsAbs(p) {
		if rel, err := filepath.Rel(repoRoot, p); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(filepath.Clean(p))
}

