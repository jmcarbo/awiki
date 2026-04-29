package synth

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"awiki/internal/adapters"
)

// fakePostHook is a test double for adapters.PostHook.
type fakePostHook struct {
	out  string
	code int
	err  error
}

func (f *fakePostHook) Run(_ context.Context, _, _ string) (string, int, error) {
	return f.out, f.code, f.err
}

var _ adapters.PostHook = (*fakePostHook)(nil)

func TestRunPluginPostHookNoHook(t *testing.T) {
	r := &Runner{}
	var buf strings.Builder
	plugin := Plugin{PostHook: ""}
	if err := r.RunPluginPostHook(context.Background(), plugin, "/page.md", &buf); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no stderr output, got %q", buf.String())
	}
}

func TestRunPluginPostHookGatedBlocked(t *testing.T) {
	r := &Runner{
		Config: map[string]string{"ALLOW_PLUGIN_POST_HOOKS": "0"},
	}
	var buf strings.Builder
	// Use a non-existent path; gating should block before stat.
	plugin := Plugin{PostHook: "/nonexistent/hook.sh"}
	if err := r.RunPluginPostHook(context.Background(), plugin, "/page.md", &buf); err != nil {
		t.Fatalf("expected nil error when gated, got %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "post-hook-blocked") {
		t.Errorf("expected post-hook-blocked in stderr, got %q", got)
	}
}

func TestRunPluginPostHookSuccess(t *testing.T) {
	// Create a real temporary script so os.Stat succeeds.
	dir := t.TempDir()
	script := filepath.Join(dir, "hook.sh")
	if err := os.WriteFile(script, []byte("#!/bin/bash\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	hook := &fakePostHook{out: "", code: 0, err: nil}
	r := &Runner{
		Config:   map[string]string{"ALLOW_PLUGIN_POST_HOOKS": "1"},
		PostHook: hook,
	}
	var buf strings.Builder
	plugin := Plugin{PostHook: script}
	if err := r.RunPluginPostHook(context.Background(), plugin, "/page.md", &buf); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestRunPluginPostHookFailure(t *testing.T) {
	// Create a real temporary script so os.Stat succeeds.
	dir := t.TempDir()
	script := filepath.Join(dir, "hook.sh")
	if err := os.WriteFile(script, []byte("#!/bin/bash\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	hook := &fakePostHook{out: "error output", code: 1, err: nil}
	r := &Runner{
		Config:   map[string]string{"ALLOW_PLUGIN_POST_HOOKS": "1"},
		PostHook: hook,
	}
	var buf strings.Builder
	plugin := Plugin{PostHook: script}
	err := r.RunPluginPostHook(context.Background(), plugin, "/page.md", &buf)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	got := buf.String()
	if !strings.Contains(got, "post-hook-failed") {
		t.Errorf("expected post-hook-failed in stderr, got %q", got)
	}
}
