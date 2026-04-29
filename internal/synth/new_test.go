package synth

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeNewRunner builds a temp-dir-backed Runner suitable for New verb tests.
// It writes a plugin file (with configurable min/max sources) and an optional
// set of source pages so that slugToPath can resolve them.
func makeNewRunner(t *testing.T, minSources, maxSources int) (dir string, r *Runner, pluginDir string) {
	t.Helper()
	dir = t.TempDir()

	contentDir := filepath.Join(dir, "content")
	synthDir := filepath.Join(contentDir, "synthesis")
	pluginDir = filepath.Join(dir, "plugins")
	_ = os.MkdirAll(synthDir, 0o755)
	_ = os.MkdirAll(filepath.Join(dir, ".awiki", "maps"), 0o755)

	writeNewTestPlugin(t, pluginDir, "briefing", minSources, maxSources)

	r = &Runner{
		RepoRoot:   dir,
		ContentDir: contentDir,
		PluginDir:  pluginDir,
		Git:        &fakeGit{tracked: false},
		Today:      "2024-06-01",
	}
	return dir, r, pluginDir
}

// writeNewTestPlugin writes a minimal plugin file with explicit min/max.
func writeNewTestPlugin(t *testing.T, pluginDir, name string, minSources, maxSources int) {
	t.Helper()
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\n" +
		"name: " + name + "\n" +
		"version: 1\n" +
		"description: test plugin\n" +
		"output_type: note\n"
	if minSources > 0 {
		body += "min_sources: " + itoa(minSources) + "\n"
	}
	if maxSources > 0 {
		body += "max_sources: " + itoa(maxSources) + "\n"
	}
	body += "---\n" +
		"Scope: {{scope_description}}\n" +
		"{{#pages}}\n" +
		"{{/pages}}\n" +
		"Done.\n"
	path := filepath.Join(pluginDir, name+".md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeSrcPage creates a source page in contentDir.
func writeSrcPage(t *testing.T, contentDir, slug string, tags []string) {
	t.Helper()
	tagLine := "["
	for i, tag := range tags {
		if i > 0 {
			tagLine += ", "
		}
		tagLine += tag
	}
	tagLine += "]"
	content := "---\ntitle: " + slug + "\ntags: " + tagLine + "\ntype: note\n---\n\n" + slug + " lead.\n"
	if err := os.WriteFile(filepath.Join(contentDir, slug+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Test 1: Happy path slugs flavor — file written; scaffold byte-equivalent;
//         stderr SYNTH-NEW emitted.
// ---------------------------------------------------------------------------

func TestNewHappyPathSlugs(t *testing.T) {
	dir, r, _ := makeNewRunner(t, 1, 10)
	contentDir := filepath.Join(dir, "content")

	// Write two source pages.
	writeSrcPage(t, contentDir, "page-a", nil)
	writeSrcPage(t, contentDir, "page-b", nil)

	var stdout, stderr bytes.Buffer
	err := r.New(context.Background(), NewOptions{
		Plugin:     "briefing",
		Topic:      "my-topic",
		ScopeKind:  "slugs",
		ScopeValue: "page-a,page-b",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	// Target file must exist.
	target := filepath.Join(r.synthDir(), "my-topic-briefing.md")
	data, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("target file not created: %v", readErr)
	}
	got := string(data)

	// Compute expected hash.
	wantHash := ScopeHash([]string{"page-a", "page-b"})

	// Validate scaffold fields.
	checks := []string{
		`title: "my-topic — briefing"`,
		"date: 2024-06-01",
		"last_updated: 2024-06-01",
		"last_generated:",
		"type: synthesis",
		"plugin: briefing",
		"scope:\n  slugs: [page-a,page-b]",
		"tags: [my-topic, briefing]",
		"sources: []",
		"draft: false",
		"<!-- BEGIN GENERATED plugin=briefing scope_hash=" + wantHash + " -->",
		"<!-- END GENERATED -->",
		"Lead paragraph — written once by user/agent, NOT regenerated.",
	}
	for _, c := range checks {
		if !strings.Contains(got, c) {
			t.Errorf("scaffold missing %q\nfull content:\n%s", c, got)
		}
	}

	// Stderr must carry SYNTH-NEW telemetry.
	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "SYNTH-NEW|target="+target) {
		t.Errorf("stderr missing SYNTH-NEW telemetry: %q", stderrStr)
	}
	if !strings.Contains(stderrStr, "|plugin=briefing|") {
		t.Errorf("stderr missing plugin field: %q", stderrStr)
	}
	if !strings.Contains(stderrStr, "|scope_hash="+wantHash) {
		t.Errorf("stderr missing scope_hash: %q", stderrStr)
	}

	// Stdout must contain prompt output.
	if !strings.Contains(stdout.String(), "Scope: explicit slug list") {
		t.Errorf("stdout missing scope description: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Done.") {
		t.Errorf("stdout missing prompt tail: %q", stdout.String())
	}
}

// ---------------------------------------------------------------------------
// Test 2: Missing plugin → exit 1.
// ---------------------------------------------------------------------------

func TestNewMissingPlugin(t *testing.T) {
	_, r, _ := makeNewRunner(t, 1, 10)

	var stdout, stderr bytes.Buffer
	err := r.New(context.Background(), NewOptions{
		Plugin:     "nonexistent-plugin",
		Topic:      "my-topic",
		ScopeKind:  "slugs",
		ScopeValue: "page-a",
	}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for missing plugin, got nil")
	}
	ee, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T: %v", err, err)
	}
	if ee.Code != 1 {
		t.Errorf("expected exit code 1, got %d", ee.Code)
	}
	if !strings.Contains(ee.Msg, "plugin load failed") {
		t.Errorf("error message missing 'plugin load failed': %q", ee.Msg)
	}
}

// ---------------------------------------------------------------------------
// Test 3: Refuses overwrite → exit 3.
// ---------------------------------------------------------------------------

func TestNewRefusesOverwrite(t *testing.T) {
	dir, r, _ := makeNewRunner(t, 1, 10)
	contentDir := filepath.Join(dir, "content")
	writeSrcPage(t, contentDir, "page-a", nil)

	// Pre-create the target file.
	target := filepath.Join(r.synthDir(), "my-topic-briefing.md")
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := r.New(context.Background(), NewOptions{
		Plugin:     "briefing",
		Topic:      "my-topic",
		ScopeKind:  "slugs",
		ScopeValue: "page-a",
	}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected ExitError for overwrite, got nil")
	}
	ee, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T: %v", err, err)
	}
	if ee.Code != 3 {
		t.Errorf("expected exit code 3, got %d", ee.Code)
	}
	if !strings.Contains(ee.Msg, "target already exists") {
		t.Errorf("error message missing 'target already exists': %q", ee.Msg)
	}
}

// ---------------------------------------------------------------------------
// Test 4: Below min_sources → exit 2.
// ---------------------------------------------------------------------------

func TestNewBelowMinSources(t *testing.T) {
	dir, r, _ := makeNewRunner(t, 5, 0) // min_sources=5, no max
	contentDir := filepath.Join(dir, "content")
	writeSrcPage(t, contentDir, "only-one", nil)

	var stdout, stderr bytes.Buffer
	err := r.New(context.Background(), NewOptions{
		Plugin:     "briefing",
		Topic:      "my-topic",
		ScopeKind:  "slugs",
		ScopeValue: "only-one",
	}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected ExitError for min_sources violation, got nil")
	}
	ee, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T: %v", err, err)
	}
	if ee.Code != 2 {
		t.Errorf("expected exit code 2, got %d", ee.Code)
	}
	if !strings.Contains(ee.Msg, "min_sources") {
		t.Errorf("error message missing 'min_sources': %q", ee.Msg)
	}
}

// ---------------------------------------------------------------------------
// Test 5: Above max_sources → exit 2.
// ---------------------------------------------------------------------------

func TestNewAboveMaxSources(t *testing.T) {
	dir, r, _ := makeNewRunner(t, 0, 2) // no min, max_sources=2
	contentDir := filepath.Join(dir, "content")
	for _, slug := range []string{"s1", "s2", "s3", "s4", "s5"} {
		writeSrcPage(t, contentDir, slug, nil)
	}

	var stdout, stderr bytes.Buffer
	err := r.New(context.Background(), NewOptions{
		Plugin:     "briefing",
		Topic:      "my-topic",
		ScopeKind:  "slugs",
		ScopeValue: "s1,s2,s3,s4,s5",
	}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected ExitError for max_sources violation, got nil")
	}
	ee, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T: %v", err, err)
	}
	if ee.Code != 2 {
		t.Errorf("expected exit code 2, got %d", ee.Code)
	}
	if !strings.Contains(ee.Msg, "max_sources") {
		t.Errorf("error message missing 'max_sources': %q", ee.Msg)
	}
}

// ---------------------------------------------------------------------------
// Test 6: Private leakage without --allow-private → exit 2.
// ---------------------------------------------------------------------------

func TestNewPrivateLeakageExit2(t *testing.T) {
	dir, r, _ := makeNewRunner(t, 1, 10)
	contentDir := filepath.Join(dir, "content")

	// Write a private source page.
	writeSrcPage(t, contentDir, "private-page", []string{"private"})

	var stdout, stderr bytes.Buffer
	err := r.New(context.Background(), NewOptions{
		Plugin:     "briefing",
		Topic:      "my-topic",      // target NOT under private/
		ScopeKind:  "slugs",
		ScopeValue: "private-page",
	}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected ExitError for private leakage, got nil")
	}
	ee, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T: %v", err, err)
	}
	if ee.Code != 2 {
		t.Errorf("expected exit code 2, got %d: %s", ee.Code, ee.Msg)
	}
	if !strings.Contains(ee.Msg, "private source in scope") {
		t.Errorf("error message missing 'private source in scope': %q", ee.Msg)
	}
}

// ---------------------------------------------------------------------------
// Test 7: --allow-private allows private sources; scaffold written, stderr
//         includes SYNTH-NEW telemetry.
// ---------------------------------------------------------------------------

func TestNewAllowPrivate(t *testing.T) {
	dir, r, _ := makeNewRunner(t, 1, 10)
	contentDir := filepath.Join(dir, "content")

	// Write a private source page.
	writeSrcPage(t, contentDir, "priv-page", []string{"private"})

	var stdout, stderr bytes.Buffer
	err := r.New(context.Background(), NewOptions{
		Plugin:       "briefing",
		Topic:        "my-topic",
		ScopeKind:    "slugs",
		ScopeValue:   "priv-page",
		AllowPrivate: true,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("New with --allow-private error: %v", err)
	}

	// Target file must be created.
	target := filepath.Join(r.synthDir(), "my-topic-briefing.md")
	if _, statErr := os.Stat(target); statErr != nil {
		t.Errorf("target file not created: %v", statErr)
	}

	// Stderr must contain SYNTH-NEW.
	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "SYNTH-NEW|") {
		t.Errorf("stderr missing SYNTH-NEW telemetry: %q", stderrStr)
	}
}

// ---------------------------------------------------------------------------
// Unit tests for buildScopeBlock and newScopeDescription.
// ---------------------------------------------------------------------------

func TestBuildScopeBlockTag(t *testing.T) {
	got := buildScopeBlock("tag", "my-tag", "", "", "")
	want := "scope:\n  tag: my-tag\n"
	if got != want {
		t.Errorf("buildScopeBlock tag:\ngot  %q\nwant %q", got, want)
	}
}

func TestBuildScopeBlockSlugs(t *testing.T) {
	got := buildScopeBlock("slugs", "a,b,c", "", "", "")
	want := "scope:\n  slugs: [a,b,c]\n"
	if got != want {
		t.Errorf("buildScopeBlock slugs:\ngot  %q\nwant %q", got, want)
	}
}

func TestBuildScopeBlockQuery(t *testing.T) {
	got := buildScopeBlock("query", "my query", "", "", "")
	want := "scope:\n  query: \"my query\"\n"
	if got != want {
		t.Errorf("buildScopeBlock query:\ngot  %q\nwant %q", got, want)
	}
}

func TestBuildScopeBlockOptionalFields(t *testing.T) {
	got := buildScopeBlock("tag", "t", "bad,nope", "2024-01-01", "note")
	want := "scope:\n  tag: t\n  exclude_tags: [bad,nope]\n  min_last_updated: 2024-01-01\n  types: [note]\n"
	if got != want {
		t.Errorf("buildScopeBlock optional fields:\ngot  %q\nwant %q", got, want)
	}
}

func TestNewScopeDescription(t *testing.T) {
	tests := []struct {
		kind string
		val  string
		n    int
		want string
	}{
		{"tag", "my-tag", 0, "pages tagged 'my-tag'"},
		{"slugs", "a,b", 2, "explicit slug list (2 pages)"},
		{"query", "foo bar", 5, "qmd query: foo bar"},
		{"unknown", "x", 0, ""},
	}
	for _, tc := range tests {
		got := newScopeDescription(tc.kind, tc.val, tc.n)
		if got != tc.want {
			t.Errorf("newScopeDescription(%q,%q,%d) = %q, want %q", tc.kind, tc.val, tc.n, got, tc.want)
		}
	}
}

func TestSynthSlugRegexpMatch(t *testing.T) {
	valid := []string{"a", "abc", "a1", "my-slug", "123", "a-b-c"}
	for _, s := range valid {
		if !synthSlugRegexpMatch(s) {
			t.Errorf("synthSlugRegexpMatch(%q) = false, want true", s)
		}
	}
	invalid := []string{"", "-abc", "ABC", "a_b", "a b", "-"}
	for _, s := range invalid {
		if synthSlugRegexpMatch(s) {
			t.Errorf("synthSlugRegexpMatch(%q) = true, want false", s)
		}
	}
}
