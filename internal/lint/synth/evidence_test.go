package synth

import (
	"os"
	"testing"
)

func TestLintS9EvidenceWordsOverCap(t *testing.T) {
	repoRoot := t.TempDir()
	contentDir := t.TempDir()
	writePluginWithCap(t, repoRoot, "briefing", 3)
	page := writeSynthPage(t, contentDir, "synthesis.md", `---
title: "Synthesis"
type: synthesis
plugin: briefing
---

<!-- BEGIN GENERATED plugin=briefing scope_hash=x -->
## Evidence
> "one two three four" — [[s1]]
<!-- END GENERATED -->
`)

	diagnostics := LintS9(page, repoRoot)

	assertDiagnostic(t, diagnostics, "ERROR", "S9", "aggregate evidence words=4 exceeds plugin cap max_evidence_total_words=3")
}

func TestLintS9EvidenceWordsUnderCap(t *testing.T) {
	repoRoot := t.TempDir()
	contentDir := t.TempDir()
	writePluginWithCap(t, repoRoot, "briefing", 4)
	page := writeSynthPage(t, contentDir, "synthesis.md", `---
title: "Synthesis"
type: synthesis
plugin: briefing
---

<!-- BEGIN GENERATED plugin=briefing scope_hash=x -->
## Evidence
> "one two three four" — [[s1]]
<!-- END GENERATED -->
`)

	diagnostics := LintS9(page, repoRoot)

	assertNoDiagnostic(t, diagnostics, "S9")
}

func writePluginWithCap(t *testing.T, repoRoot string, name string, cap int) {
	t.Helper()
	writePlugin(t, repoRoot, name, nil)
	path := repoRoot + "/synthesis-plugins/" + name + ".md"
	body := `---
name: ` + name + `
max_evidence_total_words: ` + intString(cap) + `
required_sections: []
---
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
