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

func LegacyLint(ctx context.Context, r Runner, toolRoot string, wikiRoot string, only string, onlyFile string, contentDir string, fix bool) (string, int, error) {
	// Shells the awiki binary's `lint` verb directly. Historical
	// callers wrapped `bash scripts/lint.sh` with an `AWIKI_LINT_LEGACY=1`
	// env so the shim fell through to the legacy bash logic. With
	// every namespace ported to Go, the shim is gone and the env
	// gating with it — but we keep the AWIKI_REPO_ROOT pass-through
	// because the lint engine still consults it for content discovery.
	bin := ResolveAwikiBin(toolRoot)
	envArgs := []string{}
	if wikiRoot != "" {
		envArgs = append(envArgs, "AWIKI_REPO_ROOT="+wikiRoot)
	}
	envArgs = append(envArgs, bin, "lint")
	if only != "" {
		envArgs = append(envArgs, "--only="+only)
	}
	if fix {
		envArgs = append(envArgs, "--fix")
	}
	if onlyFile != "" {
		envArgs = append(envArgs, "--file="+onlyFile)
	}
	if contentDir != "" {
		envArgs = append(envArgs, contentDir)
	}
	return r.RunInDir(ctx, toolRoot, "env", envArgs...)
}

func HugoCheck(ctx context.Context, r Runner, repoRoot string) (string, int, error) {
	return r.RunInDir(ctx, repoRoot, "hugo", "--source", ".", "--renderToMemory", "--logLevel", "error")
}
