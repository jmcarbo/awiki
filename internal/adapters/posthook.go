package adapters

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// PostHook runs a synth plugin post-hook script or command. When the
// scriptPath is a single token (no whitespace) the adapter calls it
// as `<scriptPath> --page <pagePath>` for backwards compatibility
// with the bash-era `<script.sh> --page <page>` shape. When the
// scriptPath holds multiple whitespace-separated tokens (e.g.
// `awiki synth-mindmap-validate`) the adapter splits on spaces,
// runs the first token as the program, and appends `-- <pagePath>`.
// The bash-era `script.sh -- <page>` shape is preserved that way.
//
// The caller is responsible for the ALLOW_PLUGIN_POST_HOOKS gate.
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

	tokens := strings.Fields(scriptPath)
	var cmd *exec.Cmd
	switch len(tokens) {
	case 0:
		return "", 1, errors.New("empty post-hook command")
	case 1:
		// Legacy single-token form: <script> --page <pagePath>
		cmd = exec.CommandContext(runCtx, tokens[0], "--page", pagePath)
	default:
		// Command form: <prog> <args...> -- <pagePath>
		args := append([]string{}, tokens[1:]...)
		args = append(args, "--", pagePath)
		cmd = exec.CommandContext(runCtx, tokens[0], args...)
	}
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
