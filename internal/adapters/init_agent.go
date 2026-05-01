package adapters

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ExecInitAgent is the production AgentRunner used by `awiki init`'s
// patch-identity step. It shells `<cli> -p "<prompt>"` and returns
// the trimmed stdout. A non-zero exit yields the captured stderr
// embedded in the error so the dispatcher can surface useful
// diagnostics.
//
// The bash form of this contract is:
//
//	$ <cli> -p "<prompt>"
//
// claude/codex/opencode/gemini all accept the `-p` flag for an
// in-line prompt; users who use a different CLI can pass any binary
// on PATH and the same flag is appended.
type ExecInitAgent struct{}

// Run shells <cli> -p "<prompt>" and returns trimmed stdout.
// A nil/empty cli is rejected before exec runs.
func (ExecInitAgent) Run(ctx context.Context, cli, prompt string) (string, error) {
	if cli == "" {
		return "", errors.New("init-agent: cli is empty")
	}
	cmd := exec.CommandContext(ctx, cli, "-p", prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("%s -p: exit %d: %s",
				cli, exitErr.ExitCode(), strings.TrimRight(stderr.String(), "\n"))
		}
		return "", fmt.Errorf("%s -p: %w", cli, err)
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

// ExecInitGit is the production GitInit adapter used by the init
// verb. It mirrors TemplateOrchGit but covers the narrower surface
// the init steps need.
type ExecInitGit struct{}

// SubmoduleUpdate shells `git -C <repo> submodule update --init --recursive`.
func (ExecInitGit) SubmoduleUpdate(ctx context.Context, repoRoot string) (string, int, error) {
	return runCapture(ctx, repoRoot, "git", "submodule", "update", "--init", "--recursive")
}

// Status shells `git -C <repo> status --short`.
func (ExecInitGit) Status(ctx context.Context, repoRoot string) (string, int, error) {
	return runCapture(ctx, repoRoot, "git", "status", "--short")
}

// AddAll shells `git -C <repo> add -A`.
func (ExecInitGit) AddAll(ctx context.Context, repoRoot string) (int, error) {
	_, code, err := runCapture(ctx, repoRoot, "git", "add", "-A")
	return code, err
}

// Commit shells `git -C <repo> commit -m <msg>`.
func (ExecInitGit) Commit(ctx context.Context, repoRoot, msg string) (string, int, error) {
	return runCapture(ctx, repoRoot, "git", "commit", "-m", msg)
}

// SubmoduleDeinit shells `git -C <repo> submodule deinit -f <path>`.
func (ExecInitGit) SubmoduleDeinit(ctx context.Context, repoRoot, path string) (int, error) {
	_, code, err := runCapture(ctx, repoRoot, "git", "submodule", "deinit", "-f", path)
	return code, err
}

// SubmoduleRm shells `git -C <repo> rm -f <path>`.
func (ExecInitGit) SubmoduleRm(ctx context.Context, repoRoot, path string) (int, error) {
	_, code, err := runCapture(ctx, repoRoot, "git", "rm", "-f", path)
	return code, err
}

// SubmoduleAdd shells `git -C <repo> submodule add <url> <path>`.
func (ExecInitGit) SubmoduleAdd(ctx context.Context, repoRoot, url, path string) (int, error) {
	_, code, err := runCapture(ctx, repoRoot, "git", "submodule", "add", url, path)
	return code, err
}

// RevParseHEAD shells `git -C <repo> rev-parse HEAD`.
func (ExecInitGit) RevParseHEAD(ctx context.Context, repoRoot string) (string, error) {
	out, _, err := runCapture(ctx, repoRoot, "git", "rev-parse", "HEAD")
	return strings.TrimRight(out, "\n"), err
}

// ExecInitBash is the production BashRunner. Shells `bash <script>
// [args...]` with cwd=repoRoot and stdio inherited from the parent
// (so encrypt-init.sh can prompt for a passphrase).
type ExecInitBash struct{}

// Run shells `bash <script> [args...]`. Stdio is inherited so
// interactive prompts (gpg passphrases, age recipients) work.
func (ExecInitBash) Run(ctx context.Context, repoRoot, script string, args ...string) (string, int, error) {
	full := append([]string{script}, args...)
	return runCapture(ctx, repoRoot, "bash", full...)
}

// ExecInitNPM is the production NPMRunner. Shells `npm install
// --silent` in dir.
type ExecInitNPM struct{}

func (ExecInitNPM) Install(ctx context.Context, dir string) (string, int, error) {
	return runCapture(ctx, dir, "npm", "install", "--silent")
}

// runCapture is a small helper that runs a command, returning
// (combined-output, exit-code, err). Mirrors the convention every
// other adapter in this package follows.
func runCapture(ctx context.Context, dir, name string, args ...string) (string, int, error) {
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
