package ops

import (
	"context"
	"io"
	"os/exec"
)

// newCmd is a thin os/exec wrapper used only by ops verbs that shell
// out to standalone CLIs (git, hugo, etc.). Domain logic that needs
// to be testable should go through an adapter interface; this helper
// is reserved for low-level invocations whose contract is "exit 0
// = success" with no captured output.
func newCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

// drainCmd discards an exec.Cmd's stdout and stderr so the child
// process's pipes don't block. Returns the wait error.
func drainCmd(cmd *exec.Cmd) error {
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}
