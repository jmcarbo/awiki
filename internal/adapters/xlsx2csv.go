package adapters

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// XLSXExtractOptions carries the arguments scripts/ingest-xlsx.sh passes
// to scripts/lib/xlsx-extract.py for a workbook extraction. Field order
// matches the bash invocation at scripts/ingest-xlsx.sh:82-89.
type XLSXExtractOptions struct {
	// In is the absolute path to the workbook on disk (xlsx/xls/ods).
	In string
	// OutDir is the filesystem dir where per-sheet markdown is written.
	// Bash sets it to dirname($SRC).
	OutDir string
	// CSVDir is the filesystem dir where per-sheet CSVs are written.
	// Bash sets it to raw/processed/_originals/<slug>.
	CSVDir string
	// CSVRel is the repo-relative csv dir written into the per-sheet
	// frontmatter `csv:` field. Same value as CSVDir for v1 callers.
	CSVRel string
	// OriginalRel is the repo-relative path to the archived workbook
	// copy, written into the per-sheet frontmatter `original:` field.
	OriginalRel string
	// SlugPrefix is the workbook slug used to namespace sheet slugs.
	// Bash computes it via the helper's --slugify mode.
	SlugPrefix string
	// PreviewRows controls how many rows the per-sheet markdown preview
	// table renders. Bash defaults to AWIKI_XLSX_PREVIEW_ROWS or 50.
	PreviewRows int
}

// XLSXExtract is the adapter that drives scripts/lib/xlsx-extract.py.
//
// The Python helper is intentionally not ported to Go in v1 — see the
// 2026-04-30 ingest slice plan, "Pinned open questions" section
// (xlsx implementation stays exec). This interface captures the two
// operations the bash script performs against the helper:
//
//   - Slugify converts a stem (typically the workbook basename without
//     extension) into the kebab-cased slug used as the workbook prefix
//     and as the basename of the originals directory. Bash invokes this
//     at scripts/ingest-xlsx.sh:71.
//   - Extract performs the per-sheet extraction, returning the helper's
//     single-line JSON manifest on stdout. Bash invokes it at
//     scripts/ingest-xlsx.sh:82-89.
//
// Both methods return the helper's exit code so callers can mirror the
// bash error path that surfaces `rc=<code>` in the XLSX-ERROR record.
type XLSXExtract interface {
	Slugify(ctx context.Context, stem string) (slug string, code int, err error)
	Extract(ctx context.Context, opts XLSXExtractOptions) (manifestJSON string, code int, err error)
	// CheckDep verifies the python_calamine module is importable.
	// Mirrors the bash preflight at scripts/ingest-xlsx.sh:62-67. Tests
	// honour AWIKI_FAKE_MISSING=python_calamine to force the miss path.
	CheckDep(ctx context.Context) (ok bool, err error)
}

// ExecXLSXExtract is the production adapter. It shells
// `python3 <RepoRoot>/scripts/lib/xlsx-extract.py <args>` for both
// methods, mirroring the bash script byte-for-byte.
//
// RepoRoot must be the absolute path to the repository root so the
// helper can be located irrespective of the caller's cwd. The CLI
// dispatcher resolves it (AWIKI_REPO_ROOT or `git rev-parse
// --show-toplevel`) and injects it when constructing the runner.
type ExecXLSXExtract struct {
	RepoRoot string
}

// helperPath returns the absolute path to scripts/lib/xlsx-extract.py.
func (e ExecXLSXExtract) helperPath() string {
	return filepath.Join(e.RepoRoot, "scripts", "lib", "xlsx-extract.py")
}

// CheckDep returns true when `python3 -c "import python_calamine"`
// succeeds. The AWIKI_FAKE_MISSING shortcut lets tests force the
// missing-dep code path without touching the real python install.
// Mirrors scripts/ingest-xlsx.sh:62-67.
func (e ExecXLSXExtract) CheckDep(ctx context.Context) (bool, error) {
	if os.Getenv("AWIKI_FAKE_MISSING") == "python_calamine" {
		return false, nil
	}
	cmd := exec.CommandContext(ctx, "python3", "-c", "import python_calamine")
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

// Slugify shells `python3 <helper> --slugify <stem>` and returns the
// helper's stdout (with the trailing newline stripped). The helper is
// pure: an all-non-alnum input returns an empty slug, which the caller
// must validate before promoting it to a path component.
func (e ExecXLSXExtract) Slugify(ctx context.Context, stem string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "python3", e.helperPath(), "--slugify", stem)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return strings.TrimRight(stdout.String(), "\n"), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		msg := stderr.String()
		if msg == "" {
			msg = stdout.String()
		}
		return strings.TrimRight(stdout.String(), "\n"), exitErr.ExitCode(), errors.New(msg)
	}
	return "", 127, err
}

// Extract shells `python3 <helper> --in ... --out-dir ... ...` matching
// the bash invocation at scripts/ingest-xlsx.sh:82-89 and returns the
// helper's single-line JSON manifest on stdout.
func (e ExecXLSXExtract) Extract(ctx context.Context, opts XLSXExtractOptions) (string, int, error) {
	args := []string{
		e.helperPath(),
		"--in", opts.In,
		"--out-dir", opts.OutDir,
		"--csv-dir", opts.CSVDir,
		"--csv-rel", opts.CSVRel,
		"--original-rel", opts.OriginalRel,
		"--slug-prefix", opts.SlugPrefix,
		"--preview-rows", strconv.Itoa(opts.PreviewRows),
	}
	cmd := exec.CommandContext(ctx, "python3", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		msg := stderr.String()
		if msg == "" {
			msg = stdout.String()
		}
		return stdout.String(), exitErr.ExitCode(), errors.New(msg)
	}
	return "", 127, err
}
