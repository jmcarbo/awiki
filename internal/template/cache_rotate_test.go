package template

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestRotateCache_NoDir(t *testing.T) {
	if err := RotateCache("/no/such/cache", "abcdef"); err != nil {
		t.Fatalf("RotateCache on missing dir should be no-op, got %v", err)
	}
}

func TestRotateCache_KeepsCurrentAndOnePrevious(t *testing.T) {
	dir := t.TempDir()
	mk := func(name string, mtime time.Time) {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		// Place a marker file inside so removal is observable on
		// platforms where empty-dir removal might short-circuit.
		if err := os.WriteFile(filepath.Join(p, "marker"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	mk("aaaa", now.Add(-72*time.Hour))
	mk("bbbb", now.Add(-48*time.Hour))
	mk("cccc", now.Add(-24*time.Hour)) // most recent
	mk("dddd", now.Add(-1*time.Hour))  // current
	// Underscore-prefixed dirs must always be preserved.
	mk("_fetch", now)

	if err := RotateCache(dir, "dddd"); err != nil {
		t.Fatalf("RotateCache: %v", err)
	}

	got := readDirNames(t, dir)
	want := []string{"_fetch", "cccc", "dddd"}
	if !equalStrings(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestRotateCache_CurrentMissing(t *testing.T) {
	// When current isn't in the dir, keep the most recent existing
	// non-current dir as the second slot.
	dir := t.TempDir()
	mk := func(name string, mtime time.Time) {
		p := filepath.Join(dir, name)
		_ = os.MkdirAll(p, 0o755)
		_ = os.Chtimes(p, mtime, mtime)
	}
	now := time.Now()
	mk("aaaa", now.Add(-3*time.Hour))
	mk("bbbb", now.Add(-1*time.Hour))
	if err := RotateCache(dir, "ZZZ"); err != nil {
		t.Fatal(err)
	}
	got := readDirNames(t, dir)
	want := []string{"bbbb"} // newest non-current is kept
	if !equalStrings(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func readDirNames(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range es {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
