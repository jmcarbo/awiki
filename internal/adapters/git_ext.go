package adapters

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
)

// GitExt extends the basic Git adapter with the clone/init/fetch and
// inspection operations required by `awiki ingest-git`. The bash oracle
// (scripts/lib/git-clone.sh + scripts/ingest-git.sh) drives a fixed set
// of git invocations against a working checkout — Clone, Fetch, Reset,
// RevParse, SymbolicRef, LsTree — and this interface mirrors that shape
// 1:1 so the slice 8 driver can swap in a fake without touching the
// production exec adapter.
//
// Subsequent slices (specifically slice 8 ingest-git) wire the real
// `os/exec` invocations; earlier slices keep the stubs returning
// ErrNotImplemented.
type GitExt interface {
	Git
	Init(ctx context.Context, dir string) (code int, err error)
	// Fetch shells `git -C <dir> fetch --quiet --prune <remote>`.
	// Mirrors scripts/lib/git-clone.sh:68 — `git -C "$checkout"
	// fetch --quiet --prune origin`.
	Fetch(ctx context.Context, dir, remote string) (code int, err error)
	// Reset shells `git -C <dir> reset --quiet --hard <ref>`.
	// Mirrors scripts/lib/git-clone.sh:72 — `git -C "$checkout"
	// reset --quiet --hard "origin/${default_branch}"`.
	Reset(ctx context.Context, dir, ref string) (code int, err error)
	// SymbolicRef shells `git -C <dir> symbolic-ref --short <name>`.
	// Mirrors scripts/lib/git-clone.sh:69 — used to resolve
	// `refs/remotes/origin/HEAD` to the upstream default branch.
	SymbolicRef(ctx context.Context, dir, name string) (out string, code int, err error)
	// LsTree shells `git -C <dir> ls-tree -r <ref> --name-only`.
	// Mirrors scripts/ingest-git.sh:87 — `git -C "$CHECKOUT"
	// ls-tree -r HEAD --name-only`.
	LsTree(ctx context.Context, dir, ref string) (out string, code int, err error)
}

// ExecGitExt is the production adapter. Slice 1-7 returned
// ErrNotImplemented; slice 8 wires `os/exec` calls byte-for-byte
// matching the flag strings the bash oracle uses.
type ExecGitExt struct{}

// Clone shells `git clone --filter=blob:none --quiet <url> <dest>`.
// Bash form: scripts/lib/git-clone.sh:65 — the `--filter=blob:none` is
// the partial-clone optimization the bash uses to avoid pulling blobs
// for files we do not extract. Returns the captured stderr in the error
// when the command exits non-zero so callers can surface meaningful
// messages without spelunking exec internals.
func (ExecGitExt) Clone(ctx context.Context, url, dest string) (int, error) {
	cmd := exec.CommandContext(ctx, "git", "clone", "--filter=blob:none", "--quiet", url, dest)
	return runGitCmd(cmd)
}

// RevParse shells `git -C <dir> rev-parse <ref>`. Used by both
// `awiki_git_clone_resolve` (rev-parse HEAD → SHA) and the diff
// loop (rev-parse "HEAD:<path>" → blob SHA) at scripts/ingest-git.sh:116.
// The trimmed stdout is returned; a non-zero exit yields the captured
// stderr as the error.
func (ExecGitExt) RevParse(ctx context.Context, dir, ref string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", ref)
	out, code, err := runGitCmdCapture(cmd)
	return strings.TrimRight(out, "\n"), code, err
}

// Init shells `git -C <dir> init --quiet`. Used today only by tests and
// the test-fake parity — the bash oracle does not call `git init` in
// the ingest-git flow but the interface keeps the entry-point because
// upstream Git plans (test fixtures that set up synthetic checkouts)
// rely on it.
func (ExecGitExt) Init(ctx context.Context, dir string) (int, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "init", "--quiet")
	return runGitCmd(cmd)
}

// Fetch shells `git -C <dir> fetch --quiet --prune <remote>`. Bash form:
// scripts/lib/git-clone.sh:68.
func (ExecGitExt) Fetch(ctx context.Context, dir, remote string) (int, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "fetch", "--quiet", "--prune", remote)
	return runGitCmd(cmd)
}

// Reset shells `git -C <dir> reset --quiet --hard <ref>`. Bash form:
// scripts/lib/git-clone.sh:72.
func (ExecGitExt) Reset(ctx context.Context, dir, ref string) (int, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "reset", "--quiet", "--hard", ref)
	return runGitCmd(cmd)
}

// SymbolicRef shells `git -C <dir> symbolic-ref --short <name>`. Bash
// form: scripts/lib/git-clone.sh:69 (default-branch resolution).
func (ExecGitExt) SymbolicRef(ctx context.Context, dir, name string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "symbolic-ref", "--short", name)
	out, code, err := runGitCmdCapture(cmd)
	return strings.TrimRight(out, "\n"), code, err
}

// LsTree shells `git -C <dir> ls-tree -r <ref> --name-only`. Bash form:
// scripts/ingest-git.sh:87. Returns the raw newline-separated stdout so
// callers can split + sort identically to the bash mapfile.
func (ExecGitExt) LsTree(ctx context.Context, dir, ref string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "ls-tree", "-r", ref, "--name-only")
	return runGitCmdCapture(cmd)
}

// runGitCmd runs a git command for its side-effects, returning (code,
// err). Mirrors the conventions of ExecPDFToText.Extract: a non-zero
// exit yields the captured stderr as the error string; an exec failure
// (binary not on PATH) returns code 127.
func runGitCmd(cmd *exec.Cmd) (int, error) {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), errors.New(strings.TrimRight(stderr.String(), "\n"))
	}
	return 127, err
}

// runGitCmdCapture is the stdout-capturing variant. Returns (stdout,
// code, err). Stderr is preserved in the error on non-zero exit.
func runGitCmdCapture(cmd *exec.Cmd) (string, int, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		msg := strings.TrimRight(stderr.String(), "\n")
		if msg == "" {
			msg = strings.TrimRight(stdout.String(), "\n")
		}
		return stdout.String(), exitErr.ExitCode(), errors.New(msg)
	}
	return "", 127, err
}
