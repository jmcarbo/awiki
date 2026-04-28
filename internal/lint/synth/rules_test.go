package synth

import (
	"os"
	"path/filepath"
	"testing"

	"awiki/internal/wiki"
)

func TestLintStructuralS1DoubleBegin(t *testing.T) {
	repoRoot := t.TempDir()
	contentDir := filepath.Join(repoRoot, "content")
	page := writeSynthPage(t, contentDir, "s1-double-begin.md", `---
title: "S1"
type: synthesis
plugin: briefing
scope:
  slugs: [s1]
---

<!-- BEGIN GENERATED plugin=briefing scope_hash=x -->
body
<!-- BEGIN GENERATED plugin=briefing scope_hash=x -->
body
<!-- END GENERATED -->
`)

	diagnostics := LintStructural(page, repoRoot, contentDir, nil)

	assertDiagnostic(t, diagnostics, "ERROR", "S1", "expected exactly one BEGIN GENERATED marker")
}

func TestLintStructuralS2MissingRequiredSection(t *testing.T) {
	repoRoot := t.TempDir()
	contentDir := filepath.Join(repoRoot, "content")
	writePlugin(t, repoRoot, "briefing", []string{"## TL;DR", "## Evidence"})
	page := writeSynthPage(t, contentDir, "missing-evidence.md", `---
title: "S2"
type: synthesis
plugin: briefing
scope:
  slugs: [s1]
---

<!-- BEGIN GENERATED plugin=briefing scope_hash=x -->
## TL;DR
- claim
<!-- END GENERATED -->
`)

	diagnostics := LintStructural(page, repoRoot, contentDir, nil)

	assertDiagnostic(t, diagnostics, "ERROR", "S2", "required section missing: ## Evidence")
}

func TestLintStructuralS7FeedbackCountAndThreshold(t *testing.T) {
	repoRoot := t.TempDir()
	contentDir := filepath.Join(repoRoot, "content")
	var feedback string
	for i := 0; i < 21; i++ {
		feedback += "- item\n"
	}
	page := writeSynthPage(t, contentDir, "feedback.md", `---
title: "Feedback"
type: synthesis
plugin: briefing
scope:
  slugs: [s1]
---

## Feedback
`+feedback+`
<!-- BEGIN GENERATED plugin=briefing scope_hash=x -->
## Evidence
<!-- END GENERATED -->
`)

	diagnostics := LintStructural(page, repoRoot, contentDir, nil)

	assertDiagnostic(t, diagnostics, "INFO", "S7", "feedback_count=21")
	assertDiagnostic(t, diagnostics, "WARN", "S7", "feedback_count=21 exceeds 20; consider scope refactor or page split")
}

func TestLintStructuralS8FeedbackOutOfScope(t *testing.T) {
	repoRoot := t.TempDir()
	contentDir := filepath.Join(repoRoot, "content")
	inScope := writeSynthPage(t, contentDir, "s1.md", `---
title: "S1"
type: source
tags: []
---
source body
`)
	outOfScope := writeSynthPage(t, contentDir, "s2.md", `---
title: "S2"
type: source
tags: []
---
source body
`)
	page := writeSynthPage(t, contentDir, "feedback.md", `---
title: "Feedback"
type: synthesis
plugin: briefing
scope:
  slugs: [s1]
---

## Feedback
- Add [[s2]]

<!-- BEGIN GENERATED plugin=briefing scope_hash=x -->
## Evidence
<!-- END GENERATED -->
`)
	idx := wiki.BuildIndex([]wiki.Page{inScope, outOfScope, page})

	diagnostics := LintStructural(page, repoRoot, contentDir, &idx)

	assertDiagnostic(t, diagnostics, "WARN", "S8", "feedback references out-of-scope page s2; widen scope or remove bullet")
}

func TestLintStructuralS8SkipsQueryScope(t *testing.T) {
	repoRoot := t.TempDir()
	contentDir := filepath.Join(repoRoot, "content")
	page := writeSynthPage(t, contentDir, "feedback.md", `---
title: "Feedback"
type: synthesis
plugin: briefing
scope:
  query: "memex"
---

## Feedback
- Add [[s2]]

<!-- BEGIN GENERATED plugin=briefing scope_hash=x -->
## Evidence
<!-- END GENERATED -->
`)
	idx := wiki.BuildIndex([]wiki.Page{page})

	diagnostics := LintStructural(page, repoRoot, contentDir, &idx)

	assertNoDiagnostic(t, diagnostics, "S8")
}

func writeSynthPage(t *testing.T, contentDir string, rel string, body string) wiki.Page {
	t.Helper()
	path := filepath.Join(contentDir, "synthesis", rel)
	if filepath.Base(rel) == rel && (rel == "s1.md" || rel == "s2.md") {
		path = filepath.Join(contentDir, "sources", rel)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	page, err := wiki.ParsePage(path, contentDir)
	if err != nil {
		t.Fatal(err)
	}
	return page
}

func writePlugin(t *testing.T, repoRoot string, name string, required []string) {
	t.Helper()
	var body string
	body += "---\n"
	body += "name: " + name + "\n"
	body += "required_sections:\n"
	for _, section := range required {
		body += "  - \"" + section + "\"\n"
	}
	body += "---\n"
	path := filepath.Join(repoRoot, "synthesis-plugins", name+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertDiagnostic(t *testing.T, diagnostics []Diagnostic, level string, code string, message string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Level == level && diagnostic.Code == code && diagnostic.Message == message {
			return
		}
	}
	t.Fatalf("missing %s %s %q in %#v", level, code, message, diagnostics)
}

func assertNoDiagnostic(t *testing.T, diagnostics []Diagnostic, code string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			t.Fatalf("unexpected %s diagnostic in %#v", code, diagnostics)
		}
	}
}
