// Package testutil provides shared fixture helpers for awiki tests.
package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CopyTree copies src into dst recursively. dst must not exist.
func CopyTree(t testing.TB, src, dst string) {
	t.Helper()
	if err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	}); err != nil {
		t.Fatalf("CopyTree: %v", err)
	}
}

// ReadFixtureLines loads a file as line-separated strings. Trailing
// newline is preserved or omitted as on disk.
func ReadFixtureLines(t testing.TB, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		return nil
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}
