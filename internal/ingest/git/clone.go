package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"awiki/internal/adapters"
)

// IsSSH reports whether spec looks like an `git@host:owner/repo` URL.
// Mirrors `awiki_git_clone_is_ssh` (scripts/lib/git-clone.sh:4-6).
func IsSSH(spec string) bool {
	return strings.HasPrefix(spec, "git@")
}

// httpsRe matches `(http|https)://<host>/<rest>` URLs.
var httpsRe = regexp.MustCompile(`^https?://([^/]+)/(.+)$`)

// sshRe matches `git@<host>:<rest>` URLs.
var sshRe = regexp.MustCompile(`^git@([^:]+):(.+)$`)

// RepoKey derives the slug used for the per-repo state file and lock
// directory. Mirrors `awiki_git_clone_repo_key` (scripts/lib/git-clone.sh:8-47):
//
//   - local path or `file://` → `local-<basename without .git>`
//   - http(s):// host/owner/repo[.git] → `<host with dots replaced>-<owner-repo>`
//   - git@host:owner/repo[.git]        → same shape as http(s)
//
// repoRoot is used to resolve relative paths to absolute when the spec
// is a directory; this matches the bash form which `cd "$spec" && pwd`
// to canonicalize.
func RepoKey(repoRoot, spec string) (string, error) {
	if spec == "" {
		return "", fmt.Errorf("repo_key needs spec")
	}
	// Local path branch: the bash form checks `-d "$spec" || /-prefix
	// || ./-prefix || ../-prefix` first, then attempts `cd && pwd`.
	if isLocalPath(repoRoot, spec) {
		abs := spec
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(repoRoot, abs)
		}
		if rp, err := filepath.Abs(abs); err == nil {
			abs = rp
		}
		return "local-" + strings.TrimSuffix(filepath.Base(abs), ".git"), nil
	}
	// file:// URL.
	if strings.HasPrefix(spec, "file://") {
		p := strings.TrimPrefix(spec, "file://")
		return "local-" + strings.TrimSuffix(filepath.Base(p), ".git"), nil
	}
	if m := httpsRe.FindStringSubmatch(spec); m != nil {
		host := strings.ReplaceAll(m[1], ".", "-")
		rest := strings.TrimSuffix(m[2], ".git")
		rest = strings.ReplaceAll(rest, "/", "-")
		return host + "-" + rest, nil
	}
	if m := sshRe.FindStringSubmatch(spec); m != nil {
		host := strings.ReplaceAll(m[1], ".", "-")
		rest := strings.TrimSuffix(m[2], ".git")
		rest = strings.ReplaceAll(rest, "/", "-")
		return host + "-" + rest, nil
	}
	return "", fmt.Errorf("unrecognized repo spec: %s", spec)
}

func isLocalPath(repoRoot, spec string) bool {
	if strings.HasPrefix(spec, "/") || strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") {
		return true
	}
	// `-d "$spec"` test from bash. Check both raw and as-relative-to-repoRoot.
	if info, err := os.Stat(spec); err == nil && info.IsDir() {
		return true
	}
	if info, err := os.Stat(filepath.Join(repoRoot, spec)); err == nil && info.IsDir() {
		return true
	}
	return false
}

// CloneResolution is the (checkout, head, branch) triple `awiki_git_clone_resolve`
// emits in pipe-delimited form (scripts/lib/git-clone.sh:85).
type CloneResolution struct {
	Checkout      string // local on-disk checkout path
	HeadSHA       string // git rev-parse HEAD
	DefaultBranch string // abbrev-ref or origin/HEAD symbolic-ref
}

// ResolveCheckout takes a `<repo-spec>` and produces a local working
// copy under repoRoot, returning the path + HEAD SHA + default branch.
// Mirrors `awiki_git_clone_resolve` (scripts/lib/git-clone.sh:49-86):
//
//   - local path (or `file://`) → use spec as-is.
//   - remote URL → clone (fresh) or fetch+reset (existing) under
//     `raw/_git-cache/<repo_key>`.
//   - HEAD sha + branch derived via rev-parse / symbolic-ref / abbrev-ref.
//
// Exit codes 10 / 11 from the bash form become *ResolveError in Go;
// callers map back to the bash exit codes via .Code.
func ResolveCheckout(ctx context.Context, repoRoot string, gitExt adapters.GitExt, spec, repoKey string) (CloneResolution, error) {
	if spec == "" || repoKey == "" {
		return CloneResolution{}, fmt.Errorf("resolve needs <spec> <repo_key>")
	}

	checkout := ""
	if isLocalPath(repoRoot, spec) {
		abs := spec
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(repoRoot, abs)
		}
		rp, err := filepath.Abs(abs)
		if err != nil {
			return CloneResolution{}, fmt.Errorf("abs %s: %w", abs, err)
		}
		checkout = rp
	} else if strings.HasPrefix(spec, "file://") {
		checkout = strings.TrimPrefix(spec, "file://")
	} else {
		// Remote: clone or fetch under raw/_git-cache/.
		cacheDir := filepath.Join(repoRoot, "raw", "_git-cache", repoKey)
		gitDir := filepath.Join(cacheDir, ".git")
		if _, err := os.Stat(gitDir); err != nil {
			if !os.IsNotExist(err) {
				return CloneResolution{}, &ResolveError{Code: 10, Err: err}
			}
			if err := os.MkdirAll(filepath.Join(repoRoot, "raw", "_git-cache"), 0o755); err != nil {
				return CloneResolution{}, &ResolveError{Code: 10, Err: err}
			}
			// `git clone --filter=blob:none --quiet $spec $checkout` (git-clone.sh:65).
			if code, err := gitExt.Clone(ctx, spec, cacheDir); err != nil || code != 0 {
				if err == nil {
					err = fmt.Errorf("git clone exited %d", code)
				}
				return CloneResolution{}, &ResolveError{Code: 10, Err: err}
			}
		} else {
			// Existing checkout: fetch + reset.
			if code, err := gitExt.Fetch(ctx, cacheDir, "origin"); err != nil || code != 0 {
				if err == nil {
					err = fmt.Errorf("git fetch exited %d", code)
				}
				return CloneResolution{}, &ResolveError{Code: 11, Err: err}
			}
			defaultBranch, _, _ := gitExt.SymbolicRef(ctx, cacheDir, "refs/remotes/origin/HEAD")
			defaultBranch = strings.TrimPrefix(defaultBranch, "origin/")
			if defaultBranch == "" {
				defaultBranch = "main"
			}
			if code, err := gitExt.Reset(ctx, cacheDir, "origin/"+defaultBranch); err != nil || code != 0 {
				if err == nil {
					err = fmt.Errorf("git reset exited %d", code)
				}
				return CloneResolution{}, &ResolveError{Code: 11, Err: err}
			}
		}
		checkout = cacheDir
	}

	sha, _, err := gitExt.RevParse(ctx, checkout, "HEAD")
	if err != nil || sha == "" {
		if err == nil {
			err = fmt.Errorf("git rev-parse HEAD returned empty")
		}
		return CloneResolution{}, &ResolveError{Code: 11, Err: err}
	}
	// Default-branch resolution. Bash form (git-clone.sh:79-84) tries
	// `rev-parse --abbrev-ref HEAD` first, falling back to
	// `symbolic-ref --short refs/remotes/origin/HEAD` on a detached
	// HEAD. The GitExt interface only exposes single-arg rev-parse, so
	// we use symbolic-ref against HEAD as the equivalent-success path
	// (returns the branch when HEAD points at a ref, errors on
	// detached HEAD). The detached-HEAD fallback then queries the
	// remote symbolic ref. Final fallback: "main".
	branch := ""
	if sym, _, _ := gitExt.SymbolicRef(ctx, checkout, "HEAD"); sym != "" {
		branch = sym
	}
	if branch == "" {
		sym, _, _ := gitExt.SymbolicRef(ctx, checkout, "refs/remotes/origin/HEAD")
		sym = strings.TrimPrefix(sym, "origin/")
		if sym != "" {
			branch = sym
		}
	}
	if branch == "" {
		branch = "main"
	}
	return CloneResolution{
		Checkout:      checkout,
		HeadSHA:       sha,
		DefaultBranch: branch,
	}, nil
}

// ResolveError pairs a bash-equivalent exit code with the underlying
// error so the slice 8 driver can surface the same exit code as bash.
type ResolveError struct {
	Code int
	Err  error
}

func (e *ResolveError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("resolve failed (rc=%d)", e.Code)
	}
	return fmt.Sprintf("resolve failed (rc=%d): %v", e.Code, e.Err)
}

func (e *ResolveError) ExitCode() int { return e.Code }

func (e *ResolveError) Unwrap() error { return e.Err }
