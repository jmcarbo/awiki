package adapters

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// TemplateOrchGit implements BootstrapStepGit and the broader git
// surface the template orchestrator (rerun-bootstrap-step / retrofit /
// merge / update / init) needs. It wraps `os/exec` calls in the same
// shape the bash oracle uses, with stdin/stdout/stderr inherited so
// progress / errors surface to the parent.
type TemplateOrchGit struct {
	// RepoRoot is the working directory for git invocations. All git
	// commands run with `git -C RepoRoot ...`.
	RepoRoot string
	// Stdout / Stderr default to os.Stdout/os.Stderr; tests can swap.
	Stdout io.Writer
	Stderr io.Writer
}

func (g TemplateOrchGit) stdout() io.Writer {
	if g.Stdout == nil {
		return os.Stdout
	}
	return g.Stdout
}

func (g TemplateOrchGit) stderr() io.Writer {
	if g.Stderr == nil {
		return os.Stderr
	}
	return g.Stderr
}

// HasUpdateBranch shells `git for-each-ref --format=%(refname:short)
// refs/heads/awiki-template-update/`. Mirrors the bash oracle.
func (g TemplateOrchGit) HasUpdateBranch() (bool, error) {
	cmd := exec.Command("git", "-C", g.RepoRoot, "for-each-ref",
		"--format=%(refname:short)", "refs/heads/awiki-template-update/")
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// Checkout shells `git -C <repo> checkout -q <ref>`.
func (g TemplateOrchGit) Checkout(ref string) error {
	cmd := exec.Command("git", "-C", g.RepoRoot, "checkout", "-q", ref)
	cmd.Stdout = g.stdout()
	cmd.Stderr = g.stderr()
	return cmd.Run()
}

// CheckoutNewBranch shells `git -C <repo> checkout -q -b <branch>`.
func (g TemplateOrchGit) CheckoutNewBranch(branch string) error {
	cmd := exec.Command("git", "-C", g.RepoRoot, "checkout", "-q", "-b", branch)
	cmd.Stdout = g.stdout()
	cmd.Stderr = g.stderr()
	return cmd.Run()
}

// DeleteBranch shells `git -C <repo> branch -D <branch>`. Best-effort
// (matches bash `|| true`); a non-zero exit is silently ignored.
func (g TemplateOrchGit) DeleteBranch(branch string) error {
	cmd := exec.Command("git", "-C", g.RepoRoot, "branch", "-D", branch)
	_ = cmd.Run()
	return nil
}

// AddAll shells `git -C <repo> add -A`.
func (g TemplateOrchGit) AddAll() error {
	cmd := exec.Command("git", "-C", g.RepoRoot, "add", "-A")
	cmd.Stdout = g.stdout()
	cmd.Stderr = g.stderr()
	return cmd.Run()
}

// DiffCachedQuiet shells `git -C <repo> diff --cached --quiet` and
// reports whether the index is clean (exit 0).
func (g TemplateOrchGit) DiffCachedQuiet() (bool, error) {
	cmd := exec.Command("git", "-C", g.RepoRoot, "diff", "--cached", "--quiet")
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		// Exit 1 = differences present; >1 = error.
		if exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("git diff --cached --quiet: exit %d", exitErr.ExitCode())
	}
	return false, err
}

// Commit shells `git -C <repo> commit -q -m <msg>`.
func (g TemplateOrchGit) Commit(msg string) error {
	cmd := exec.Command("git", "-C", g.RepoRoot, "commit", "-q", "-m", msg)
	cmd.Stdout = g.stdout()
	cmd.Stderr = g.stderr()
	return cmd.Run()
}

// TemplateOrchBash implements BootstrapStepBash.
type TemplateOrchBash struct {
	Stdout io.Writer
	Stderr io.Writer
}

// RunBashScript shells `bash <script>` with env replacing the inherited
// environment entirely (matches `env -i`).
func (b TemplateOrchBash) RunBashScript(script string, env []string) (int, error) {
	cmd := exec.Command("bash", script)
	cmd.Env = env
	cmd.Stdout = b.Stdout
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = b.Stderr
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		// Match bash oracle's `|| true` semantics: a non-zero step
		// body exit is reported as code != 0 but not a fatal error.
		return exitErr.ExitCode(), nil
	}
	return 127, err
}

// TemplateOrchConfirm implements BootstrapStepConfirm by reading a
// single line from stdin and treating any answer starting with y/Y as
// affirmative. Mirrors the bash `read -r -p "Run? [y/N] " ans; [[ "$ans"
// =~ ^[Yy] ]]` shape.
type TemplateOrchConfirm struct {
	In  io.Reader
	Out io.Writer
}

// Confirm prints prompt to Out and reads one line from In. Returns true
// when the line begins with y or Y.
func (c TemplateOrchConfirm) Confirm(prompt string) (bool, error) {
	out := c.Out
	if out == nil {
		out = os.Stderr
	}
	if _, err := io.WriteString(out, prompt); err != nil {
		return false, err
	}
	in := c.In
	if in == nil {
		in = os.Stdin
	}
	r := bufio.NewReader(in)
	line, err := r.ReadString('\n')
	if err != nil && len(line) == 0 {
		return false, nil
	}
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return false, nil
	}
	c0 := line[0]
	return c0 == 'y' || c0 == 'Y', nil
}

// ArchiveTreeToDir shells `git -C <repo> archive --format=tar HEAD |
// tar -x -C <dst>`. Mirrors the bash oracle (template-init.sh +
// template-update.sh `git archive ... | tar -x`).
func (g TemplateOrchGit) ArchiveTreeToDir(repoRoot, dst string) error {
	if repoRoot == "" {
		return fmt.Errorf("archive: repoRoot empty")
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	gitCmd := exec.Command("git", "-C", repoRoot, "archive", "--format=tar", "HEAD")
	tarCmd := exec.Command("tar", "-x", "-C", dst)
	pipe, err := gitCmd.StdoutPipe()
	if err != nil {
		return err
	}
	tarCmd.Stdin = pipe
	tarCmd.Stderr = g.stderr()
	gitCmd.Stderr = g.stderr()
	if err := tarCmd.Start(); err != nil {
		return err
	}
	if err := gitCmd.Run(); err != nil {
		_ = tarCmd.Wait()
		return err
	}
	return tarCmd.Wait()
}

// MergeFile shells `git merge-file --diff3 -L current -L base -L new
// <cur> <base> <new>`. Mirrors scripts/template-merge.sh + sync.py
// apply-three-way. Returns the merge-file exit code (0 = clean, >0 =
// conflict count).
func (g TemplateOrchGit) MergeFile(cur, base, newFile string) (int, error) {
	cmd := exec.Command("git", "merge-file", "--diff3",
		"-L", "current", "-L", "base", "-L", "new",
		cur, base, newFile)
	cmd.Stdout = g.stdout()
	cmd.Stderr = g.stderr()
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 127, err
}

// LsFilesIn shells `git -C <dir> ls-files`. Mirrors scripts/.../sync.py
// usage in apply-attributes.
func (g TemplateOrchGit) LsFilesIn(dir string) (string, int, error) {
	cmd := exec.Command("git", "-C", dir, "ls-files")
	out, err := cmd.Output()
	if err == nil {
		return string(out), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode(), nil
	}
	return "", 127, err
}

// CheckAttrAll shells `git -c core.attributesfile=<attrFile>
// check-attr -a -- <path>` with GIT_ATTR_NOSYSTEM=1. Mirrors the bash
// oracle in template-attr-audit.sh + sync.py.
func (g TemplateOrchGit) CheckAttrAll(attrFile, path string) (string, error) {
	cmd := exec.Command("git",
		"-c", "core.attributesfile="+attrFile,
		"check-attr", "-a", "--", path)
	cmd.Env = append(os.Environ(), "GIT_ATTR_NOSYSTEM=1")
	out, err := cmd.Output()
	if err == nil {
		return string(out), nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), nil
	}
	return "", err
}

// captureRun is a small helper that runs cmd and returns combined
// stderr in the error on non-zero exit. Reused by other adapter
// methods.
func captureRun(cmd *exec.Cmd) error {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("%s: exit %d: %s", cmd.Path, exitErr.ExitCode(),
			strings.TrimRight(stderr.String(), "\n"))
	}
	return err
}
