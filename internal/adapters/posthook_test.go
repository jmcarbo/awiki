package adapters

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExecPostHookRunsScriptAndCapturesOutput(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "hook.sh")
	body := "#!/usr/bin/env bash\necho \"hook: $@\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	h := ExecPostHook{Timeout: time.Second}
	out, code, err := h.Run(context.Background(), script, "/some/page.md")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != 0 {
		t.Fatalf("code: %d", code)
	}
	if got, want := out, "hook: --page /some/page.md\n"; got != want {
		t.Fatalf("out: got %q want %q", got, want)
	}
}

func TestExecPostHookHonorsTimeout(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "slow.sh")
	body := "#!/usr/bin/env bash\nsleep 2\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	h := ExecPostHook{Timeout: 100 * time.Millisecond}
	_, code, err := h.Run(context.Background(), script, "/p.md")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if code == 0 {
		t.Fatalf("code: %d (want non-zero)", code)
	}
}
