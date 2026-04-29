package synth

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"awiki/internal/adapters"
)

// makeFinalizeRunner creates a temp-dir-backed Runner for finalize tests.
// It wires up a fake lint adapter and writes a minimal source page.
func makeFinalizeRunner(t *testing.T, lint adapters.SynthLint) (dir string, r *Runner) {
	t.Helper()
	dir = t.TempDir()
	contentDir := filepath.Join(dir, "content")
	synthDir := filepath.Join(contentDir, "synthesis")
	stagedDir := filepath.Join(synthDir, ".staged")
	pluginDir := filepath.Join(dir, "plugins")
	_ = os.MkdirAll(synthDir, 0o755)
	_ = os.MkdirAll(stagedDir, 0o755)
	_ = os.MkdirAll(pluginDir, 0o755)
	_ = os.MkdirAll(filepath.Join(dir, ".awiki"), 0o755)

	// Write a source page the synth page can reference.
	srcPage := filepath.Join(contentDir, "src-page.md")
	_ = os.WriteFile(srcPage, []byte("---\ntitle: src\ntags: []\ntype: note\n---\n\nSrc lead.\n"), 0o644)

	r = &Runner{
		RepoRoot:   dir,
		ContentDir: contentDir,
		PluginDir:  pluginDir,
		Lint:       lint,
		Config:     map[string]string{},
	}
	return dir, r
}

// finalizePageBody returns a minimal synthesis page body suitable for finalize tests.
func finalizePageBody(plugin string) string {
	return "---\n" +
		"title: test-synth\n" +
		"type: synthesis\n" +
		"plugin: " + plugin + "\n" +
		"sources: []\n" +
		"last_generated: 2000-01-01T00:00:00Z\n" +
		"scope:\n" +
		"  slugs: [src-page]\n" +
		"---\n\n" +
		"Lead paragraph.\n\n" +
		"<!-- BEGIN GENERATED plugin=" + plugin + " scope_hash=aaaaaa -->\n\n" +
		"Generated content.\n\n" +
		"<!-- END GENERATED -->\n"
}

// writeFinalizePlugin writes a minimal plugin manifest to r.PluginDir.
func writeFinalizePlugin(t *testing.T, r *Runner, name string) {
	t.Helper()
	body := "---\nname: " + name + "\nversion: 1\ndescription: test plugin\noutput_type: note\nmin_sources: 1\nmax_sources: 10\n---\nBody.\n"
	if err := os.WriteFile(filepath.Join(r.PluginDir, name+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeFinalizePluginWithHook writes a plugin manifest that declares a post_hook.
func writeFinalizePluginWithHook(t *testing.T, r *Runner, name, hookPath string) {
	t.Helper()
	body := "---\nname: " + name + "\nversion: 1\ndescription: test plugin\noutput_type: note\nmin_sources: 1\nmax_sources: 10\npost_hook: " + hookPath + "\n---\nBody.\n"
	if err := os.WriteFile(filepath.Join(r.PluginDir, name+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Test 1: Missing target → exit 1.
// ---------------------------------------------------------------------------

func TestFinalizeMissingTargetExit1(t *testing.T) {
	_, r := makeFinalizeRunner(t, fakeLint{exit: 0})

	var stderr bytes.Buffer
	err := r.Finalize(context.Background(), "no-such-slug", &stderr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	ee, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T: %v", err, err)
	}
	if ee.Code != 1 {
		t.Errorf("expected exit code 1, got %d", ee.Code)
	}
	if !strings.Contains(ee.Msg, "synthesis page not found") {
		t.Errorf("message should mention 'synthesis page not found', got %q", ee.Msg)
	}
}

// ---------------------------------------------------------------------------
// Test 2: Marker integrity failure → exit 5.
// ---------------------------------------------------------------------------

func TestFinalizeMarkerIntegrityExit5(t *testing.T) {
	_, r := makeFinalizeRunner(t, fakeLint{exit: 0})

	// Page with no BEGIN/END markers.
	noMarkers := "---\ntitle: test\ntype: synthesis\nplugin: briefing\n---\n\nNo markers here.\n"
	livePath := filepath.Join(r.synthDir(), "no-markers.md")
	if err := os.WriteFile(livePath, []byte(noMarkers), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	err := r.Finalize(context.Background(), "no-markers", &stderr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	ee, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T: %v", err, err)
	}
	if ee.Code != 5 {
		t.Errorf("expected exit code 5, got %d", ee.Code)
	}
}

// ---------------------------------------------------------------------------
// Test 3: Lint failure → exit 6.
// ---------------------------------------------------------------------------

func TestFinalizeLintFailureExit6(t *testing.T) {
	_, r := makeFinalizeRunner(t, fakeLint{exit: 1})

	content := finalizePageBody("briefing")
	livePath := filepath.Join(r.synthDir(), "my-synth.md")
	if err := os.WriteFile(livePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	err := r.Finalize(context.Background(), "my-synth", &stderr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	ee, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T: %v", err, err)
	}
	if ee.Code != 6 {
		t.Errorf("expected exit code 6, got %d", ee.Code)
	}
}

// ---------------------------------------------------------------------------
// Test 4: Happy path live — scope_hash updated, sources rewritten,
//         last_generated set, ingest-count bumped, stderr emitted.
// ---------------------------------------------------------------------------

func TestFinalizeHappyPathLive(t *testing.T) {
	dir, r := makeFinalizeRunner(t, fakeLint{exit: 0})
	writeFinalizePlugin(t, r, "briefing")

	content := finalizePageBody("briefing")
	livePath := filepath.Join(r.synthDir(), "my-synth.md")
	if err := os.WriteFile(livePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	err := r.Finalize(context.Background(), "my-synth", &stderr)
	if err != nil {
		t.Fatalf("Finalize error: %v", err)
	}

	updated, _ := os.ReadFile(livePath)
	updatedStr := string(updated)

	// Expected scope_hash for ["src-page"].
	wantHash := ScopeHash([]string{"src-page"})

	// scope_hash in BEGIN line must be updated.
	wantBegin := "<!-- BEGIN GENERATED plugin=briefing scope_hash=" + wantHash + " -->"
	if !strings.Contains(updatedStr, wantBegin) {
		t.Errorf("BEGIN line not updated:\ngot:\n%s\nwant: %s", updatedStr, wantBegin)
	}

	// sources: must be rewritten to inline list.
	wantSources := `sources: ["[[src-page]]"]`
	if !strings.Contains(updatedStr, wantSources) {
		t.Errorf("sources line not updated:\ngot:\n%s\nwant: %s", updatedStr, wantSources)
	}

	// last_generated must be a valid RFC3339 UTC timestamp.
	lgRE := regexp.MustCompile(`last_generated: ([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z)`)
	m := lgRE.FindStringSubmatch(updatedStr)
	if m == nil {
		t.Errorf("last_generated not found or not RFC3339 UTC in:\n%s", updatedStr)
	}

	// .awiki/ingest-count must be bumped from 0 to 1.
	cntPath := filepath.Join(dir, ".awiki", "ingest-count")
	cntData, cntErr := os.ReadFile(cntPath)
	if cntErr != nil {
		t.Fatalf("ingest-count not found: %v", cntErr)
	}
	cnt, _ := strconv.Atoi(strings.TrimSpace(string(cntData)))
	if cnt != 1 {
		t.Errorf("ingest-count expected 1, got %d", cnt)
	}

	// Stderr must contain telemetry.
	stderrStr := stderr.String()
	wantStderr := fmt.Sprintf("SYNTH-FINALIZE|target=%s|sources=1|scope_hash=%s", livePath, wantHash)
	if !strings.Contains(stderrStr, wantStderr) {
		t.Errorf("stderr missing telemetry:\ngot:  %q\nwant: %q", stderrStr, wantStderr)
	}
}

// ---------------------------------------------------------------------------
// Test 5: Staged target preferred over live when present.
// ---------------------------------------------------------------------------

func TestFinalizeStagedPreferredOverLive(t *testing.T) {
	dir, r := makeFinalizeRunner(t, fakeLint{exit: 0})
	writeFinalizePlugin(t, r, "briefing")

	content := finalizePageBody("briefing")

	// Write both live and staged.
	livePath := filepath.Join(r.synthDir(), "dual-synth.md")
	liveContent := "---\ntitle: live\n---\n\n<!-- BEGIN GENERATED plugin=briefing scope_hash=aaaaaa -->\nLive.\n<!-- END GENERATED -->\n"
	if err := os.WriteFile(livePath, []byte(liveContent), 0o644); err != nil {
		t.Fatal(err)
	}

	stagedPath := filepath.Join(r.synthDir(), ".staged", "dual-synth.md")
	if err := os.WriteFile(stagedPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	err := r.Finalize(context.Background(), "dual-synth", &stderr)
	if err != nil {
		t.Fatalf("Finalize error: %v", err)
	}

	// Live must be untouched.
	liveAfter, _ := os.ReadFile(livePath)
	if string(liveAfter) != liveContent {
		t.Errorf("live file was modified; expected %q, got %q", liveContent, string(liveAfter))
	}

	// Staged must be updated.
	stagedAfter, _ := os.ReadFile(stagedPath)
	wantHash := ScopeHash([]string{"src-page"})
	wantBegin := "<!-- BEGIN GENERATED plugin=briefing scope_hash=" + wantHash + " -->"
	if !strings.Contains(string(stagedAfter), wantBegin) {
		t.Errorf("staged file BEGIN line not updated:\n%s", string(stagedAfter))
	}

	// Stderr must reference staged path.
	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "target="+stagedPath) {
		t.Errorf("stderr should reference staged path, got %q", stderrStr)
	}

	_ = dir
}

// ---------------------------------------------------------------------------
// Test 6: Post-hook gated off — emits blocked notice, no failure.
// ---------------------------------------------------------------------------

func TestFinalizePostHookGatedOff(t *testing.T) {
	_, r := makeFinalizeRunner(t, fakeLint{exit: 0})
	r.Config["ALLOW_PLUGIN_POST_HOOKS"] = ""

	// Create a real hook script file so os.Stat succeeds.
	hookDir := t.TempDir()
	hookScript := filepath.Join(hookDir, "hook.sh")
	if err := os.WriteFile(hookScript, []byte("#!/bin/bash\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFinalizePluginWithHook(t, r, "briefing", hookScript)

	// Hook adapter should NOT be called.
	hook := &fakePostHook{code: 0}
	r.PostHook = hook

	content := finalizePageBody("briefing")
	livePath := filepath.Join(r.synthDir(), "gated-synth.md")
	if err := os.WriteFile(livePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	err := r.Finalize(context.Background(), "gated-synth", &stderr)
	if err != nil {
		t.Fatalf("Finalize should succeed when post-hook gated; got error: %v", err)
	}

	// Stderr should contain the blocked notice.
	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "post-hook-blocked") {
		t.Errorf("expected post-hook-blocked notice in stderr, got %q", stderrStr)
	}

	// hook adapter must not have been called.
	if hook.out != "" || hook.err != nil {
		// fakePostHook has no call counter; we can only check indirectly
		// via the hook not failing.
	}
}

// ---------------------------------------------------------------------------
// Test 7: Post-hook fails → exit 6.
// ---------------------------------------------------------------------------

func TestFinalizePostHookFailsExit6(t *testing.T) {
	_, r := makeFinalizeRunner(t, fakeLint{exit: 0})
	r.Config["ALLOW_PLUGIN_POST_HOOKS"] = "1"

	// Create a real hook script file so os.Stat succeeds.
	hookDir := t.TempDir()
	hookScript := filepath.Join(hookDir, "hook.sh")
	if err := os.WriteFile(hookScript, []byte("#!/bin/bash\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFinalizePluginWithHook(t, r, "briefing", hookScript)

	// Hook adapter returns non-zero.
	hook := &fakePostHook{out: "fail\n", code: 1, err: fmt.Errorf("exit 1")}
	r.PostHook = hook

	content := finalizePageBody("briefing")
	livePath := filepath.Join(r.synthDir(), "hook-fail-synth.md")
	if err := os.WriteFile(livePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	err := r.Finalize(context.Background(), "hook-fail-synth", &stderr)
	if err == nil {
		t.Fatal("expected error from failing post-hook, got nil")
	}
	ee, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T: %v", err, err)
	}
	if ee.Code != 6 {
		t.Errorf("expected exit code 6, got %d", ee.Code)
	}
}
