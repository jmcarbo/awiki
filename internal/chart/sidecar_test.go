package chart

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndReadSidecars(t *testing.T) {
	dir := t.TempDir()
	spec := map[string]any{"mark": "bar"}
	svg := []byte("<svg>hello</svg>")
	if err := WriteSidecars(dir, "my-chart", spec, svg); err != nil {
		t.Fatalf("WriteSidecars: %v", err)
	}
	// Verify files exist.
	for _, f := range []string{"my-chart.json", "my-chart.svg", "my-chart.svg.hash"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("expected file %s to exist: %v", f, err)
		}
	}
	// ReadHash should return the hash and ok=true.
	h, ok := ReadHash(dir, "my-chart")
	if !ok {
		t.Fatal("ReadHash returned ok=false")
	}
	expected := Hash(spec)
	if h != expected {
		t.Errorf("hash mismatch: got %q want %q", h, expected)
	}
}

func TestReadHash_Absent(t *testing.T) {
	dir := t.TempDir()
	h, ok := ReadHash(dir, "nonexistent")
	if ok {
		t.Error("want ok=false for absent hash file")
	}
	if h != "" {
		t.Errorf("want empty hash, got %q", h)
	}
}

func TestWriteFailed_Truncated(t *testing.T) {
	dir := t.TempDir()
	longMsg := string(make([]byte, 300))
	for i := range 300 {
		longMsg = longMsg[:i] + "x" + longMsg[i+1:]
	}
	if err := WriteFailed(dir, "chart-id", longMsg); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "chart-id.svg.failed"))
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > 200 {
		t.Errorf("expected at most 200 bytes, got %d", len(b))
	}
}

func TestCleanup_RemovesOrphans(t *testing.T) {
	dir := t.TempDir()
	// Create sidecars for two charts.
	for _, name := range []string{"keep.svg", "keep.svg.hash", "keep.json", "orphan.svg", "orphan.svg.hash", "orphan.json", "orphan.svg.failed"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	keep := map[string]struct{}{"keep": {}}
	removed, err := Cleanup(dir, keep)
	if err != nil {
		t.Fatalf("Cleanup error: %v", err)
	}
	if len(removed) != 4 {
		t.Errorf("expected 4 orphan files removed, got %d: %v", len(removed), removed)
	}
	// keep files should still exist.
	for _, f := range []string{"keep.svg", "keep.svg.hash", "keep.json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("keep file should exist: %s", f)
		}
	}
}

func TestCleanup_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	removed, err := Cleanup(dir, map[string]struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 0 {
		t.Errorf("expected no files removed from empty dir, got %d", len(removed))
	}
}

func TestCleanup_NonexistentDir(t *testing.T) {
	removed, err := Cleanup("/nonexistent/path/xyz", map[string]struct{}{})
	if err != nil {
		t.Fatal("should not error for nonexistent dir")
	}
	if len(removed) != 0 {
		t.Errorf("expected 0 removed, got %d", len(removed))
	}
}
