package ops

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeMover bypasses git so unit tests don't depend on a real repo.
// Falls through to a plain rename, mirroring the bash fallback path.
type fakeMover struct{}

func (fakeMover) Move(ctx context.Context, repoRoot, src, dst string) error {
	return os.Rename(src, dst)
}

type fakeLogger struct {
	calls []string
}

func (f *fakeLogger) Append(ctx context.Context, repoRoot, action, msg string) error {
	f.calls = append(f.calls, action+"|"+msg)
	return nil
}

func setupRenameRepo(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content", "entities"))
	mustWrite(t, filepath.Join(tmp, "content", "entities", "foo.md"),
		"---\ntitle: \"Foo\"\ntype: entity\n---\n\nBody.\n")
	mustWrite(t, filepath.Join(tmp, "content", "entities", "bar.md"),
		"---\ntitle: \"Bar\"\ntype: entity\n---\n\nReferences [[foo]].\n")
	return tmp
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRenameMovesFileAndUpdatesWikilinks(t *testing.T) {
	tmp := setupRenameRepo(t)
	logger := &fakeLogger{}
	var stdout, stderr bytes.Buffer
	code := Rename(RenameOptions{
		OldSlug:  "foo",
		NewSlug:  "foo-renamed",
		RepoRoot: tmp,
		GitMover: fakeMover{},
		Logger:   logger,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(tmp, "content", "entities", "foo-renamed.md")); err != nil {
		t.Fatalf("renamed file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "content", "entities", "foo.md")); err == nil {
		t.Fatalf("old file still present")
	}
	bar, _ := os.ReadFile(filepath.Join(tmp, "content", "entities", "bar.md"))
	if !strings.Contains(string(bar), "[[foo-renamed]]") {
		t.Fatalf("bar.md not updated: %q", bar)
	}
	if !strings.Contains(stdout.String(), "RENAME-OK|old=foo|new=foo-renamed") {
		t.Fatalf("missing record: %q", stdout.String())
	}
	if len(logger.calls) != 1 || !strings.HasPrefix(logger.calls[0], "rename|foo -> foo-renamed") {
		t.Fatalf("logger=%v", logger.calls)
	}
}

func TestRenameUpdatesPipedWikilinks(t *testing.T) {
	tmp := setupRenameRepo(t)
	mustWrite(t, filepath.Join(tmp, "content", "entities", "bar.md"),
		"References [[foo|see foo]].\n")
	var stdout, stderr bytes.Buffer
	code := Rename(RenameOptions{
		OldSlug:  "foo",
		NewSlug:  "foo2",
		RepoRoot: tmp,
		GitMover: fakeMover{},
		Logger:   &fakeLogger{},
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	bar, _ := os.ReadFile(filepath.Join(tmp, "content", "entities", "bar.md"))
	if !strings.Contains(string(bar), "[[foo2|see foo]]") {
		t.Fatalf("piped link not updated: %q", bar)
	}
}

func TestRenameMissingSlug(t *testing.T) {
	tmp := setupRenameRepo(t)
	var stdout, stderr bytes.Buffer
	code := Rename(RenameOptions{
		OldSlug:  "missing",
		NewSlug:  "anything",
		RepoRoot: tmp,
		GitMover: fakeMover{},
		Logger:   &fakeLogger{},
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "slug not found: missing") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestRenameTargetExists(t *testing.T) {
	tmp := setupRenameRepo(t)
	// Pre-create the target.
	mustWrite(t, filepath.Join(tmp, "content", "entities", "foo2.md"), "x")
	var stdout, stderr bytes.Buffer
	code := Rename(RenameOptions{
		OldSlug:  "foo",
		NewSlug:  "foo2",
		RepoRoot: tmp,
		GitMover: fakeMover{},
		Logger:   &fakeLogger{},
	}, &stdout, &stderr)
	if code != 3 {
		t.Fatalf("expected 3, got %d", code)
	}
	if !strings.Contains(stderr.String(), "target exists:") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestRenameCLIArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RenameCLI([]string{"only-one"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero")
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}
