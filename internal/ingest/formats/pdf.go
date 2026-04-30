// Package formats holds the per-format extractors invoked by the
// ingest orchestrator and by the dedicated `awiki ingest-<format>`
// verbs. Each extractor turns a source file into a markdown page on
// disk and emits the bash-parity status records on stdout.
package formats

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"awiki/internal/ingest"
)

// PDFOptions describes one `awiki ingest-pdf <path>` invocation.
//
// SourcePath is the user-supplied path to the PDF. It must be relative
// to RepoRoot and rooted at `raw/inbox/` (mirroring the bash guard in
// scripts/ingest-pdf.sh:9). Absolute paths under RepoRoot are also
// accepted and normalized to repo-relative form.
type PDFOptions struct {
	SourcePath string
}

// PDFResult is what the verb returns to the caller (CLI or
// orchestrator). DestPath and OriginalPath are repo-relative,
// matching the form bash emits in the PDF-CONVERTED record.
type PDFResult struct {
	SourcePath   string
	DestPath     string
	OriginalPath string
}

// IngestPDF ports `scripts/ingest-pdf.sh` to Go. The flow mirrors the
// bash byte-for-byte:
//
//  1. Validate the source has a .pdf extension, exists, and lives
//     under raw/inbox/.
//  2. Compute the sibling .md output path: `${PDF%.pdf}.md`.
//  3. Copy the source PDF into raw/processed/_originals/.
//  4. Extract text via the injected PDFToText adapter.
//  5. Prepend the canonical frontmatter (title / date / last_updated
//     / type / tags / aliases / sources / original / draft) and write
//     the result to the destination path.
//  6. Remove the source PDF from the inbox.
//  7. Emit two stdout records, byte-identical with bash:
//
//     PDF-CONVERTED|in=<src>|out=<dst>|orig=<orig>
//     Now run: just ingest <dst>
//
// On adapter failure the source PDF is left untouched and an
// ingest.ExitError{Code: 2} is returned (extractor failure exit code,
// per the design doc's exit-code contract).
//
// IngestPDF returns a PDFResult so the orchestrator (slice 7) can hand
// the rendered markdown path to the bookkeep step.
func IngestPDF(ctx context.Context, r *ingest.Runner, opts PDFOptions, stdout, stderr io.Writer) (*PDFResult, error) {
	src := opts.SourcePath
	if src == "" {
		fmt.Fprintln(stderr, "usage: ingest-pdf.sh <pdf-path-under-raw/inbox/>")
		return nil, &ingest.ExitError{Code: 1, Msg: "usage"}
	}

	// Normalize: if absolute and under RepoRoot, convert to repo-relative.
	if filepath.IsAbs(src) {
		if rel, err := filepath.Rel(r.RepoRoot, src); err == nil && !strings.HasPrefix(rel, "..") {
			src = filepath.ToSlash(rel)
		}
	} else {
		src = filepath.ToSlash(filepath.Clean(src))
	}

	if !strings.HasSuffix(src, ".pdf") {
		fmt.Fprintln(stderr, "not a pdf")
		return nil, &ingest.ExitError{Code: 1, Msg: "not a pdf"}
	}
	abs := filepath.Join(r.RepoRoot, src)
	if info, err := os.Stat(abs); err != nil || info.IsDir() {
		fmt.Fprintf(stderr, "not found: %s\n", src)
		return nil, &ingest.ExitError{Code: 1, Msg: "not found"}
	}
	if !strings.HasPrefix(src, "raw/inbox/") {
		fmt.Fprintln(stderr, "must be under raw/inbox/")
		return nil, &ingest.ExitError{Code: 1, Msg: "must be under raw/inbox/"}
	}

	// Compute sibling .md path. Bash uses `${PDF%.pdf}.md` which is a
	// suffix swap; we mirror that.
	dst := strings.TrimSuffix(src, ".pdf") + ".md"
	dstAbs := filepath.Join(r.RepoRoot, dst)

	// Stash original under raw/processed/_originals/<basename>.
	origDir := "raw/processed/_originals"
	origName := filepath.Base(src)
	orig := origDir + "/" + origName
	origAbs := filepath.Join(r.RepoRoot, orig)
	if err := os.MkdirAll(filepath.Join(r.RepoRoot, origDir), 0o755); err != nil {
		return nil, fmt.Errorf("ingest-pdf: mkdir originals: %w", err)
	}
	if err := copyFile(abs, origAbs); err != nil {
		return nil, fmt.Errorf("ingest-pdf: stash original: %w", err)
	}

	// Extract text via the adapter. Bash falls back to `marker` when
	// pdftotext is missing; the parity here is "use the configured
	// adapter". Callers wanting marker should set
	// AWIKI_INGEST_PDF_LEGACY=1 to fall back to the bash script (which
	// retains the marker branch).
	if r.PDFToText == nil {
		return nil, fmt.Errorf("ingest-pdf: PDFToText adapter is nil")
	}
	text, code, err := r.PDFToText.Extract(ctx, abs)
	if err != nil || code != 0 {
		fmt.Fprintf(stderr, "ingest-pdf: pdftotext failed (exit %d): %v\n", code, err)
		return nil, &ingest.ExitError{Code: 2, Msg: "pdftotext failed"}
	}

	// Build the frontmatter+body. Bash uses `date '+%Y-%m-%d'` for both
	// `date` and `last_updated`; in Go we read Runner.Today (set by the
	// CLI via todayDate, overridable by tests).
	today := r.Today
	if today == "" {
		today = todayFallback(r)
	}
	baseTitle := strings.TrimSuffix(filepath.Base(src), ".pdf")
	fm := buildFrontmatter(baseTitle, today, orig)
	body := fm + text

	// Ensure the destination directory exists. The bash form lets the
	// caller's existing layout do the work; here we mkdir defensively
	// since callers may target a fresh inbox subdir.
	if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
		return nil, fmt.Errorf("ingest-pdf: mkdir dest: %w", err)
	}
	if err := os.WriteFile(dstAbs, []byte(body), 0o644); err != nil {
		return nil, fmt.Errorf("ingest-pdf: write dest: %w", err)
	}

	// Remove the source PDF from the inbox (bash: `rm "$PDF"`).
	if err := os.Remove(abs); err != nil {
		return nil, fmt.Errorf("ingest-pdf: remove source: %w", err)
	}

	// Emit the bash-parity records. scripts/ingest-pdf.sh:35-36.
	fmt.Fprintf(stdout, "PDF-CONVERTED|in=%s|out=%s|orig=%s\n", src, dst, orig)
	fmt.Fprintf(stdout, "Now run: just ingest %s\n", dst)

	return &PDFResult{
		SourcePath:   src,
		DestPath:     dst,
		OriginalPath: orig,
	}, nil
}

// buildFrontmatter mirrors the bash printf at scripts/ingest-pdf.sh:27-28
// byte-for-byte. The bash form is:
//
//	---
//	title: "<base>"
//	date: <today>
//	last_updated: <today>
//	type: source
//	tags: [pdf]
//	aliases: []
//	sources: []
//	original: <orig>
//	draft: false
//	---
//	<blank line>
func buildFrontmatter(title, today, orig string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("title: \"%s\"\n", title))
	b.WriteString(fmt.Sprintf("date: %s\n", today))
	b.WriteString(fmt.Sprintf("last_updated: %s\n", today))
	b.WriteString("type: source\n")
	b.WriteString("tags: [pdf]\n")
	b.WriteString("aliases: []\n")
	b.WriteString("sources: []\n")
	b.WriteString(fmt.Sprintf("original: %s\n", orig))
	b.WriteString("draft: false\n")
	b.WriteString("---\n\n")
	return b.String()
}

// todayFallback returns today's date when r.Today is empty. Kept as a
// separate helper so tests can wire deterministic clocks via r.Today
// without dragging time imports into the main path.
func todayFallback(r *ingest.Runner) string {
	if r.NowFn != nil {
		return r.NowFn().Format("2006-01-02")
	}
	return ""
}

// copyFile copies src -> dst preserving mode. Mirrors bash `cp`.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}
