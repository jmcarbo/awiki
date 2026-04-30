package template

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMigrationHeader_Basic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "0001-foo.sh")
	writeFile(t, p, `#!/usr/bin/env bash
# migration: 0001-foo
# requires: 0.1.0
# touches: content/**
# idempotent: true
echo hi
`)
	h, err := ParseMigrationHeader(p)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"migration":  "0001-foo",
		"requires":   "0.1.0",
		"touches":    "content/**",
		"idempotent": "true",
	}
	for k, v := range want {
		if h[k] != v {
			t.Errorf("h[%q]=%q want %q", k, h[k], v)
		}
	}
}

func TestValidateMigrationHeader_Missing(t *testing.T) {
	h := map[string]string{"migration": "x"}
	missing := ValidateMigrationHeader(h)
	want := []string{"idempotent", "requires", "touches"}
	if !equalStrings(missing, want) {
		t.Errorf("missing=%v want %v", missing, want)
	}
}

func TestValidateMigrationTouches_Blocked(t *testing.T) {
	cases := []struct {
		touches string
		want    string
	}{
		{"content/**", ""},
		{"secrets/db.env", "secrets/db.env"},
		{"themes/foo themes/bar", "themes/foo"},
		{".git", ".git"},
		{".awiki/foo", ".awiki/foo"},
	}
	for _, tc := range cases {
		h := map[string]string{"touches": tc.touches}
		got := ValidateMigrationTouches(h)
		if got != tc.want {
			t.Errorf("touches=%q got=%q want=%q", tc.touches, got, tc.want)
		}
	}
}

func TestMigrationGlobMatch(t *testing.T) {
	cases := []struct {
		glob, path string
		want       bool
	}{
		{"content/**", "content/foo.md", true},
		{"content/**", "content/sub/foo.md", true},
		{"content/**", "raw/foo.md", false},
		{"content/**.md", "content/foo.md", true},
		{"*.md", "WIKI.md", true},
		{"WIKI.md", "wiki.md", false},
	}
	for _, tc := range cases {
		got := MigrationGlobMatch(tc.glob, tc.path)
		if got != tc.want {
			t.Errorf("MigrationGlobMatch(%q,%q)=%v want %v", tc.glob, tc.path, got, tc.want)
		}
	}
}

func TestMigrationParseFrontmatter(t *testing.T) {
	text := `---
id: 0001-foo
scope_glob: "content/**"
risk: medium
---
body
`
	fm := MigrationParseFrontmatter(text)
	if fm["id"] != "0001-foo" {
		t.Errorf("id=%q", fm["id"])
	}
	if fm["scope_glob"] != "content/**" {
		t.Errorf("scope_glob=%q", fm["scope_glob"])
	}
	if fm["risk"] != "medium" {
		t.Errorf("risk=%q", fm["risk"])
	}
}

func TestResolvePromptScope(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "content", "a.md"), "x")
	writeFile(t, filepath.Join(dir, "content", "sub", "b.md"), "x")
	writeFile(t, filepath.Join(dir, "raw", "c.md"), "x")
	got, err := ResolvePromptScope(dir, "content/**")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"content/a.md", "content/sub/b.md"}
	if !equalStrings(got, want) {
		t.Errorf("got=%v want=%v", got, want)
	}
}

type fakePromptCheckAttr struct {
	encrypted map[string]bool
}

func (f *fakePromptCheckAttr) CheckAttrFilter(attrFile, path string) (string, error) {
	if f.encrypted[path] {
		return path + ": filter: git-crypt\n", nil
	}
	return path + ": filter: unspecified\n", nil
}

func TestStagePrompt_Success(t *testing.T) {
	dir := t.TempDir()
	prompt := filepath.Join(dir, "0001-foo.prompt.md")
	writeFile(t, prompt, `---
id: 0001-foo
scope_glob: content/**
risk: low
---
Body content here.
`)
	writeFile(t, filepath.Join(dir, "user", "content", "a.md"), "x")
	pending := filepath.Join(dir, "pending")
	staged, sid, risk, matches, err := StagePrompt(PromptStageInput{
		PromptPath: prompt,
		UserTree:   filepath.Join(dir, "user"),
		PendingDir: pending,
	}, nil)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if sid != "0001-foo" {
		t.Errorf("sid=%q", sid)
	}
	if risk != "low" {
		t.Errorf("risk=%q", risk)
	}
	if !equalStrings(matches, []string{"content/a.md"}) {
		t.Errorf("matches=%v", matches)
	}
	body, err := os.ReadFile(staged)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Body content here.") {
		t.Errorf("body missing original content")
	}
	if !strings.Contains(string(body), "## Resolved scope") {
		t.Errorf("missing Resolved scope section")
	}
	if !strings.Contains(string(body), "- content/a.md") {
		t.Errorf("missing match line")
	}
	if !strings.Contains(string(body), "## Risk\nlow") {
		t.Errorf("missing risk section")
	}
}

func TestStagePrompt_BlockedScope(t *testing.T) {
	dir := t.TempDir()
	prompt := filepath.Join(dir, "0001-foo.prompt.md")
	writeFile(t, prompt, `---
id: 0001-foo
scope_glob: secrets/**
risk: low
---
`)
	_, _, _, _, err := StagePrompt(PromptStageInput{
		PromptPath: prompt,
		UserTree:   dir,
		PendingDir: filepath.Join(dir, "pending"),
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "scope_glob blocked") {
		t.Errorf("err=%v", err)
	}
}

func TestStagePrompt_EncryptedMatchRejected(t *testing.T) {
	dir := t.TempDir()
	prompt := filepath.Join(dir, "0001-foo.prompt.md")
	writeFile(t, prompt, `---
id: 0001-foo
scope_glob: content/**
risk: low
---
`)
	writeFile(t, filepath.Join(dir, "user", "content", "secret.bin"), "x")
	gitAttr := filepath.Join(dir, "user", ".gitattributes")
	writeFile(t, gitAttr, "secret.bin filter=git-crypt\n")
	attr := &fakePromptCheckAttr{encrypted: map[string]bool{"content/secret.bin": true}}
	_, _, _, _, err := StagePrompt(PromptStageInput{
		PromptPath:  prompt,
		UserTree:    filepath.Join(dir, "user"),
		PendingDir:  filepath.Join(dir, "pending"),
		GitAttrFile: gitAttr,
	}, attr)
	if err == nil || !strings.Contains(err.Error(), "encrypted path") {
		t.Errorf("err=%v", err)
	}
}

// fakeMigrationRunner is a deterministic stub for MigrationRunner.
type fakeMigrationRunner struct {
	bashScripts []string
	bashArgs    [][]string
	bashCode    int
	bashErr     error
	prePorcelain  string
	postPorcelain string
	restoreCalls  [][]string
}

func (f *fakeMigrationRunner) RunBash(script, cwd string, args, env []string) (int, error) {
	f.bashScripts = append(f.bashScripts, script)
	f.bashArgs = append(f.bashArgs, append([]string{}, args...))
	return f.bashCode, f.bashErr
}
func (f *fakeMigrationRunner) GitStatusPorcelain(repoRoot string) (string, error) {
	if len(f.bashScripts) <= 1 {
		return f.prePorcelain, nil
	}
	return f.postPorcelain, nil
}
func (f *fakeMigrationRunner) GitRestoreFromHead(repoRoot string, paths []string) error {
	f.restoreCalls = append(f.restoreCalls, append([]string{}, paths...))
	return nil
}

func TestRunMigration_CleanRun(t *testing.T) {
	dir := t.TempDir()
	mig := filepath.Join(dir, "0001-foo.sh")
	writeFile(t, mig, `#!/usr/bin/env bash
# migration: 0001-foo
# requires: 0.1.0
# touches: content/**
# idempotent: true
:
`)
	r := &fakeMigrationRunner{
		prePorcelain:  "",
		postPorcelain: " M content/foo.md\n",
	}
	res, err := RunMigration(r, MigrationRunInput{
		Script:     mig,
		RepoRoot:   dir,
		OldVersion: "0.1.0",
		NewVersion: "0.2.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Code != 0 || len(res.AwikiWrites) != 0 || len(res.OutOfScope) != 0 {
		t.Errorf("res=%#v want clean", res)
	}
	if len(r.bashScripts) != 2 {
		t.Errorf("expected 2 bash invocations, got %d", len(r.bashScripts))
	}
	if !equalStrings(r.bashArgs[0], []string{"--dry-run"}) {
		t.Errorf("first call args=%v want --dry-run", r.bashArgs[0])
	}
}

func TestRunMigration_AwikiWriteHalts(t *testing.T) {
	dir := t.TempDir()
	mig := filepath.Join(dir, "0001-foo.sh")
	writeFile(t, mig, `#!/usr/bin/env bash
# migration: 0001-foo
# requires: 0.1.0
# touches: content/**
# idempotent: true
:
`)
	r := &fakeMigrationRunner{
		postPorcelain: " M .awiki/foo.json\n",
	}
	res, err := RunMigration(r, MigrationRunInput{
		Script: mig, RepoRoot: dir, OldVersion: "a", NewVersion: "b",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.AwikiWrites) != 1 {
		t.Errorf("AwikiWrites=%v want 1", res.AwikiWrites)
	}
	if len(r.restoreCalls) != 1 {
		t.Errorf("expected restore call")
	}
}

func TestRunMigration_OutOfScopeUntracked(t *testing.T) {
	dir := t.TempDir()
	mig := filepath.Join(dir, "0001-foo.sh")
	writeFile(t, mig, `#!/usr/bin/env bash
# migration: 0001-foo
# requires: 0.1.0
# touches: content/**
# idempotent: true
:
`)
	// Pretend the migration created an untracked file outside touches.
	writeFile(t, filepath.Join(dir, "raw", "leak.txt"), "x")
	r := &fakeMigrationRunner{
		postPorcelain: "?? raw/leak.txt\n",
	}
	res, err := RunMigration(r, MigrationRunInput{
		Script: mig, RepoRoot: dir, OldVersion: "a", NewVersion: "b",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.OutOfScope) != 1 || res.OutOfScope[0] != "raw/leak.txt" {
		t.Errorf("OutOfScope=%v", res.OutOfScope)
	}
	// File should have been removed.
	if _, err := os.Stat(filepath.Join(dir, "raw", "leak.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected removed: %v", err)
	}
}

func TestRunMigration_BlockedTouches(t *testing.T) {
	dir := t.TempDir()
	mig := filepath.Join(dir, "0001-foo.sh")
	writeFile(t, mig, `#!/usr/bin/env bash
# migration: 0001-foo
# requires: 0.1.0
# touches: secrets/x
# idempotent: true
:
`)
	_, err := RunMigration(&fakeMigrationRunner{}, MigrationRunInput{
		Script: mig, RepoRoot: dir,
	})
	if err == nil || !strings.Contains(err.Error(), "touches blocked") {
		t.Errorf("err=%v", err)
	}
}
