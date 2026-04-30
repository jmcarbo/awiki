package template

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// RotateCache prunes orphaned per-commit cache directories under
// cacheDir. It mirrors `cache_rotate.py`:
//   - directories whose name starts with "_" are ignored (e.g. "_fetch").
//   - the directory named current is always kept.
//   - one additional non-current directory is kept (the most recent by
//     mtime).
//   - all remaining sha-named directories are removed.
//
// A missing or non-directory cacheDir is a no-op (matches Python).
func RotateCache(cacheDir, current string) error {
	info, err := os.Stat(cacheDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return err
	}

	type dirEntry struct {
		name  string
		mtime int64
	}
	var dirs []dirEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), "_") {
			continue
		}
		fi, err := os.Stat(filepath.Join(cacheDir, e.Name()))
		if err != nil {
			// Skip dirs we can't stat — `Path.stat()` would also raise,
			// but the oracle's iterdir + stat order matches our Stat
			// here.
			continue
		}
		dirs = append(dirs, dirEntry{name: e.Name(), mtime: fi.ModTime().UnixNano()})
	}

	// Sort by mtime descending (newest first) — match Python's
	// `sort(key=lambda d: d.stat().st_mtime, reverse=True)`.
	sort.SliceStable(dirs, func(i, j int) bool {
		return dirs[i].mtime > dirs[j].mtime
	})

	keep := []string{current}
	// Add up to 1 most recent that isn't current.
	for _, d := range dirs {
		if !slices.Contains(keep, d.name) && len(keep) < 2 {
			keep = append(keep, d.name)
			break
		}
	}

	for _, d := range dirs {
		if slices.Contains(keep, d.name) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(cacheDir, d.name)); err != nil {
			// Mirror shutil.rmtree(..., ignore_errors=True): swallow.
			continue
		}
	}
	return nil
}
