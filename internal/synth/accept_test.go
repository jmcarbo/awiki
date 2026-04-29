package synth

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"awiki/internal/adapters"
)

// fakeLint is a test double for adapters.SynthLint.
type fakeLint struct {
	exit int
}

func (f fakeLint) Run(_ context.Context, _ string, _ ...string) (string, int, error) {
	if f.exit == 0 {
		return "", 0, nil
	}
	return "lint failure\n", f.exit, fmt.Errorf("exit %d", f.exit)
}

var _ adapters.SynthLint = fakeLint{}

// makeAcceptRunner builds a temp-dir-backed Runner for accept-stage tests.
func makeAcceptRunner(t *testing.T, lint adapters.SynthLint) (dir string, r *Runner) {
	t.Helper()
	dir = t.TempDir()
	contentDir := filepath.Join(dir, "content")
	synthDir := filepath.Join(contentDir, "synthesis")
	stagedDir := filepath.Join(synthDir, ".staged")
	pluginDir := filepath.Join(dir, "plugins")
	_ = os.MkdirAll(synthDir, 0o755)
	_ = os.MkdirAll(stagedDir, 0o755)
	_ = os.MkdirAll(pluginDir, 0o755)

	r = &Runner{
		RepoRoot:   dir,
		ContentDir: contentDir,
		PluginDir:  pluginDir,
		Lint:       lint,
		Config:     map[string]string{},
	}
	return dir, r
}

// writeStagedFile writes content to the staged directory.
func writeStagedFile(t *testing.T, r *Runner, slug, content string) string {
	t.Helper()
	stagedDir := filepath.Join(r.synthDir(), ".staged")
	path := filepath.Join(stagedDir, slug+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeStagedFile: %v", err)
	}
	return path
}

// minimalStagedPage returns a minimal synthesis page body suitable for staged files.
func minimalStagedPage(plugin string) string {
	return "---\ntitle: test\ntype: synthesis\nplugin: " + plugin + "\n---\n\nLead.\n\n" +
		"<!-- BEGIN GENERATED plugin=" + plugin + " scope_hash=abc123 -->\n\nGenerated.\n\n<!-- END GENERATED -->\n"
}

// ---------------------------------------------------------------------------
// Test 1: Missing staged → exit 7. Lint must NOT be invoked.
// ---------------------------------------------------------------------------

func TestAcceptStageMissingStagedExit7(t *testing.T) {
	_, r := makeAcceptRunner(t, fakeLint{exit: 0})

	var stderr bytes.Buffer
	err := r.AcceptStage(context.Background(), "no-such-slug", &stderr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	ee, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T: %v", err, err)
	}
	if ee.Code != 7 {
		t.Errorf("expected exit code 7, got %d", ee.Code)
	}
	if !strings.Contains(ee.Msg, "no staged file at") {
		t.Errorf("message should mention 'no staged file at', got %q", ee.Msg)
	}
}

// ---------------------------------------------------------------------------
// Test 2: Lint fails → exit 6, live file untouched.
// ---------------------------------------------------------------------------

func TestAcceptStageLintFailsExit6(t *testing.T) {
	_, r := makeAcceptRunner(t, fakeLint{exit: 1})

	content := minimalStagedPage("briefing")
	writeStagedFile(t, r, "my-synth", content)

	// Pre-create a live file so we can verify it stays untouched.
	livePath := filepath.Join(r.synthDir(), "my-synth.md")
	origContent := "original live content\n"
	if err := os.WriteFile(livePath, []byte(origContent), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	err := r.AcceptStage(context.Background(), "my-synth", &stderr)
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

	// Live file must still have original content.
	got, _ := os.ReadFile(livePath)
	if string(got) != origContent {
		t.Errorf("live file was modified; expected %q, got %q", origContent, string(got))
	}
}

// ---------------------------------------------------------------------------
// Test 3: Happy path, no post-hook — staged moves to live, staged removed,
// stderr record emitted.
// ---------------------------------------------------------------------------

func TestAcceptStageHappyPathNoPostHook(t *testing.T) {
	_, r := makeAcceptRunner(t, fakeLint{exit: 0})

	// Write a plugin with no post_hook.
	pluginBody := "---\nname: briefing\nversion: 1\ndescription: test\noutput_type: note\n---\nBody.\n"
	if err := os.WriteFile(filepath.Join(r.PluginDir, "briefing.md"), []byte(pluginBody), 0o644); err != nil {
		t.Fatal(err)
	}

	content := minimalStagedPage("briefing")
	stagedPath := writeStagedFile(t, r, "my-synth", content)
	livePath := filepath.Join(r.synthDir(), "my-synth.md")

	var stderr bytes.Buffer
	err := r.AcceptStage(context.Background(), "my-synth", &stderr)
	if err != nil {
		t.Fatalf("AcceptStage error: %v", err)
	}

	// Live file must contain the staged content.
	got, readErr := os.ReadFile(livePath)
	if readErr != nil {
		t.Fatalf("live file not found: %v", readErr)
	}
	if string(got) != content {
		t.Errorf("live file content mismatch:\ngot  %q\nwant %q", string(got), content)
	}

	// Staged file must be removed.
	if _, statErr := os.Stat(stagedPath); statErr == nil {
		t.Error("staged file still exists after AcceptStage")
	}

	// Stderr must contain the telemetry record.
	stderrStr := stderr.String()
	wantStderr := "SYNTH-ACCEPT-STAGE|target=" + livePath
	if !strings.Contains(stderrStr, wantStderr) {
		t.Errorf("stderr missing telemetry:\ngot  %q\nwant %q", stderrStr, wantStderr)
	}
}

// ---------------------------------------------------------------------------
// Test 4: Post-hook gated off — live promoted, exit 0, blocked notice emitted.
// ---------------------------------------------------------------------------

func TestAcceptStagePostHookGatedOff(t *testing.T) {
	_, r := makeAcceptRunner(t, fakeLint{exit: 0})
	r.Config["ALLOW_PLUGIN_POST_HOOKS"] = ""

	// Create a temp script file so os.Stat succeeds inside RunPluginPostHook.
	dir := t.TempDir()
	hookScript := filepath.Join(dir, "hook.sh")
	if err := os.WriteFile(hookScript, []byte("#!/bin/bash\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a plugin with a post_hook pointing at the temp script.
	pluginBody := "---\nname: briefing\nversion: 1\ndescription: test\noutput_type: note\npost_hook: " + hookScript + "\n---\nBody.\n"
	if err := os.WriteFile(filepath.Join(r.PluginDir, "briefing.md"), []byte(pluginBody), 0o644); err != nil {
		t.Fatal(err)
	}

	// hook adapter should NOT be called.
	hook := &fakePostHook{code: 0}
	r.PostHook = hook

	content := minimalStagedPage("briefing")
	writeStagedFile(t, r, "gated-synth", content)
	livePath := filepath.Join(r.synthDir(), "gated-synth.md")

	var stderr bytes.Buffer
	err := r.AcceptStage(context.Background(), "gated-synth", &stderr)
	if err != nil {
		t.Fatalf("AcceptStage should succeed when post-hook gated; got error: %v", err)
	}

	// Live must be written.
	if _, statErr := os.Stat(livePath); statErr != nil {
		t.Errorf("live file not created: %v", statErr)
	}

	// Stderr should contain the blocked notice (emitted by RunPluginPostHook).
	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "post-hook-blocked") {
		t.Errorf("expected post-hook-blocked notice in stderr, got %q", stderrStr)
	}
}

// ---------------------------------------------------------------------------
// Test 5: Post-hook fails when allowed → exit 6.
// ---------------------------------------------------------------------------

func TestAcceptStagePostHookFailsExit6(t *testing.T) {
	_, r := makeAcceptRunner(t, fakeLint{exit: 0})
	r.Config["ALLOW_PLUGIN_POST_HOOKS"] = "1"

	// Create a real temp script file so os.Stat succeeds.
	dir := t.TempDir()
	hookScript := filepath.Join(dir, "hook.sh")
	if err := os.WriteFile(hookScript, []byte("#!/bin/bash\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a plugin with a post_hook.
	pluginBody := "---\nname: briefing\nversion: 1\ndescription: test\noutput_type: note\npost_hook: " + hookScript + "\n---\nBody.\n"
	if err := os.WriteFile(filepath.Join(r.PluginDir, "briefing.md"), []byte(pluginBody), 0o644); err != nil {
		t.Fatal(err)
	}

	// Hook adapter returns non-zero.
	hook := &fakePostHook{out: "fail\n", code: 1, err: fmt.Errorf("exit 1")}
	r.PostHook = hook

	content := minimalStagedPage("briefing")
	writeStagedFile(t, r, "hook-fail-synth", content)

	var stderr bytes.Buffer
	err := r.AcceptStage(context.Background(), "hook-fail-synth", &stderr)
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
