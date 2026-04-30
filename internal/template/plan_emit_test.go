package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakePlanEmitGit returns a fixed exit code, ignoring the inputs.
type fakePlanEmitGit struct{ code int }

func (f *fakePlanEmitGit) MergeFileDryRun(curBytes, baseBytes, newBytes []byte) (int, error) {
	return f.code, nil
}

func loadFixtureManifest(t *testing.T, dir string) *Manifest {
	t.Helper()
	body := `schema_version = 1
template_version = "0.1.0"
[strategies]
overwrite = ["scripts/**"]
preserve = ["content/**"]
three_way = ["WIKI.md"]
attributes_merge = [".gitattributes"]
template_only = ["template.manifest.toml", "migrations/**"]
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = ["dep-check"]
[bootstrap.dangerous]
ids = []
`
	mp := filepath.Join(dir, "template.manifest.toml")
	if err := os.WriteFile(mp, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(mp)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestEmitPlan_Header(t *testing.T) {
	dir := t.TempDir()
	m := loadFixtureManifest(t, dir)
	in := PlanEmitInput{
		OldTree:   filepath.Join(dir, "old"),
		NewTree:   filepath.Join(dir, "new"),
		UserTree:  filepath.Join(dir, "user"),
		Manifest:  m,
		CommitOld: "AAA",
		CommitNew: "BBB",
	}
	_ = os.MkdirAll(in.OldTree, 0o755)
	_ = os.MkdirAll(in.NewTree, 0o755)
	_ = os.MkdirAll(in.UserTree, 0o755)
	lines, _, err := EmitPlan(&fakePlanEmitGit{}, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) < 2 {
		t.Fatalf("expected at least header+footer, got %v", lines)
	}
	if lines[0].Format() != "PLAN|header|1|AAA|BBB" {
		t.Errorf("header=%q", lines[0].Format())
	}
	footer := lines[len(lines)-1].Format()
	if footer != "PLAN|footer|errors=0|warnings=0|prompts=0|conflicts=0" {
		t.Errorf("footer=%q", footer)
	}
}

func TestEmitPlan_OverwriteAndDeletion(t *testing.T) {
	dir := t.TempDir()
	m := loadFixtureManifest(t, dir)
	old := filepath.Join(dir, "old")
	newT := filepath.Join(dir, "new")
	user := filepath.Join(dir, "user")
	writeFile(t, filepath.Join(old, "scripts", "a.sh"), "old\n")
	writeFile(t, filepath.Join(newT, "scripts", "a.sh"), "new\n")  // overwrite (changed)
	writeFile(t, filepath.Join(old, "scripts", "del.sh"), "del\n") // deletion-in-template
	writeFile(t, filepath.Join(newT, "scripts", "added.sh"), "x\n") // new (overwrite due to scripts/** strategy)
	writeFile(t, filepath.Join(user, "scripts", "a.sh"), "user\n")

	lines, counts, err := EmitPlan(&fakePlanEmitGit{}, PlanEmitInput{
		OldTree:   old,
		NewTree:   newT,
		UserTree:  user,
		Manifest:  m,
		CommitOld: "X",
		CommitNew: "Y",
	})
	if err != nil {
		t.Fatal(err)
	}

	formatted := make([]string, len(lines))
	for i, l := range lines {
		formatted[i] = l.Format()
	}
	got := strings.Join(formatted, "\n")
	mustContain := []string{
		"PLAN|overwrite|scripts/a.sh|",
		"PLAN|overwrite|scripts/added.sh|",
		"PLAN|deletion-in-template|scripts/del.sh|",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("missing %q in:\n%s", s, got)
		}
	}
	if counts.Errors != 0 || counts.Conflicts != 0 {
		t.Errorf("counts=%+v", counts)
	}
}

func TestEmitPlan_NewFilePromptCounts(t *testing.T) {
	dir := t.TempDir()
	// Manifest with no globs that match "random/foo.md" so default
	// strategy "prompt" applies.
	m := loadFixtureManifest(t, dir)
	old := filepath.Join(dir, "old")
	newT := filepath.Join(dir, "new")
	user := filepath.Join(dir, "user")
	_ = os.MkdirAll(old, 0o755)
	_ = os.MkdirAll(user, 0o755)
	writeFile(t, filepath.Join(newT, "random", "foo.md"), "x")

	lines, counts, err := EmitPlan(&fakePlanEmitGit{}, PlanEmitInput{
		OldTree:  old,
		NewTree:  newT,
		UserTree: user,
		Manifest: m,
	})
	if err != nil {
		t.Fatal(err)
	}
	if counts.Prompts != 1 {
		t.Errorf("Prompts=%d want 1", counts.Prompts)
	}
	got := joinLines(lines)
	if !strings.Contains(got, "PLAN|new_file|random/foo.md|prompt") {
		t.Errorf("missing new_file line:\n%s", got)
	}
}

func TestEmitPlan_ThreeWayConflict(t *testing.T) {
	dir := t.TempDir()
	m := loadFixtureManifest(t, dir)
	old := filepath.Join(dir, "old")
	newT := filepath.Join(dir, "new")
	user := filepath.Join(dir, "user")
	writeFile(t, filepath.Join(old, "WIKI.md"), "base\n")
	writeFile(t, filepath.Join(newT, "WIKI.md"), "newbody\n")
	writeFile(t, filepath.Join(user, "WIKI.md"), "userbody\n")
	g := &fakePlanEmitGit{code: 2}
	lines, counts, err := EmitPlan(g, PlanEmitInput{
		OldTree: old, NewTree: newT, UserTree: user, Manifest: m,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := joinLines(lines)
	if !strings.Contains(got, "PLAN|three_way|WIKI.md|conflict-predicted|2") {
		t.Errorf("missing conflict line:\n%s", got)
	}
	if counts.Conflicts != 2 {
		t.Errorf("Conflicts=%d want 2", counts.Conflicts)
	}
}

func TestEmitPlan_MigrationLines(t *testing.T) {
	dir := t.TempDir()
	m := loadFixtureManifest(t, dir)
	old := filepath.Join(dir, "old")
	newT := filepath.Join(dir, "new")
	user := filepath.Join(dir, "user")
	_ = os.MkdirAll(old, 0o755)
	_ = os.MkdirAll(user, 0o755)
	mig := filepath.Join(newT, "migrations")
	_ = os.MkdirAll(mig, 0o755)
	writeFile(t, filepath.Join(mig, "0001-foo.sh"), "#!/usr/bin/env bash\necho hi\n")
	writeFile(t, filepath.Join(mig, "0002-bar.prompt.md"), `---
id: 0002-bar
scope_glob: content/**
risk: low
---
body
`)

	lines, _, err := EmitPlan(&fakePlanEmitGit{}, PlanEmitInput{
		OldTree: old, NewTree: newT, UserTree: user, Manifest: m,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := joinLines(lines)
	if !strings.Contains(got, "PLAN|migration|0001-foo|migrations/0001-foo.sh|2-lines") {
		t.Errorf("missing migration line:\n%s", got)
	}
	if !strings.Contains(got, "PLAN|migration-content|0001-foo|sha256:") {
		t.Errorf("missing migration-content line:\n%s", got)
	}
	if !strings.Contains(got, "PLAN|migration-prompt|0002-bar|migrations/0002-bar.prompt.md|content/**|0|low") {
		t.Errorf("missing migration-prompt line:\n%s", got)
	}
}

func TestEmitPlan_BootstrapStepNew(t *testing.T) {
	dir := t.TempDir()
	m := loadFixtureManifest(t, dir)
	old := filepath.Join(dir, "old")
	newT := filepath.Join(dir, "new")
	user := filepath.Join(dir, "user")
	_ = os.MkdirAll(old, 0o755)
	_ = os.MkdirAll(user, 0o755)
	writeFile(t, filepath.Join(newT, "BOOTSTRAP.md"), "### Setup\n<!-- bootstrap-step: dep-check -->\nbody\n")
	lines, _, err := EmitPlan(&fakePlanEmitGit{}, PlanEmitInput{
		OldTree: old, NewTree: newT, UserTree: user, Manifest: m,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := joinLines(lines)
	if !strings.Contains(got, "PLAN|bootstrap-step-new|dep-check") {
		t.Errorf("missing bootstrap-step-new line:\n%s", got)
	}
}

func joinLines(lines []PlanLine) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.Format())
		b.WriteByte('\n')
	}
	return b.String()
}
