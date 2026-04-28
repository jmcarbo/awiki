package synth

import (
	"testing"

	"awiki/internal/wiki"
)

func TestResolveScopeSlugsExplicitSlugs(t *testing.T) {
	contentDir := t.TempDir()
	page := writeSynthPage(t, contentDir, "synthesis.md", `---
title: "Synthesis"
type: synthesis
plugin: briefing
scope:
  slugs: [s2, s1]
---
body
`)

	slugs := ResolveScopeSlugs(page, nil)

	if got, want := joinSlugs(slugs), "s1,s2"; got != want {
		t.Fatalf("ResolveScopeSlugs() = %q, want %q", got, want)
	}
}

func TestResolveScopeSlugsTag(t *testing.T) {
	contentDir := t.TempDir()
	s1 := writeSynthPage(t, contentDir, "s1.md", `---
title: "S1"
type: source
tags: [memex]
---
body
`)
	s2 := writeSynthPage(t, contentDir, "s2.md", `---
title: "S2"
type: source
tags: [other]
---
body
`)
	page := writeSynthPage(t, contentDir, "synthesis.md", `---
title: "Synthesis"
type: synthesis
plugin: briefing
scope:
  tag: memex
---
body
`)
	idx := wiki.BuildIndex([]wiki.Page{s1, s2, page})

	slugs := ResolveScopeSlugs(page, &idx)

	if got, want := joinSlugs(slugs), "s1"; got != want {
		t.Fatalf("ResolveScopeSlugs() = %q, want %q", got, want)
	}
}

func TestLintS4OutOfScopeCitation(t *testing.T) {
	contentDir := t.TempDir()
	s1 := writeSynthPage(t, contentDir, "s1.md", `---
title: "S1"
type: source
---
body
`)
	s2 := writeSynthPage(t, contentDir, "s2.md", `---
title: "S2"
type: source
---
body
`)
	page := writeSynthPage(t, contentDir, "synthesis.md", `---
title: "Synthesis"
type: synthesis
plugin: briefing
scope:
  slugs: [s1]
---

<!-- BEGIN GENERATED plugin=briefing scope_hash=x -->
Finding [[s2]]
<!-- END GENERATED -->
`)
	idx := wiki.BuildIndex([]wiki.Page{s1, s2, page})

	diagnostics := LintS4(page, &idx)

	assertDiagnostic(t, diagnostics, "ERROR", "S4", "citation [[s2]] is out of scope")
}

func TestLintS5ScopeDrift(t *testing.T) {
	contentDir := t.TempDir()
	s1 := writeSynthPage(t, contentDir, "s1.md", `---
title: "S1"
type: source
---
body
`)
	page := writeSynthPage(t, contentDir, "synthesis.md", `---
title: "Synthesis"
type: synthesis
plugin: briefing
scope:
  slugs: [s1]
---

<!-- BEGIN GENERATED plugin=briefing scope_hash=000000 -->
body
<!-- END GENERATED -->
`)
	idx := wiki.BuildIndex([]wiki.Page{s1, page})

	diagnostics := LintS5(page, &idx)

	assertDiagnostic(t, diagnostics, "WARN", "S5", "scope drift (declared=000000 current="+ScopeHash([]string{"s1"})+"); consider regen")
}

func TestLintS5SkipsQueryScope(t *testing.T) {
	contentDir := t.TempDir()
	page := writeSynthPage(t, contentDir, "synthesis.md", `---
title: "Synthesis"
type: synthesis
plugin: briefing
scope:
  query: "memex"
---

<!-- BEGIN GENERATED plugin=briefing scope_hash=000000 -->
body
<!-- END GENERATED -->
`)
	idx := wiki.BuildIndex([]wiki.Page{page})

	diagnostics := LintS5(page, &idx)

	assertNoDiagnostic(t, diagnostics, "S5")
}

func joinSlugs(slugs []string) string {
	out := ""
	for _, slug := range slugs {
		if out != "" {
			out += ","
		}
		out += slug
	}
	return out
}
