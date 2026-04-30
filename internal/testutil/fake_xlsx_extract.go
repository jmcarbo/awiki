package testutil

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"awiki/internal/adapters"
)

// FakeXLSXExtract is a test double for adapters.XLSXExtract. It does
// not invoke python3 or python_calamine; instead Slugify computes the
// slug in-process and Extract returns canned manifest JSON, optionally
// staging canned per-sheet markdown + csv files into the configured
// out-dir / csv-dir so fixture tests can verify the on-disk artifacts
// the bash oracle would have produced.
//
// One of ManifestJSON or ManifestPath must be set when the test plans
// to drive Extract. ManifestPath wins when both are non-empty; the file
// is read on every Extract call so fixtures can share the on-disk test
// tree. Code/Err let tests simulate adapter failures (mirrors
// FakePDFToText / FakeWhisper).
//
// Files keyed by repo-relative destination path. The fake writes each
// pair to opts.OutDir (for `.md`) or opts.CSVDir (for `.csv`) on
// Extract calls, matching how the real helper stages per-sheet outputs.
// Keys are relative paths; the basename's extension determines which
// destination directory the fake writes into.
type FakeXLSXExtract struct {
	// SlugMap allows tests to override the Slugify result for specific
	// stems. Lookups fall back to the trivial computeSlug if the stem
	// is absent from the map.
	SlugMap map[string]string

	// ManifestJSON is returned verbatim from Extract (with a trailing
	// newline appended to match the real helper's print(...)).
	ManifestJSON string
	// ManifestPath, when non-empty, supersedes ManifestJSON.
	ManifestPath string

	// Files maps a basename (e.g. "sample-sheet-1.md") to the bytes
	// the fake writes into opts.OutDir / opts.CSVDir. The destination
	// dir is chosen by the file extension (.md -> OutDir, .csv ->
	// CSVDir). The fake mirrors the real helper's behavior of
	// "produce the per-sheet artifacts on disk".
	Files map[string][]byte

	// Code and Err override Extract's successful return when set.
	// Slugify ignores them — tests that need to simulate slugify
	// failures should set SlugMap (or use a stem the trivial slugifier
	// reduces to ""). Mirrors how the bash oracle treats slugify and
	// extract as separate helper invocations: failing the second does
	// not retroactively fail the first.
	Code int
	Err  error

	// DepMissing makes CheckDep return (false, nil). Mirrors the
	// AWIKI_FAKE_MISSING=python_calamine env-var path on the production
	// adapter.
	DepMissing bool
}

// CheckDep returns whether python-calamine is "available" in the test
// scenario. Tests set DepMissing=true to exercise the missing-dep
// branch without touching env vars.
func (f *FakeXLSXExtract) CheckDep(_ context.Context) (bool, error) {
	if f.DepMissing {
		return false, nil
	}
	return true, nil
}

var fakeXLSXSlugRe = regexp.MustCompile(`[^a-z0-9-]+`)

// computeSlug mirrors scripts/lib/xlsx-extract.py:slugify enough for
// the simple test cases. Lowercase, non-alnum -> '-', collapse runs,
// strip leading/trailing '-'. Returns "" for empty / all-non-alnum
// input, matching the helper's contract.
func computeSlug(text string) string {
	s := strings.ToLower(text)
	s = fakeXLSXSlugRe.ReplaceAllString(s, "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

// Slugify returns the canned slug for stem if present in SlugMap,
// otherwise the in-process slugify. Slugify never fails — tests that
// want a "bad slug" path use a stem that the trivial slugifier reduces
// to "" (e.g. "---") rather than wiring a separate failure mode.
func (f *FakeXLSXExtract) Slugify(_ context.Context, stem string) (string, int, error) {
	if f.SlugMap != nil {
		if v, ok := f.SlugMap[stem]; ok {
			return v, 0, nil
		}
	}
	return computeSlug(stem), 0, nil
}

// Extract writes the canned per-sheet files into opts.OutDir /
// opts.CSVDir and returns the canned manifest JSON. Mirrors the real
// helper's "produce on-disk artifacts + emit single-line manifest on
// stdout" contract.
func (f *FakeXLSXExtract) Extract(_ context.Context, opts adapters.XLSXExtractOptions) (string, int, error) {
	if f.Err != nil || f.Code != 0 {
		return "", f.Code, f.Err
	}

	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return "", 1, err
	}
	if err := os.MkdirAll(opts.CSVDir, 0o755); err != nil {
		return "", 1, err
	}

	for name, body := range f.Files {
		ext := filepath.Ext(name)
		var dst string
		switch ext {
		case ".md":
			dst = filepath.Join(opts.OutDir, name)
		case ".csv":
			dst = filepath.Join(opts.CSVDir, name)
		default:
			return "", 1, errors.New("FakeXLSXExtract: unsupported file extension " + ext)
		}
		if err := os.WriteFile(dst, body, 0o644); err != nil {
			return "", 1, err
		}
	}

	if f.ManifestPath != "" {
		data, err := os.ReadFile(f.ManifestPath)
		if err != nil {
			return "", 1, err
		}
		out := string(data)
		if !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		return out, 0, nil
	}
	out := f.ManifestJSON
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out, 0, nil
}

// Compile-time interface check.
var _ adapters.XLSXExtract = (*FakeXLSXExtract)(nil)
