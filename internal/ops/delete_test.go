package ops

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRemover struct{}

func (fakeRemover) Remove(ctx context.Context, repoRoot, path string) error {
	return os.Remove(path)
}

func setupDeleteRepo(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content", "entities"))
	mustWrite(t, filepath.Join(tmp, "content", "entities", "foo.md"),
		"---\ntitle: \"Foo\"\ntype: entity\n---\n\nBody.\n")
	mustWrite(t, filepath.Join(tmp, "content", "entities", "bar.md"),
		"---\ntitle: \"Bar\"\ntype: entity\n---\n\nReferences [[foo]].\n")
	return tmp
}

func TestDeleteRemovesFile(t *testing.T) {
	tmp := setupDeleteRepo(t)
	var stdout, stderr bytes.Buffer
	code := Delete(DeleteOptions{
		Slug:     "foo",
		RepoRoot: tmp,
		GitRm:    fakeRemover{},
		Logger:   &fakeLogger{},
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(tmp, "content", "entities", "foo.md")); err == nil {
		t.Fatalf("foo.md still present")
	}
	if !strings.Contains(stdout.String(), "DELETE-OK|slug=foo") {
		t.Fatalf("missing record: %q", stdout.String())
	}
}

func TestDeleteMarksWikilinksBroken(t *testing.T) {
	tmp := setupDeleteRepo(t)
	var stdout, stderr bytes.Buffer
	code := Delete(DeleteOptions{
		Slug:     "foo",
		RepoRoot: tmp,
		GitRm:    fakeRemover{},
		Logger:   &fakeLogger{},
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	bar, _ := os.ReadFile(filepath.Join(tmp, "content", "entities", "bar.md"))
	if !strings.Contains(string(bar), "broken: was [[foo]]") {
		t.Fatalf("broken marker missing: %q", bar)
	}
}

func TestDeletePipedWikilinksMarked(t *testing.T) {
	tmp := setupDeleteRepo(t)
	mustWrite(t, filepath.Join(tmp, "content", "entities", "bar.md"),
		"References [[foo|see foo]].\n")
	var stdout, stderr bytes.Buffer
	Delete(DeleteOptions{
		Slug:     "foo",
		RepoRoot: tmp,
		GitRm:    fakeRemover{},
		Logger:   &fakeLogger{},
	}, &stdout, &stderr)
	bar, _ := os.ReadFile(filepath.Join(tmp, "content", "entities", "bar.md"))
	if !strings.Contains(string(bar), "broken: was [[foo|... -->foo|see foo]]") {
		t.Fatalf("piped broken marker missing: %q", bar)
	}
}

func TestDeleteUnknownSlug(t *testing.T) {
	tmp := setupDeleteRepo(t)
	var stdout, stderr bytes.Buffer
	code := Delete(DeleteOptions{
		Slug:     "nonexistent",
		RepoRoot: tmp,
		GitRm:    fakeRemover{},
		Logger:   &fakeLogger{},
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
}

func TestDeleteCLIUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := DeleteCLI(nil, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero")
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}
