package git

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func transformFixtureRoot(t *testing.T, name string) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", "..", ".."))
	return filepath.Join(repoRoot, "tests", "fixtures", "ingest", "git", name)
}

// TestTransformBasic drives Transform against the transform-basic
// fixture: a single upstream README.md with frontmatter, a wikilink
// candidate, a code fence, and an external URL.
func TestTransformBasic(t *testing.T) {
	fixture := transformFixtureRoot(t, "transform-basic")
	tmp := t.TempDir()

	srcRoot := filepath.Join(fixture, "input")
	in := filepath.Join(srcRoot, "README.md")
	outPath := filepath.Join(tmp, "content", "sources", "git-example-readme.md")

	opts := TransformOptions{
		InPath:       in,
		OutPath:      outPath,
		RepoKey:      "github-com-example-example",
		RepoName:     "example",
		RepoRelpath:  "README.md",
		GitURL:       "https://github.com/example/example.git",
		GitBlobSHA:   "abc123",
		AssetOutDir:  filepath.Join(tmp, "content", "sources", "_assets", "git-example"),
		UpstreamRoot: srcRoot,
		SlugMap: map[string]string{
			"README.md":      "git-example-readme",
			"docs/intro.md":  "git-example-docs-intro",
			"docs/notes.md":  "git-example-docs-notes",
		},
		Today: "2026-04-30",
	}

	var stdout bytes.Buffer
	res, err := Transform(opts, &stdout)
	if err != nil {
		t.Fatalf("Transform: %v\nstdout: %s", err, stdout.String())
	}
	if !res.Wrote {
		t.Fatal("expected Wrote=true")
	}
	if !strings.Contains(stdout.String(), "OK|0") {
		t.Errorf("stdout missing OK|0: %q", stdout.String())
	}

	// Compare emitted file byte-for-byte with expected_files.
	wantPath := filepath.Join(fixture, "expected_files", "content", "sources", "git-example-readme.md")
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read out: %v", err)
	}
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read expected: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestTransformSkipsShortBody(t *testing.T) {
	tmp := t.TempDir()
	in := filepath.Join(tmp, "tiny.md")
	if err := os.WriteFile(in, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "out", "tiny.md")
	opts := TransformOptions{
		InPath:      in,
		OutPath:     out,
		RepoKey:     "k",
		RepoName:    "k",
		RepoRelpath: "tiny.md",
		GitURL:      "u",
		GitBlobSHA:  "h",
		Today:       "2026-04-30",
	}
	var stdout bytes.Buffer
	res, err := Transform(opts, &stdout)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if res.Wrote {
		t.Error("expected Wrote=false on short body")
	}
	if !strings.Contains(stdout.String(), "WARN|skipped: <10 char body") {
		t.Errorf("stdout missing skipped warning: %q", stdout.String())
	}
	if _, err := os.Stat(out); err == nil {
		t.Errorf("expected out file NOT to exist on skip; stat err = nil")
	}
}

func TestTransformInputMissing(t *testing.T) {
	tmp := t.TempDir()
	opts := TransformOptions{
		InPath:      filepath.Join(tmp, "missing.md"),
		OutPath:     filepath.Join(tmp, "out.md"),
		RepoKey:     "k",
		RepoName:    "k",
		RepoRelpath: "missing.md",
		GitURL:      "u",
		GitBlobSHA:  "h",
		Today:       "2026-04-30",
	}
	var stdout bytes.Buffer
	_, err := Transform(opts, &stdout)
	if err == nil {
		t.Fatal("expected error on missing input")
	}
	if !strings.Contains(err.Error(), "in not found") {
		t.Errorf("error %v missing 'in not found'", err)
	}
}

func TestTransformPrivateAddsTag(t *testing.T) {
	tmp := t.TempDir()
	in := filepath.Join(tmp, "doc.md")
	body := "# Hello\n\nThis is a longer body to clear the 10-char threshold.\n"
	if err := os.WriteFile(in, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "out.md")
	opts := TransformOptions{
		InPath:      in,
		OutPath:     out,
		RepoKey:     "k",
		RepoName:    "k",
		RepoRelpath: "doc.md",
		GitURL:      "u",
		GitBlobSHA:  "h",
		Private:     true,
		Today:       "2026-04-30",
	}
	var stdout bytes.Buffer
	if _, err := Transform(opts, &stdout); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "tags: [git, k, private]") {
		t.Errorf("expected private tag in frontmatter, got:\n%s", got)
	}
}

func TestTransformImageCopy(t *testing.T) {
	tmp := t.TempDir()
	upstream := filepath.Join(tmp, "upstream")
	if err := os.MkdirAll(filepath.Join(upstream, "docs", "img"), 0o755); err != nil {
		t.Fatal(err)
	}
	imgPath := filepath.Join(upstream, "docs", "img", "logo.png")
	if err := os.WriteFile(imgPath, []byte("PNGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := filepath.Join(upstream, "docs", "page.md")
	body := "# Page\n\nHere is an image: ![logo](img/logo.png)\n\nMore body to pad past 10 chars.\n"
	if err := os.WriteFile(in, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "content", "sources", "git-k-docs-page.md")
	assetDir := filepath.Join(tmp, "content", "sources", "_assets", "git-k")
	opts := TransformOptions{
		InPath:       in,
		OutPath:      out,
		RepoKey:      "k",
		RepoName:     "k",
		RepoRelpath:  "docs/page.md",
		GitURL:       "u",
		GitBlobSHA:   "h",
		AssetOutDir:  assetDir,
		UpstreamRoot: upstream,
		Today:        "2026-04-30",
	}
	var stdout bytes.Buffer
	if _, err := Transform(opts, &stdout); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	// Asset must have been copied.
	want := filepath.Join(assetDir, "docs", "img", "logo.png")
	got, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("expected asset at %s, err = %v", want, err)
	}
	if string(got) != "PNGDATA" {
		t.Errorf("asset bytes wrong: %q", got)
	}
	// Output should reference the asset under the content-relative path.
	outBytes, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(outBytes), "![logo](") {
		t.Errorf("output missing rewritten image link:\n%s", outBytes)
	}
}

func TestTransformImageMissingEmitsWarning(t *testing.T) {
	tmp := t.TempDir()
	upstream := filepath.Join(tmp, "upstream")
	if err := os.MkdirAll(filepath.Join(upstream, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	in := filepath.Join(upstream, "docs", "page.md")
	body := "# Page\n\n![missing](img/nope.png)\n\nLong enough body to clear.\n"
	if err := os.WriteFile(in, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "out.md")
	opts := TransformOptions{
		InPath:       in,
		OutPath:      out,
		RepoKey:      "k",
		RepoName:     "k",
		RepoRelpath:  "docs/page.md",
		GitURL:       "u",
		GitBlobSHA:   "h",
		AssetOutDir:  filepath.Join(tmp, "assets"),
		UpstreamRoot: upstream,
		Today:        "2026-04-30",
	}
	var stdout bytes.Buffer
	res, err := Transform(opts, &stdout)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if len(res.Warnings) == 0 {
		t.Error("expected warnings for missing image")
	}
	if !strings.Contains(stdout.String(), "WARN|missing image:") {
		t.Errorf("stdout missing warning: %q", stdout.String())
	}
}

func TestStripUpstreamFrontmatterTitleQuoted(t *testing.T) {
	cases := []struct {
		in    string
		title string
	}{
		{"---\ntitle: bare\n---\nbody", "bare"},
		{"---\ntitle: \"quoted\"\n---\nbody", "quoted"},
		{"---\ntitle: 'sing'\n---\nbody", "sing"},
		{"no frontmatter here", ""},
	}
	for _, c := range cases {
		meta, _ := stripUpstreamFrontmatter(c.in)
		if meta["title"] != c.title {
			t.Errorf("strip(%q): title %q want %q", c.in, meta["title"], c.title)
		}
	}
}

func TestDeriveFallbackTitle(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/path/to/getting-started.md", "Getting Started"},
		{"foo.md", "Foo"},
		{"hello-world-readme.mdx", "Hello World Readme"},
	}
	for _, c := range cases {
		got := deriveFallbackTitle(c.in)
		if got != c.want {
			t.Errorf("deriveFallbackTitle(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeRelpathEdges(t *testing.T) {
	cases := []struct {
		current string
		target  string
		want    string
	}{
		{"docs/foo.md", "bar.md", "docs/bar.md"},
		{"docs/foo.md", "../top.md", "top.md"},
		{"top.md", "../escape.md", ""},
		{"top.md", "https://example/x.md", ""},
		{"top.md", "/abs.md", ""},
		{"top.md", "page.md#anchor", "page.md"},
		{"top.md", "page.md?q=1", "page.md"},
	}
	for _, c := range cases {
		got := normalizeRelpath(c.current, c.target)
		if got != c.want {
			t.Errorf("normalize(%q,%q) = %q, want %q", c.current, c.target, got, c.want)
		}
	}
}

func TestFenceLineSetBacktickFence(t *testing.T) {
	body := strings.Join([]string{
		"line0",
		"```bash",
		"code1",
		"code2",
		"```",
		"line5",
	}, "\n")
	got := fenceLineSet(body)
	for _, i := range []int{1, 2, 3, 4} {
		if !got[i] {
			t.Errorf("expected line %d to be in fence", i)
		}
	}
	for _, i := range []int{0, 5} {
		if got[i] {
			t.Errorf("expected line %d NOT to be in fence", i)
		}
	}
}
