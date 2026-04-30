package ops

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeOneShotBuild(t *testing.T) {
	dir := setupBuildRepo(t)
	// Pre-populate stale Hugo state to confirm Serve clears it.
	mustWrite(t, filepath.Join(dir, ".hugo_build.lock"), "stale")
	mustMkdir(t, filepath.Join(dir, "resources", "_gen"))
	mustWrite(t, filepath.Join(dir, "resources", "_gen", "old.txt"), "x")
	mustMkdir(t, filepath.Join(dir, "public"))
	mustWrite(t, filepath.Join(dir, "public", "stale.html"), "x")

	var stdout bytes.Buffer
	rc := Serve(ServeOptions{
		RepoRoot:    dir,
		SkipWatcher: true,
		SkipHugo:    true,
	}, &stdout, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d stdout=%s", rc, stdout.String())
	}
	if !strings.Contains(stdout.String(), "BUILD-OK|") {
		t.Fatalf("expected BUILD-OK in stdout: %s", stdout.String())
	}
	for _, p := range []string{".hugo_build.lock", "resources/_gen", "public"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err == nil {
			t.Fatalf("Serve did not clear %s", p)
		}
	}
}
