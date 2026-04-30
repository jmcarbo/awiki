package template

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// repoRootForTest walks up from the test source file to find the
// repo root (the directory containing scripts/templates/). The test
// binary cwd is the package dir so we can't rely on relative paths.
func repoRootForTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file is .../internal/template/payloads_test.go
	dir := filepath.Dir(file)
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "scripts", "templates")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not locate scripts/templates from test file")
	return ""
}

func TestPayloadBytes_MatchesSource(t *testing.T) {
	root := repoRootForTest(t)
	cases := []struct {
		id   PayloadID
		file string
	}{
		{PayloadDataLayer, "wiki-data-layer.md"},
		{PayloadTaskLayer, "wiki-task-layer.md"},
		{PayloadWeeklyReview, "wiki-weekly-review.md"},
	}
	for _, tc := range cases {
		t.Run(string(tc.id), func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join(root, "scripts", "templates", tc.file))
			if err != nil {
				t.Fatalf("read source: %v", err)
			}
			got, err := PayloadBytes(tc.id)
			if err != nil {
				t.Fatalf("PayloadBytes: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("byte mismatch for %s (embed=%d source=%d)", tc.id, len(got), len(want))
			}
		})
	}
}

func TestPayloadBytes_Unknown(t *testing.T) {
	if _, err := PayloadBytes("does-not-exist"); err == nil {
		t.Fatal("expected error for unknown payload id")
	}
}

func TestAllPayloadIDs_MatchKnownConstants(t *testing.T) {
	ids := AllPayloadIDs()
	if len(ids) != 3 {
		t.Fatalf("expected 3 payload ids, got %d", len(ids))
	}
	want := map[PayloadID]bool{
		PayloadDataLayer:    true,
		PayloadTaskLayer:    true,
		PayloadWeeklyReview: true,
	}
	for _, id := range ids {
		if !want[id] {
			t.Errorf("unexpected payload id: %s", id)
		}
		delete(want, id)
	}
	if len(want) > 0 {
		t.Errorf("missing ids: %v", want)
	}
}

func TestPayloadFS_HasAllFiles(t *testing.T) {
	fsys := PayloadFS()
	for _, id := range AllPayloadIDs() {
		f, err := fsys.Open(string(id) + ".md")
		if err != nil {
			t.Errorf("open %s: %v", id, err)
			continue
		}
		f.Close()
	}
}
