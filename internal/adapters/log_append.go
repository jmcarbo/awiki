package adapters

import (
	"context"
	"os/exec"
)

// LogAppend writes a journal entry to <repoRoot>/content/log.md by
// shelling scripts/log-append.sh. The bash form (scripts/ingest.sh:66)
// best-effort calls the script and ignores errors; this mirrors that.
//
// Future: a dedicated Go ops/log domain port (see TODO comments in
// internal/ingest/{bookkeep,capture}.go) will replace the shell-out
// with a native implementation. Until then, the adapter keeps the
// shim in one place so tests can fake it.
type LogAppend interface {
	Append(ctx context.Context, repoRoot, action, msg string) error
}

// ExecLogAppend is the production adapter — shells the bash script.
type ExecLogAppend struct{}

func (ExecLogAppend) Append(ctx context.Context, repoRoot, action, msg string) error {
	script := resolveScript(repoRoot, "log-append.sh")
	// Bash: bash $SCRIPT_DIR/log-append.sh <action> <msg...>
	// (scripts/ingest.sh:66 — no `--` separator; log-append.sh
	// concatenates remaining args with $*).
	cmd := exec.CommandContext(ctx, "bash", script, action, msg)
	cmd.Dir = repoRoot
	// Bash form pipes stderr to /dev/null inside ingest.sh's caller chain.
	// The adapter swallows the error here so a missing script does not
	// fail the bookkeep; non-fatal log loss is preferable to a broken
	// ingest path.
	_ = cmd.Run()
	return nil
}
