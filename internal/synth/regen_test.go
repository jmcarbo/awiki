package synth

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"awiki/internal/adapters"
)

// testPlugin returns the path to a tiny test plugin written into dir.
func writeTestPlugin(t *testing.T, pluginDir, name string) {
	t.Helper()
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `---
name: ` + name + `
version: 1
description: test plugin
output_type: note
min_sources: 1
max_sources: 10
---
Scope: {{scope_description}}
{{#pages}}
{{/pages}}
{{#feedback}}
Feedback:
{{feedback}}
{{/feedback}}
Done.
`
	path := filepath.Join(pluginDir, name+".md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// synthPageWithMarkers builds a synthesis page body with BEGIN/END markers.
func synthPageWithMarkers(plugin, scopeFM, feedbackSection string) string {
	fm := "---\ntitle: test-synth\ntype: synthesis\nplugin: " + plugin + "\n" + scopeFM + "\n---\n\n"
	body := "Lead paragraph.\n\n"
	if feedbackSection != "" {
		body += feedbackSection + "\n\n"
	}
	body += "<!-- BEGIN GENERATED plugin=" + plugin + " scope_hash=aaaaaa -->\nOld generated content.\n<!-- END GENERATED -->\n"
	return fm + body
}

// makeRegenRunner creates a temp-dir-backed Runner with a fakeGit, ready for
// regen tests. It also writes a tiny source page so slugToPath can resolve it.
func makeRegenRunner(t *testing.T, git adapters.SynthGit) (dir string, r *Runner) {
	t.Helper()
	dir = t.TempDir()

	contentDir := filepath.Join(dir, "content")
	synthDir := filepath.Join(contentDir, "synthesis")
	pluginDir := filepath.Join(dir, "plugins")
	_ = os.MkdirAll(synthDir, 0o755)
	_ = os.MkdirAll(filepath.Join(dir, ".awiki", "maps"), 0o755)

	writeTestPlugin(t, pluginDir, "briefing")

	// Write a source page that the synth page references.
	srcPage := filepath.Join(contentDir, "src-page.md")
	_ = os.WriteFile(srcPage, []byte("---\ntitle: src\ntags: []\ntype: note\n---\n\nSrc lead.\n"), 0o644)

	r = &Runner{
		RepoRoot:   dir,
		ContentDir: contentDir,
		PluginDir:  pluginDir,
		Git:        git,
	}
	return dir, r
}

// writeSynthFile writes the synth page for slug into the runner's synthDir.
func writeSynthFile(t *testing.T, r *Runner, slug, content string) string {
	t.Helper()
	path := filepath.Join(r.synthDir(), slug+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeSynthFile: %v", err)
	}
	return path
}

// ---------------------------------------------------------------------------
// Test 1: Live regen happy path — --force, markers updated, stderr correct.
// ---------------------------------------------------------------------------

func TestRegenLiveHappyPath(t *testing.T) {
	git := &fakeGit{tracked: true, head: "", headOk: false} // --force; git not consulted
	dir, r := makeRegenRunner(t, git)

	pageContent := synthPageWithMarkers("briefing", "scope:\n  slugs: [src-page]", "")
	livePath := writeSynthFile(t, r, "my-synth", pageContent)

	var stdout, stderr bytes.Buffer
	err := r.Regen(context.Background(), RegenOptions{Slug: "my-synth", Force: true}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Regen error: %v", err)
	}

	// Verify the live file was modified.
	updated, _ := os.ReadFile(livePath)
	updatedStr := string(updated)

	// Compute expected hash.
	wantHash := ScopeHash([]string{"src-page"})

	// BEGIN line must carry updated plugin+hash.
	wantBegin := "<!-- BEGIN GENERATED plugin=briefing scope_hash=" + wantHash + " -->"
	if !strings.Contains(updatedStr, wantBegin) {
		t.Errorf("BEGIN line not updated:\ngot file:\n%s\nwant line: %s", updatedStr, wantBegin)
	}

	// Body of region must be cleared (only blank line between BEGIN and END).
	beginIdx := strings.Index(updatedStr, wantBegin)
	endIdx := strings.Index(updatedStr, "<!-- END GENERATED -->")
	if beginIdx < 0 || endIdx < 0 {
		t.Fatal("markers not found in file")
	}
	between := updatedStr[beginIdx+len(wantBegin) : endIdx]
	if strings.TrimSpace(between) != "" {
		t.Errorf("region body not cleared; between markers: %q", between)
	}

	// Stderr must contain telemetry.
	stderrStr := stderr.String()
	wantStderr := "SYNTH-REGEN|target=" + livePath + "|stage=0|scope_hash=" + wantHash
	if !strings.Contains(stderrStr, wantStderr) {
		t.Errorf("stderr missing telemetry:\ngot:  %q\nwant: %q", stderrStr, wantStderr)
	}

	// Stdout must contain rendered prompt.
	stdoutStr := stdout.String()
	if !strings.Contains(stdoutStr, "Scope: explicit slug list") {
		t.Errorf("stdout missing scope_description: %q", stdoutStr)
	}
	if !strings.Contains(stdoutStr, "Done.") {
		t.Errorf("stdout missing prompt tail: %q", stdoutStr)
	}

	// last_updated in frontmatter must be unchanged (we don't touch it in regen).
	_ = dir // ensure dir used
}

// ---------------------------------------------------------------------------
// Test 2: --stage mode — staged file written; live untouched; stderr stage=1.
// ---------------------------------------------------------------------------

func TestRegenStageMode(t *testing.T) {
	git := &fakeGit{tracked: false}
	_, r := makeRegenRunner(t, git)

	pageContent := synthPageWithMarkers("briefing", "scope:\n  slugs: [src-page]", "")
	livePath := writeSynthFile(t, r, "stage-synth", pageContent)
	liveOriginal, _ := os.ReadFile(livePath)

	var stdout, stderr bytes.Buffer
	err := r.Regen(context.Background(), RegenOptions{Slug: "stage-synth", Stage: true}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Regen error: %v", err)
	}

	// Live file must be untouched.
	liveAfter, _ := os.ReadFile(livePath)
	if !bytes.Equal(liveOriginal, liveAfter) {
		t.Errorf("live file was modified in --stage mode")
	}

	// Staged file must exist.
	stagedPath := filepath.Join(r.synthDir(), ".staged", "stage-synth.md")
	stagedData, err := os.ReadFile(stagedPath)
	if err != nil {
		t.Fatalf("staged file not found: %v", err)
	}

	// Staged file must have updated markers.
	wantHash := ScopeHash([]string{"src-page"})
	wantBegin := "<!-- BEGIN GENERATED plugin=briefing scope_hash=" + wantHash + " -->"
	if !strings.Contains(string(stagedData), wantBegin) {
		t.Errorf("staged file missing updated BEGIN line:\n%s", stagedData)
	}

	// Stderr must report stage=1.
	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "|stage=1|") {
		t.Errorf("stderr missing stage=1: %q", stderrStr)
	}
	if !strings.Contains(stderrStr, "target="+stagedPath) {
		t.Errorf("stderr missing staged target path: %q", stderrStr)
	}
}

// ---------------------------------------------------------------------------
// Test 3: --force skips hand-edit check even when git shows divergent HEAD.
// ---------------------------------------------------------------------------

func TestRegenForceSkipsHandEdit(t *testing.T) {
	// Git returns divergent content (would fail hand-edit check).
	modifiedHead := synthPageWithMarkers("briefing", "scope:\n  slugs: [src-page]", "")
	git := &fakeGit{tracked: true, head: modifiedHead + "extra line\n", headOk: true}
	_, r := makeRegenRunner(t, git)

	pageContent := synthPageWithMarkers("briefing", "scope:\n  slugs: [src-page]", "")
	writeSynthFile(t, r, "force-synth", pageContent)

	var stdout, stderr bytes.Buffer
	err := r.Regen(context.Background(), RegenOptions{Slug: "force-synth", Force: true}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error with --force, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Test 4: Hand-edit exit 4 — divergent HEAD, no --force or --stage.
// ---------------------------------------------------------------------------

func TestRegenHandEditExit4(t *testing.T) {
	// Working copy region differs from HEAD region → hand-edit.
	headPage := synthPageWithMarkers("briefing", "scope:\n  slugs: [src-page]", "")
	workPage := strings.Replace(headPage, "Old generated content.", "Hand-edited!", 1)

	git := &fakeGit{tracked: true, head: headPage, headOk: true}
	_, r := makeRegenRunner(t, git)
	writeSynthFile(t, r, "handedit-synth", workPage)

	var stdout, stderr bytes.Buffer
	err := r.Regen(context.Background(), RegenOptions{Slug: "handedit-synth"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected ExitError, got nil")
	}
	ee, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T: %v", err, err)
	}
	if ee.Code != 4 {
		t.Errorf("expected exit code 4, got %d", ee.Code)
	}
	if !strings.Contains(ee.Msg, "hand-edit detected") {
		t.Errorf("error message missing 'hand-edit detected': %q", ee.Msg)
	}
}

// ---------------------------------------------------------------------------
// Test 5: Untracked file — git reports untracked, regen proceeds normally.
// ---------------------------------------------------------------------------

func TestRegenUntrackedFile(t *testing.T) {
	// LsFiles returns tracked=false → untracked → skip hand-edit check.
	git := &fakeGit{tracked: false}
	_, r := makeRegenRunner(t, git)

	pageContent := synthPageWithMarkers("briefing", "scope:\n  slugs: [src-page]", "")
	writeSynthFile(t, r, "untracked-synth", pageContent)

	var stdout, stderr bytes.Buffer
	// No --force, no --stage: relies on untracked path to proceed.
	err := r.Regen(context.Background(), RegenOptions{Slug: "untracked-synth"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error for untracked file, got: %v", err)
	}
	if !strings.Contains(stderr.String(), "SYNTH-REGEN|") {
		t.Errorf("expected stderr telemetry, got: %q", stderr.String())
	}
}

// ---------------------------------------------------------------------------
// Test 6: Privacy fail-closed — source page has "private" tag; target not
//         private → exit 2.
// ---------------------------------------------------------------------------

func TestRegenPrivacyFailClosed(t *testing.T) {
	git := &fakeGit{tracked: false}
	dir, r := makeRegenRunner(t, git)

	// Overwrite src-page with a private tag.
	contentDir := filepath.Join(dir, "content")
	privSrc := filepath.Join(contentDir, "priv-src.md")
	_ = os.WriteFile(privSrc, []byte("---\ntitle: priv-src\ntags: [private]\ntype: note\n---\n\nPrivate lead.\n"), 0o644)

	pageContent := synthPageWithMarkers("briefing", "scope:\n  slugs: [priv-src]", "")
	writeSynthFile(t, r, "privacy-synth", pageContent)

	var stdout, stderr bytes.Buffer
	err := r.Regen(context.Background(), RegenOptions{Slug: "privacy-synth", Force: true}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected ExitError for privacy violation, got nil")
	}
	ee, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T: %v", err, err)
	}
	if ee.Code != 2 {
		t.Errorf("expected exit code 2, got %d", ee.Code)
	}
	if !strings.Contains(ee.Msg, "private source") {
		t.Errorf("error message missing 'private source': %q", ee.Msg)
	}
}
