package adapters

import (
	"context"
	"errors"
	"os/exec"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) (output string, code int, err error)
	RunInDir(ctx context.Context, dir string, name string, args ...string) (output string, code int, err error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
	return ExecRunner{}.RunInDir(ctx, "", name, args...)
}

func (ExecRunner) RunInDir(ctx context.Context, dir string, name string, args ...string) (string, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
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

func LegacyLint(ctx context.Context, r Runner, repoRoot string, only string, onlyFile string, contentDir string, fix bool) (string, int, error) {
	args := []string{"AWIKI_LINT_LEGACY=1"}
	if repoRoot != "" {
		args = append(args, "AWIKI_REPO_ROOT="+repoRoot)
	}
	args = append(args, "bash", "scripts/lint.sh")
	if only != "" {
		args = append(args, "--only="+only)
	}
	if fix {
		args = append(args, "--fix")
	}
	if onlyFile != "" {
		args = append(args, "--file="+onlyFile)
	}
	if contentDir != "" {
		args = append(args, contentDir)
	}
	return r.RunInDir(ctx, repoRoot, "env", args...)
}

func HugoCheck(ctx context.Context, r Runner) (string, int, error) {
	return r.Run(ctx, "hugo", "--source", ".", "--renderToMemory", "--logLevel", "error")
}
