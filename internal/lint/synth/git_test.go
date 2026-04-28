package synth

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"awiki/internal/wiki"
)

func TestLintS6GeneratedRegionEdit(t *testing.T) {
	repoRoot, contentDir, pagePath := setupS6Repo(t)
	replaceInFile(t, pagePath, "last_updated: 2026-04-01", "last_updated: 2026-05-01")
	replaceInFile(t, pagePath, "foo claim", "foo CLAIM-EDITED")
	page, err := wiki.ParsePage(pagePath, contentDir)
	if err != nil {
		t.Fatal(err)
	}

	diagnostics := LintS6(page, repoRoot)

	assertDiagnostic(t, diagnostics, "WARN", "S6", "hand-edit inside generated region (last_updated=2026-05-01 > last_generated=2026-04-15T00:00:00Z)")
}

func TestLintS6FeedbackOnlyEdit(t *testing.T) {
	repoRoot, contentDir, pagePath := setupS6Repo(t)
	replaceInFile(t, pagePath, "last_updated: 2026-04-01", "last_updated: 2026-05-01")
	replaceInFile(t, pagePath, "- earlier note", "- earlier note\n- new note")
	page, err := wiki.ParsePage(pagePath, contentDir)
	if err != nil {
		t.Fatal(err)
	}

	diagnostics := LintS6(page, repoRoot)

	assertNoDiagnostic(t, diagnostics, "S6")
}

func setupS6Repo(t *testing.T) (string, string, string) {
	t.Helper()
	repoRoot := t.TempDir()
	contentDir := filepath.Join(repoRoot, "content")
	pagePath := filepath.Join(contentDir, "synthesis", "s6-page.md")
	if err := os.MkdirAll(filepath.Dir(pagePath), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `---
title: "S6"
date: 2026-04-01
last_updated: 2026-04-01
last_generated: 2026-04-15T00:00:00Z
type: synthesis
plugin: briefing
scope:
  slugs: [foo]
---

Lead.

## Feedback

- earlier note

<!-- BEGIN GENERATED plugin=briefing scope_hash=x -->
## TL;DR
- foo claim [[foo]]
<!-- END GENERATED -->
`
	if err := os.WriteFile(pagePath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoRoot, "init", "-q")
	runGit(t, repoRoot, "config", "user.email", "t@t")
	runGit(t, repoRoot, "config", "user.name", "t")
	runGit(t, repoRoot, "add", ".")
	runGit(t, repoRoot, "commit", "-qm", "init")
	return repoRoot, contentDir, pagePath
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func replaceInFile(t *testing.T, path string, old string, new string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, old) {
		t.Fatalf("%q not found in %s", old, path)
	}
	text = strings.Replace(text, old, new, 1)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
