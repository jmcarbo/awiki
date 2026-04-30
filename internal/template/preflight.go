package template

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PreflightGit captures the git operations the preflight checks need.
// The bash + Python oracle invokes `git diff-index`, `git ls-files
// --others --exclude-standard`, `git submodule status`, `git rev-parse
// --abbrev-ref HEAD`, and (optionally) `git-crypt status`. Each method
// returns (stdout, exit-code, error). A nil error with non-zero code is
// treated as command failure (matching the oracle); a non-nil error
// indicates the binary is missing entirely (oracle exit 127).
type PreflightGit interface {
	// DiffIndexQuiet returns 0 when the working tree matches HEAD,
	// non-zero when staged or unstaged tracked changes exist.
	DiffIndexQuiet() (code int, err error)
	// LsFilesUntracked returns the list of untracked files (one per
	// line) honoring .gitignore.
	LsFilesUntracked() (out string, code int, err error)
	// SubmoduleStatus returns the raw `git submodule status` output.
	SubmoduleStatus() (out string, code int, err error)
	// CurrentBranch returns `git rev-parse --abbrev-ref HEAD`.
	CurrentBranch() (out string, code int, err error)
	// GitCryptStatusEncrypted returns the output of `git-crypt status
	// -e` (one path per line). A code of 127 (binary missing) signals
	// the oracle should skip the encryption check entirely.
	GitCryptStatusEncrypted() (out string, code int, err error)
	// GitCryptStatus returns plain `git-crypt status` output. The
	// oracle inspects it for the substring "locked" to decide whether
	// the checkout is locked.
	GitCryptStatus() (out string, code int, err error)
}

// CheckTree mirrors `preflight.py check-tree`. Returns nil for clean,
// otherwise an error whose message matches the Python stderr line.
// Errors from missing binaries are surfaced as ErrPreflightToolMissing.
func CheckTree(g PreflightGit) error {
	if g == nil {
		return errors.New("preflight: git adapter is nil")
	}
	code, err := g.DiffIndexQuiet()
	if err != nil {
		return err
	}
	if code != 0 {
		return errors.New("dirty tree: staged or unstaged tracked changes (modified)")
	}
	out, code, err := g.LsFilesUntracked()
	if err != nil {
		return err
	}
	if code == 0 && strings.TrimSpace(out) != "" {
		return fmt.Errorf("dirty tree: untracked files\n%s", out)
	}
	out, code, err = g.SubmoduleStatus()
	if err != nil {
		return err
	}
	if code == 0 {
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
				return fmt.Errorf("dirty tree: submodule %s", line)
			}
		}
	}
	return nil
}

// CheckBranch mirrors `preflight.py check-branch --expected <name>`.
// Returns:
//   - nil when current == expected and current does not start with
//     "awiki-template-update/".
//   - ErrPreflightNotInRepo when `git rev-parse --abbrev-ref HEAD`
//     fails (oracle exit 2).
//   - a generic error for the halt cases (oracle exit 1).
func CheckBranch(g PreflightGit, expected string) error {
	if g == nil {
		return errors.New("preflight: git adapter is nil")
	}
	out, code, err := g.CurrentBranch()
	if err != nil || code != 0 {
		return ErrPreflightNotInRepo
	}
	cur := strings.TrimSpace(out)
	if strings.HasPrefix(cur, "awiki-template-update/") {
		return fmt.Errorf("halt: on update branch %s; use --continue or --abort", cur)
	}
	if cur != expected {
		return fmt.Errorf("halt: on branch %s, expected %s", cur, expected)
	}
	return nil
}

// CheckPendingPrompts mirrors `preflight.py check-pending-prompts`.
// Returns nil for clean (no .awiki/pending-prompts directory or no .md
// files) and a halt error listing the file basenames otherwise.
func CheckPendingPrompts(repoRoot string) error {
	pp := filepath.Join(repoRoot, ".awiki", "pending-prompts")
	info, err := os.Stat(pp)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(pp)
	if err != nil {
		return err
	}
	var leftovers []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".md") {
			leftovers = append(leftovers, e.Name())
		}
	}
	if len(leftovers) == 0 {
		return nil
	}
	sort.Strings(leftovers)
	var b strings.Builder
	b.WriteString("halt: pending LLM migrations from previous cycle:\n  ")
	b.WriteString(strings.Join(leftovers, "\n  "))
	b.WriteString("\nRun agent to complete or delete prompts before next update.")
	return errors.New(b.String())
}

// CheckEncryption mirrors `preflight.py check-encryption --manifest`.
// Returns nil when no encrypted paths require merging or when git-crypt
// is unavailable (silent skip). Returns a halt error when an encrypted
// path resolves to three_way / attributes_merge AND the checkout is
// locked.
func CheckEncryption(g PreflightGit, manifestPath string) error {
	if g == nil {
		return errors.New("preflight: git adapter is nil")
	}
	out, code, err := g.GitCryptStatusEncrypted()
	if err != nil {
		// Silent skip — Python catches everything except the explicit
		// 127 case below; treat any execution failure the same way.
		return nil
	}
	if code == 127 {
		// git-crypt binary not present.
		return nil
	}
	if strings.TrimSpace(out) == "" {
		return nil
	}
	statusOut, _, err := g.GitCryptStatus()
	if err != nil {
		return nil
	}
	if !strings.Contains(strings.ToLower(statusOut), "locked") {
		return nil
	}
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(out, "\n") {
		p := strings.TrimSpace(line)
		if p == "" {
			continue
		}
		s := manifest.ResolveStrategy(p)
		if s == "three_way" || s == "attributes_merge" {
			return fmt.Errorf(
				"halt: encrypted path %s would require merge but checkout is locked.\n"+
					"Run `git-crypt unlock` first.", p)
		}
	}
	return nil
}

// ErrPreflightNotInRepo is returned by CheckBranch when the underlying
// `git rev-parse` fails. Maps to Python exit code 2.
var ErrPreflightNotInRepo = errors.New("not in a git repo")

// ErrPreflightToolMissing is returned by adapters when a required tool
// (git, git-crypt) is missing entirely. Maps to Python exit code 127.
var ErrPreflightToolMissing = errors.New("preflight: required tool missing")
