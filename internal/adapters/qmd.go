package adapters

import (
	"context"
	"errors"
	"os/exec"
)

// ExecQmd is the production Qmd adapter. It shells to the `qmd`
// binary on PATH. Tests inject a fake.
type ExecQmd struct{}

func (ExecQmd) Search(ctx context.Context, repoRoot, query string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "qmd", "search", "--", query)
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

func (ExecQmd) Reindex(ctx context.Context, repoRoot string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "qmd", "reindex")
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
