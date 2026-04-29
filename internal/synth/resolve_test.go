package synth

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"awiki/internal/adapters"
)

// fakeQmd is a test double for adapters.Qmd.
type fakeQmd struct {
	output string
	code   int
	err    error
}

func (f *fakeQmd) Search(_ context.Context, _, _ string) (string, int, error) {
	return f.output, f.code, f.err
}

func (f *fakeQmd) Reindex(_ context.Context, _ string) (string, int, error) {
	return "", 0, nil
}

// Compile-time check.
var _ adapters.Qmd = (*fakeQmd)(nil)

// makeTestRepo creates a minimal repo structure in a temp dir and returns
// a configured Runner. It writes synth pages and regular content pages.
func makeTestRepo(t *testing.T) (string, *Runner) {
	t.Helper()
	dir := t.TempDir()

	contentDir := filepath.Join(dir, "content")
	synthDir := filepath.Join(contentDir, "synthesis")
	_ = os.MkdirAll(synthDir, 0o755)
	_ = os.MkdirAll(filepath.Join(dir, ".awiki", "maps"), 0o755)

	r := &Runner{
		RepoRoot:   dir,
		ContentDir: contentDir,
	}
	return dir, r
}

func writePage(t *testing.T, dir, relPath, content string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	_ = os.MkdirAll(filepath.Dir(full), 0o755)
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("writePage %s: %v", relPath, err)
	}
}

func writeSynthPage(t *testing.T, dir, slug, fmExtra string) {
	t.Helper()
	path := filepath.Join(dir, "content", "synthesis", slug+".md")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	content := "---\ntitle: " + slug + "\ntype: synthesis\n" + fmExtra + "\n---\nbody\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeSynthPage %s: %v", slug, err)
	}
}

func runResolve(t *testing.T, r *Runner, slug string) []string {
	t.Helper()
	var buf bytes.Buffer
	if err := r.Resolve(context.Background(), slug, &buf); err != nil {
		t.Fatalf("Resolve(%q): %v", slug, err)
	}
	out := strings.TrimRight(buf.String(), "\n")
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// TestResolveSlugsFlavor: scope.kind=slugs, no filters.
func TestResolveSlugsFlavor(t *testing.T) {
	dir, r := makeTestRepo(t)

	// Write synth page with slugs scope.
	writeSynthPage(t, dir, "my-synth",
		"scope:\n  slugs: [beta, alpha, gamma]\n")

	// Write the referenced pages so slugToPath can resolve them (needed for
	// privacy filter). No filter is active but filterPrivacy is always called.
	for _, slug := range []string{"alpha", "beta", "gamma"} {
		writePage(t, dir, "content/"+slug+".md",
			"---\ntitle: "+slug+"\ntags: []\ntype: note\n---\n")
	}

	got := runResolve(t, r, "my-synth")
	want := []string{"alpha", "beta", "gamma"}
	if !equalStringSlice(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestResolveTagFlavor: scope.kind=tag, small content tree.
func TestResolveTagFlavor(t *testing.T) {
	dir, r := makeTestRepo(t)

	// Three pages; two have tag "decision", one does not.
	writePage(t, dir, "content/page-a.md",
		"---\ntitle: a\ntags: [decision, project]\ntype: note\nlast_updated: 2024-01-01\n---\n")
	writePage(t, dir, "content/page-b.md",
		"---\ntitle: b\ntags: [decision]\ntype: note\nlast_updated: 2024-01-01\n---\n")
	writePage(t, dir, "content/page-c.md",
		"---\ntitle: c\ntags: [unrelated]\ntype: note\nlast_updated: 2024-01-01\n---\n")

	writeSynthPage(t, dir, "tag-synth",
		"scope:\n  tag: decision\n")

	got := runResolve(t, r, "tag-synth")
	// Expect page-a and page-b (sorted).
	if len(got) != 2 {
		t.Fatalf("got %v (len=%d), want 2 slugs", got, len(got))
	}
	if got[0] != "page-a" || got[1] != "page-b" {
		t.Fatalf("got %v, want [page-a page-b]", got)
	}
}

// TestResolveQueryFlavor: scope.kind=query with injected fake Qmd.
func TestResolveQueryFlavor(t *testing.T) {
	dir, r := makeTestRepo(t)

	// Fake qmd returns two result lines (slug + extra text).
	r.Qmd = &fakeQmd{
		output: "alpha some score\nbeta another\n",
	}

	// Write pages so privacy filter resolves them.
	writePage(t, dir, "content/alpha.md",
		"---\ntitle: alpha\ntags: []\ntype: note\n---\n")
	writePage(t, dir, "content/beta.md",
		"---\ntitle: beta\ntags: []\ntype: note\n---\n")

	writeSynthPage(t, dir, "query-synth",
		"scope:\n  query: \"decision making\"\n")

	got := runResolve(t, r, "query-synth")
	want := []string{"alpha", "beta"}
	if !equalStringSlice(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestResolveAppliesExcludeTags: pages with excluded tags are dropped.
func TestResolveAppliesExcludeTags(t *testing.T) {
	dir, r := makeTestRepo(t)

	writePage(t, dir, "content/keep.md",
		"---\ntitle: keep\ntags: []\ntype: note\n---\n")
	writePage(t, dir, "content/drop.md",
		"---\ntitle: drop\ntags: [archive]\ntype: note\n---\n")

	writeSynthPage(t, dir, "excl-synth",
		"scope:\n  slugs: [keep, drop]\n  exclude_tags: [archive]\n")

	got := runResolve(t, r, "excl-synth")
	if len(got) != 1 || got[0] != "keep" {
		t.Fatalf("got %v, want [keep]", got)
	}
}

// TestResolveAppliesMinLastUpdated: pages older than min are dropped.
func TestResolveAppliesMinLastUpdated(t *testing.T) {
	dir, r := makeTestRepo(t)

	writePage(t, dir, "content/new-page.md",
		"---\ntitle: new\ntags: []\ntype: note\nlast_updated: 2025-01-01\n---\n")
	writePage(t, dir, "content/old-page.md",
		"---\ntitle: old\ntags: []\ntype: note\nlast_updated: 2020-01-01\n---\n")

	writeSynthPage(t, dir, "mlu-synth",
		"scope:\n  slugs: [new-page, old-page]\n  min_last_updated: 2024-01-01\n")

	got := runResolve(t, r, "mlu-synth")
	if len(got) != 1 || got[0] != "new-page" {
		t.Fatalf("got %v, want [new-page]", got)
	}
}

// TestResolveSortsOutput: output is always ASCII-sorted.
func TestResolveSortsOutput(t *testing.T) {
	dir, r := makeTestRepo(t)

	for _, slug := range []string{"zebra", "apple", "mango"} {
		writePage(t, dir, "content/"+slug+".md",
			"---\ntitle: "+slug+"\ntags: []\ntype: note\n---\n")
	}

	// Intentionally in reverse order.
	writeSynthPage(t, dir, "sort-synth",
		"scope:\n  slugs: [zebra, mango, apple]\n")

	got := runResolve(t, r, "sort-synth")
	want := []string{"apple", "mango", "zebra"}
	if !equalStringSlice(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestResolveExcludesPrivatePagesByDefault: pages tagged "private" are
// excluded when the synth page is not under content/private/.
func TestResolveExcludesPrivatePagesByDefault(t *testing.T) {
	dir, r := makeTestRepo(t)

	writePage(t, dir, "content/public-page.md",
		"---\ntitle: public\ntags: []\ntype: note\n---\n")
	writePage(t, dir, "content/private-page.md",
		"---\ntitle: private\ntags: [private]\ntype: note\n---\n")

	writeSynthPage(t, dir, "priv-synth",
		"scope:\n  slugs: [public-page, private-page]\n")

	got := runResolve(t, r, "priv-synth")
	if len(got) != 1 || got[0] != "public-page" {
		t.Fatalf("got %v, want [public-page]", got)
	}
}

// TestResolveAllowsPrivateWhenSynthIsPrivate: if the synth page lives under
// content/private/, private pages are included.
func TestResolveAllowsPrivateWhenSynthIsPrivate(t *testing.T) {
	dir, _ := makeTestRepo(t)

	// Write the synth page under content/private/synthesis/.
	privateSynthPath := filepath.Join(dir, "content", "private", "synthesis", "priv-synth.md")
	_ = os.MkdirAll(filepath.Dir(privateSynthPath), 0o755)
	content := "---\ntitle: priv-synth\ntype: synthesis\nscope:\n  slugs: [public-page, private-page]\n---\nbody\n"
	if err := os.WriteFile(privateSynthPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	writePage(t, dir, "content/public-page.md",
		"---\ntitle: public\ntags: []\ntype: note\n---\n")
	writePage(t, dir, "content/private-page.md",
		"---\ntitle: private\ntags: [private]\ntype: note\n---\n")

	r2 := &Runner{
		RepoRoot:   dir,
		ContentDir: filepath.Join(dir, "content"),
	}

	// Resolve directly by calling internal helper with the private synth path.
	synthPagePath := privateSynthPath
	data, err := os.ReadFile(synthPagePath)
	if err != nil {
		t.Fatal(err)
	}
	fmText := extractFrontmatterText(string(data))
	scope := ParseScopeFromFrontmatter(fmText)
	synthPrivate := isPrivatePath(synthPagePath)

	if !synthPrivate {
		t.Fatal("expected synthPrivate=true for path under /private/")
	}

	slugs, err := r2.resolveScope(context.Background(), scope, synthPrivate)
	if err != nil {
		t.Fatal(err)
	}

	// Both pages should be present.
	found := make(map[string]bool)
	for _, s := range slugs {
		found[s] = true
	}
	if !found["public-page"] || !found["private-page"] {
		t.Fatalf("got %v, want both public-page and private-page", slugs)
	}
}

func equalStringSlice(a, b []string) bool {
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
