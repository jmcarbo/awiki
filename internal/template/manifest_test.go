package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureV0Manifest = `schema_version = 1
template_version = "0.1.0"

[strategies]
overwrite        = ["scripts/**", "BOOTSTRAP.md"]
preserve         = ["content/**", "raw/**"]
three_way        = ["WIKI.md", "hugo.toml"]
attributes_merge = [".gitattributes"]
template_only    = ["template.manifest.toml", "migrations/**"]

[new_file_default]
strategy = "prompt"

[bootstrap]
ordered_steps = ["dep-check", "domain", "stage-commit"]

[bootstrap.dangerous]
ids = []
`

func writeManifest(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "template.manifest.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return p
}

func TestLoadManifest_FixtureV0(t *testing.T) {
	p := writeManifest(t, fixtureV0Manifest)
	m, err := LoadManifest(p)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.SchemaVersion != 1 {
		t.Errorf("schema_version=%d want 1", m.SchemaVersion)
	}
	if m.TemplateVersion != "0.1.0" {
		t.Errorf("template_version=%q want 0.1.0", m.TemplateVersion)
	}
	if m.NewFileDefault.Strategy != "prompt" {
		t.Errorf("new_file_default=%q want prompt", m.NewFileDefault.Strategy)
	}
	wantOrdered := []string{"dep-check", "domain", "stage-commit"}
	if len(m.Bootstrap.OrderedSteps) != len(wantOrdered) {
		t.Fatalf("ordered_steps=%v want %v", m.Bootstrap.OrderedSteps, wantOrdered)
	}
	for i, s := range wantOrdered {
		if m.Bootstrap.OrderedSteps[i] != s {
			t.Errorf("ordered_steps[%d]=%q want %q", i, m.Bootstrap.OrderedSteps[i], s)
		}
	}
}

func TestLoadManifest_MissingFile(t *testing.T) {
	_, err := LoadManifest("/definitely/not/here.toml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "manifest not found") {
		t.Errorf("error=%q does not match oracle message", err.Error())
	}
}

func TestFormatLoad_MatchesOracle(t *testing.T) {
	p := writeManifest(t, fixtureV0Manifest)
	m, err := LoadManifest(p)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	got := m.FormatLoad()
	want := "schema_version=1\ntemplate_version=0.1.0\nnew_file_default=prompt\n"
	if got != want {
		t.Errorf("FormatLoad=%q want %q", got, want)
	}
}

func TestFormatLoad_DefaultsToPrompt_WhenMissing(t *testing.T) {
	body := `schema_version = 0
template_version = ""
[strategies]
[bootstrap]
ordered_steps = []
[bootstrap.dangerous]
ids = []
`
	p := writeManifest(t, body)
	m, err := LoadManifest(p)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	got := m.FormatLoad()
	want := "schema_version=0\ntemplate_version=\nnew_file_default=prompt\n"
	if got != want {
		t.Errorf("FormatLoad=%q want %q", got, want)
	}
}

func TestResolveStrategy_FixtureV0(t *testing.T) {
	p := writeManifest(t, fixtureV0Manifest)
	m, err := LoadManifest(p)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	cases := []struct {
		relpath string
		want    string
	}{
		{"scripts/foo.sh", "overwrite"},
		{"WIKI.md", "three_way"},
		{".gitattributes", "attributes_merge"},
		{"content/notes/foo.md", "preserve"},
		{"random/unknown.txt", "prompt"},
		{"BOOTSTRAP.md", "overwrite"},
		{"hugo.toml", "three_way"},
		{"raw/img.png", "preserve"},
		{"migrations/0001-foo.toml", "template_only"},
	}
	for _, tc := range cases {
		t.Run(tc.relpath, func(t *testing.T) {
			got := m.ResolveStrategy(tc.relpath)
			if got != tc.want {
				t.Errorf("ResolveStrategy(%q)=%q want %q", tc.relpath, got, tc.want)
			}
		})
	}
}

func TestResolveStrategy_MostSpecificGlobWins(t *testing.T) {
	// Mirrors the bats test "most-specific glob wins (content/log.md)":
	// preserve = ["content/**"], three_way = ["content/log.md"]. The
	// literal "content/log.md" is more specific than "content/**" so
	// three_way must win even though preserve has lower precedence.
	body := `schema_version = 1
template_version = "0.0.0"
[strategies]
overwrite = []
preserve = ["content/**"]
three_way = ["content/log.md"]
attributes_merge = []
template_only = []
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = []
[bootstrap.dangerous]
ids = []
`
	p := writeManifest(t, body)
	m, _ := LoadManifest(p)
	if got := m.ResolveStrategy("content/log.md"); got != "three_way" {
		t.Errorf("ResolveStrategy=%q want three_way", got)
	}
	if got := m.ResolveStrategy("content/other.md"); got != "preserve" {
		t.Errorf("ResolveStrategy=%q want preserve", got)
	}
}

func TestHasGlobOverlap_Clean(t *testing.T) {
	p := writeManifest(t, fixtureV0Manifest)
	m, _ := LoadManifest(p)
	if _, _, ok := m.HasGlobOverlap(); ok {
		t.Error("expected no overlap on clean fixture")
	}
}

func TestHasGlobOverlap_Duplicate(t *testing.T) {
	body := `schema_version = 1
template_version = "0.0.0"
[strategies]
overwrite = ["scripts/**", "scripts/**"]
preserve = []
three_way = []
attributes_merge = []
template_only = []
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = []
[bootstrap.dangerous]
ids = []
`
	p := writeManifest(t, body)
	m, _ := LoadManifest(p)
	strat, glob, ok := m.HasGlobOverlap()
	if !ok {
		t.Fatal("expected overlap")
	}
	if strat != "overwrite" {
		t.Errorf("strategy=%q want overwrite", strat)
	}
	if glob != "scripts/**" {
		t.Errorf("glob=%q want scripts/**", glob)
	}
}

func TestBootstrapIDs(t *testing.T) {
	p := writeManifest(t, fixtureV0Manifest)
	m, _ := LoadManifest(p)
	got := m.Bootstrap.OrderedSteps
	want := []string{"dep-check", "domain", "stage-commit"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestDangerousIDs_Empty(t *testing.T) {
	p := writeManifest(t, fixtureV0Manifest)
	m, _ := LoadManifest(p)
	if len(m.Bootstrap.Dangerous.IDs) != 0 {
		t.Errorf("expected no dangerous ids, got %v", m.Bootstrap.Dangerous.IDs)
	}
}

func TestFnmatch(t *testing.T) {
	cases := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"*.md", "WIKI.md", true},
		{"*.md", "scripts/foo.md", true}, // fnmatch * spans /
		{"WIKI.md", "WIKI.md", true},
		{"WIKI.md", "wiki.md", false},
		{"foo?.md", "foo1.md", true},
		{"foo?.md", "foo12.md", false},
		{"[ab]c", "ac", true},
		{"[ab]c", "cc", false},
		{"[!ab]c", "cc", true},
	}
	for _, tc := range cases {
		got := fnmatch(tc.pattern, tc.name)
		if got != tc.want {
			t.Errorf("fnmatch(%q,%q)=%v want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
}

func TestGlobMatches_DoubleStar(t *testing.T) {
	cases := []struct {
		glob, rel string
		want      bool
	}{
		{"scripts/**", "scripts/foo.sh", true},
		{"scripts/**", "scripts/sub/bar.sh", true},
		{"scripts/**", "scripts", true}, // fnmatch behavior: prefix == relpath
		{"scripts/**", "other/foo.sh", false},
		{"content/**", "content/notes/foo.md", true},
		{"content/**", "content/notes/sub/foo.md", true},
		{"content/**", "raw/foo.md", false},
		{"a/**/c", "a/b/c", true},
		{"a/**/c", "a/c", true},
		{"a/**/c", "a/x/y/c", true},
	}
	for _, tc := range cases {
		got := globMatches(tc.glob, tc.rel)
		if got != tc.want {
			t.Errorf("globMatches(%q,%q)=%v want %v", tc.glob, tc.rel, got, tc.want)
		}
	}
}
