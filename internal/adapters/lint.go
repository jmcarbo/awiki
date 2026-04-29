package adapters

import (
	"context"
	"errors"
	"os/exec"
)

// SynthLint shells `bash <repoRoot>/scripts/lint.sh <args...>`.
type SynthLint interface {
	Run(ctx context.Context, repoRoot string, args ...string) (output string, code int, err error)
}

type ExecSynthLint struct{}

func (ExecSynthLint) Run(ctx context.Context, repoRoot string, args ...string) (string, int, error) {
	all := append([]string{"scripts/lint.sh"}, args...)
	cmd := exec.CommandContext(ctx, "bash", all...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode(), err
	}
	return string(out), 127, err
}
