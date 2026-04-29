package synth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// minimalTemplate is a fixture template exercising all substitution features.
const minimalTemplate = `You are a briefing agent for {{scope_description}}.

Pages to brief:
{{#pages}}
- [[stub]] (note) — stub lead
{{/pages}}

{{#feedback}}
User feedback:
{{feedback}}
{{/feedback}}

End of prompt.
`

// makeTestPage writes a temporary page file and returns the dir.
func makeTestPage(t *testing.T, dir, slug, typ, body string) {
	t.Helper()
	content := "---\ntitle: \"" + slug + "\"\ntype: " + typ + "\n---\n\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, slug+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEmitPromptScopeDescription(t *testing.T) {
	r := &Runner{ContentDir: t.TempDir()}
	plugin := Plugin{Body: minimalTemplate}
	var buf strings.Builder
	if err := r.EmitPrompt(&buf, plugin, nil, "pages tagged 'foo'", ""); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "pages tagged 'foo'") {
		t.Errorf("scope_description not substituted: %q", got)
	}
	if strings.Contains(got, "{{scope_description}}") {
		t.Errorf("{{scope_description}} literal still present: %q", got)
	}
}

func TestEmitPromptFeedbackDroppedWhenEmpty(t *testing.T) {
	r := &Runner{ContentDir: t.TempDir()}
	plugin := Plugin{Body: minimalTemplate}
	var buf strings.Builder
	if err := r.EmitPrompt(&buf, plugin, nil, "desc", ""); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if strings.Contains(got, "{{#feedback}}") || strings.Contains(got, "{{/feedback}}") {
		t.Errorf("feedback block delimiters not removed: %q", got)
	}
	if strings.Contains(got, "User feedback:") {
		t.Errorf("feedback section present when feedback empty: %q", got)
	}
}

func TestEmitPromptFeedbackRenderedWhenPresent(t *testing.T) {
	r := &Runner{ContentDir: t.TempDir()}
	plugin := Plugin{Body: minimalTemplate}
	var buf strings.Builder
	if err := r.EmitPrompt(&buf, plugin, nil, "desc", "```text\nfix this\n```"); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "fix this") {
		t.Errorf("feedback content not rendered: %q", got)
	}
	if strings.Contains(got, "{{#feedback}}") || strings.Contains(got, "{{/feedback}}") {
		t.Errorf("feedback block delimiters not removed: %q", got)
	}
}

func TestEmitPromptPagesBlockSingleSlug(t *testing.T) {
	dir := t.TempDir()
	makeTestPage(t, dir, "my-note", "note", "This is the lead line.")
	r := &Runner{ContentDir: dir}
	plugin := Plugin{Body: minimalTemplate}
	var buf strings.Builder
	if err := r.EmitPrompt(&buf, plugin, []string{"my-note"}, "desc", ""); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "- [[my-note]] (note) — This is the lead line.") {
		t.Errorf("pages block not rendered correctly: %q", got)
	}
	if strings.Contains(got, "{{#pages}}") || strings.Contains(got, "{{/pages}}") {
		t.Errorf("pages block delimiters not removed: %q", got)
	}
}

func TestEmitPromptPagesBlockMultiSlugs(t *testing.T) {
	dir := t.TempDir()
	makeTestPage(t, dir, "alpha", "note", "Alpha lead.")
	makeTestPage(t, dir, "beta", "article", "Beta lead.")
	r := &Runner{ContentDir: dir}
	plugin := Plugin{Body: minimalTemplate}
	var buf strings.Builder
	if err := r.EmitPrompt(&buf, plugin, []string{"alpha", "beta"}, "desc", ""); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "- [[alpha]] (note) — Alpha lead.") {
		t.Errorf("alpha not in pages block: %q", got)
	}
	if !strings.Contains(got, "- [[beta]] (article) — Beta lead.") {
		t.Errorf("beta not in pages block: %q", got)
	}
}

func TestEmitPromptExactByteOutput(t *testing.T) {
	// Minimal template with no pages and no feedback.
	tmpl := "Scope: {{scope_description}}\n{{#pages}}\n{{/pages}}\nDone.\n"
	r := &Runner{ContentDir: t.TempDir()}
	plugin := Plugin{Body: tmpl}
	var buf strings.Builder
	if err := r.EmitPrompt(&buf, plugin, nil, "test scope", ""); err != nil {
		t.Fatal(err)
	}
	// With no slugs, pagesBlock returns "". expandPagesBlock:
	// prefix = "Scope: test scope\n" trimmed to "Scope: test scope"
	// rows = ""
	// suffix = stripLeadingNewline("\nDone.\n") = "Done.\n"
	// result = "Scope: test scope" + "" + "\n" + "Done.\n"
	want := "Scope: test scope\n\nDone.\n"
	got := buf.String()
	if got != want {
		t.Errorf("exact output mismatch:\ngot  %q\nwant %q", got, want)
	}
}

func TestRenderFeedbackBlockEmpty(t *testing.T) {
	page := []byte("---\ntitle: test\n---\n\nNo feedback section.\n")
	got := renderFeedbackBlock(page)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestRenderFeedbackBlockWithBullets(t *testing.T) {
	page := []byte(`---
title: test
---

Content.

## Feedback

- fix the intro
- add more examples

## Notes

Other stuff.
`)
	got := renderFeedbackBlock(page)
	want := "```text\nfix the intro\nadd more examples\n```"
	if got != want {
		t.Errorf("renderFeedbackBlock:\ngot  %q\nwant %q", got, want)
	}
}

func TestRenderFeedbackBlockStopsAtGeneratedMarker(t *testing.T) {
	page := []byte(`---
title: test
---

## Feedback

- bullet one

<!-- BEGIN GENERATED plugin=x scope_hash=abc -->
generated
<!-- END GENERATED -->
`)
	got := renderFeedbackBlock(page)
	want := "```text\nbullet one\n```"
	if got != want {
		t.Errorf("renderFeedbackBlock:\ngot  %q\nwant %q", got, want)
	}
}

func TestExpandPagesBlockPreservesNewlines(t *testing.T) {
	body := "Before.\n{{#pages}}\nignored content\n{{/pages}}\nAfter.\n"
	rows := "- [[a]] (note) — lead"
	got := expandPagesBlock(body, rows)
	want := "Before.\n- [[a]] (note) — lead\nAfter.\n"
	if got != want {
		t.Errorf("expandPagesBlock:\ngot  %q\nwant %q", got, want)
	}
}
