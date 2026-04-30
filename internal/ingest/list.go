package ingest

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ListBatch prints every regular file under `raw/inbox/batch/` (recursive)
// to stdout, sorted lexicographically, one per line. Each path is emitted
// as a repo-relative path beginning with `raw/inbox/batch/...` — byte-
// identical with the bash form:
//
//	find raw/inbox/batch -type f | sort
//
// when `just ingest-batch-list` is invoked from RepoRoot.
//
// If the batch directory does not exist this returns nil with no output.
// (The bash `find` errors when the directory is missing, but in practice
// callers invoke this only after watchdog has created the tree, and the
// justfile recipe runs `find` directly with no `2>/dev/null`. Mirroring
// the tolerant behavior here matches user expectation when the dir is
// genuinely absent — and matches the existing ListGit tolerance.)
func (r *Runner) ListBatch(stdout io.Writer) error {
	root := r.BatchDir()
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("ingest-batch-list: stat %s: %w", root, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("ingest-batch-list: %s is not a directory", root)
	}

	var paths []string
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Mirror `find -type f`: only regular files (skip symlinks et al
		// in the same way `find` does — `find` follows mode, and a
		// symlink to a regular file does not match `-type f` unless `-L`
		// is passed). filepath.Walk reports the symlink's lstat info, so
		// `info.Mode().IsRegular()` is the right test.
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(r.RepoRoot, path)
		if err != nil {
			return err
		}
		// Use forward slashes regardless of platform (matches bash output
		// on the supported macOS/linux targets). filepath.ToSlash is a
		// no-op on Unix; defensive on other platforms.
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		return fmt.Errorf("ingest-batch-list: walk %s: %w", root, err)
	}

	sort.Strings(paths)
	for _, p := range paths {
		fmt.Fprintln(stdout, p)
	}
	return nil
}

// ListGit prints every entry in `.awiki/git-state/`, with the trailing
// `.json` suffix stripped, to stdout, sorted lexicographically, one per
// line. Byte-identical with the bash form:
//
//	ls -1 .awiki/git-state/ 2>/dev/null | sed 's/\.json$//'
//
// when invoked from RepoRoot.
//
// If the directory does not exist this returns nil with no output —
// matching the bash `2>/dev/null` swallow.
func (r *Runner) ListGit(stdout io.Writer) error {
	root := r.GitStateDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("ingest-git-list: read %s: %w", root, err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		// `ls -1` lists every entry name (files, dirs, symlinks). We
		// mirror that and let `sed 's/\.json$//'` strip the suffix when
		// present. Non-`.json` entries pass through unchanged.
		names = append(names, strings.TrimSuffix(e.Name(), ".json"))
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintln(stdout, n)
	}
	return nil
}
