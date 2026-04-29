package adapters

import (
	"context"
	"errors"
	"os/exec"
)

// SynthGit is the minimal git CLI wrapper synth verbs use. ExecSynthGit
// implements it via os/exec; tests inject fakes.
type SynthGit interface {
	LsFiles(ctx context.Context, repoRoot, path string) (tracked bool, err error)
	ShowHead(ctx context.Context, repoRoot, path string) (content string, ok bool, err error)
}

type ExecSynthGit struct{}

func (ExecSynthGit) LsFiles(ctx context.Context, repoRoot, path string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-files", "--error-unmatch", "--", path)
	cmd.Dir = repoRoot
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (ExecSynthGit) ShowHead(ctx context.Context, repoRoot, path string) (string, bool, error) {
	if err := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "HEAD").Run(); err != nil {
		return "", false, nil
	}
	cmd := exec.CommandContext(ctx, "git", "show", "HEAD:"+path)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", false, nil
		}
		return "", false, err
	}
	return string(out), true, nil
}
