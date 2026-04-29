package adapters

import (
	"context"
	"errors"
	"os/exec"
	"time"
)

// PostHook runs a synth plugin post-hook script. The script receives
// `--page <pagePath>` as arguments. The implementation enforces a
// timeout; the caller is responsible for the
// ALLOW_PLUGIN_POST_HOOKS gate.
type PostHook interface {
	Run(ctx context.Context, scriptPath, pagePath string) (output string, code int, err error)
}

// ExecPostHook is the production implementation. Default timeout 60s.
type ExecPostHook struct {
	Timeout time.Duration
}

func (h ExecPostHook) Run(ctx context.Context, scriptPath, pagePath string) (string, int, error) {
	timeout := h.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, scriptPath, "--page", pagePath)
	out, err := cmd.CombinedOutput()
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return string(out), 124, runCtx.Err()
	}
	if err == nil {
		return string(out), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode(), err
	}
	return string(out), 127, err
}
