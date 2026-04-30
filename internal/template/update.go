package template

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// UpdateOptions captures every flag template-update.sh exposes.
type UpdateOptions struct {
	Ref                     string
	Source                  string
	Apply                   bool
	DryRun                  bool
	Continue                bool
	Abort                   bool
	Status                  bool
	SchemaUpgrade           bool
	AcceptSourceChange      bool
	AcceptAttributeChanges  bool
	AcceptManualCommits     bool
	PersistSource           bool
	PrintMigrations         bool
	NonInteractive          bool
	VerifySignature         bool
	GC                      bool
	RePin                   string
	RerunBootstrapStep      string
	SkipMigration           string
}

// UpdateDeps bundles every adapter the update orchestrator needs. The
// production wiring lives under internal/adapters/template_orch.go;
// tests inject fakes per-method.
type UpdateDeps struct {
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader

	// Git provides the broad git surface (branch ops, commit, fetch,
	// archive, ls-files, check-attr, merge-file).
	Git UpdateGit
	// Bash runs migration scripts and bootstrap step bodies.
	Bash BootstrapStepBash
	// Confirm prompts the user (interactive-only flows).
	Confirm BootstrapStepConfirm
	// Preflight wires the preflight git interface used by phase 0.
	Preflight PreflightGit
}

// UpdateGit is the union of every git operation the update orchestrator
// needs across all phases.
type UpdateGit interface {
	BootstrapStepGit
	SyncGit
	PlanEmitGit
	InitArchiver
	// FetchClone clones source (URL or local path) to dest with depth
	// 50; falls back to copy + git init when source is a non-git path.
	FetchClone(source, dest string) error
	// FetchUpdate runs `git -C <dir> fetch --depth 50 origin`. Best-
	// effort; oracle ignores failures.
	FetchUpdate(dir string) error
	// RevParseHEAD returns the current HEAD SHA in dir.
	RevParseHEAD(dir string) (string, error)
	// RevParseRef returns the SHA for ref in dir.
	RevParseRef(dir, ref string) (string, error)
	// CurrentBranchName returns `git rev-parse --abbrev-ref HEAD` in
	// the orchestrator's repoRoot.
	CurrentBranchName() (string, error)
	// VerifyTag runs `git verify-tag <target>` in dir.
	VerifyTag(dir, target string) error
	// VerifyCommit runs `git verify-commit <target>` in dir.
	VerifyCommit(dir, target string) error
	// ResetHard runs `git reset --hard <ref>`.
	ResetHard(ref string) error
	// HasBranch returns true iff refs/heads/<branch> exists.
	HasBranch(branch string) (bool, error)
	// ArchiveTreeToDir is inherited from InitArchiver.
}

// gcCache rotates the cache, keeping the current pin + immediate
// previous (matches `cache_rotate.py`).
func gcCache(repoRoot, currentCommit string) error {
	return RotateCache(filepath.Join(repoRoot, ".awiki", "template-cache"), currentCommit)
}

// AbortUpdate handles `--abort`: deletes the in-progress branch +
// _fetch dir + restores default branch. Mirrors the bash oracle.
func AbortUpdate(deps UpdateDeps, repoRoot, defaultBranch string) error {
	fetchDir := filepath.Join(repoRoot, ".awiki", "template-cache", "_fetch")
	state := filepath.Join(fetchDir, ".update-state.json")
	if _, err := os.Stat(state); err == nil {
		// Read branch from state.
		branch, _ := GetStateField(state, "branch")
		if defaultBranch != "" {
			_ = deps.Git.Checkout(defaultBranch)
		}
		if branch != "" && branch != "null" {
			has, _ := deps.Git.HasBranch(branch)
			if has {
				_ = deps.Git.DeleteBranch(branch)
			}
		}
		_ = os.RemoveAll(fetchDir)
		fmt.Fprintf(deps.Stdout, "info: aborted update; %s restored\n", defaultBranch)
		return nil
	}
	// Even with no state, clean up bare _fetch if present.
	if _, err := os.Stat(fetchDir); err == nil {
		_ = os.RemoveAll(fetchDir)
	}
	fmt.Fprintln(deps.Stdout, "info: no in-progress update")
	return nil
}

// updateProvenanceFromState applies pending entries from state file
// into template.json (Commit D). Mirrors the inline Python heredoc.
func updateProvenanceFromState(pj, statePath string) error {
	pjData, err := os.ReadFile(pj)
	if err != nil {
		return err
	}
	var d map[string]any
	if err := json.Unmarshal(pjData, &d); err != nil {
		return err
	}
	stData, err := os.ReadFile(statePath)
	if err != nil {
		return err
	}
	var s map[string]any
	if err := json.Unmarshal(stData, &s); err != nil {
		return err
	}
	if pending, ok := s["applied_migrations_pending"].([]any); ok && len(pending) > 0 {
		appliedRaw, _ := d["applied_migrations"].([]any)
		appliedRaw = append(appliedRaw, pending...)
		d["applied_migrations"] = appliedRaw
	}
	if pending, ok := s["bootstrap_steps_pending"].([]any); ok && len(pending) > 0 {
		stepsRaw, _ := d["bootstrap_steps_done"].([]any)
		stepsRaw = append(stepsRaw, pending...)
		d["bootstrap_steps_done"] = stepsRaw
	}
	if pending, ok := s["deleted_pending"].([]any); ok && len(pending) > 0 {
		delRaw, _ := d["deleted"].([]any)
		delRaw = append(delRaw, pending...)
		d["deleted"] = delRaw
	}
	out, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(pj, out, 0o644)
}

// resolveCommitNew returns the SHA the update should advance to, given
// either an explicit ref or HEAD of the fetched dir.
func resolveCommitNew(g UpdateGit, fetchDir, ref string) (string, error) {
	if ref != "" {
		return g.RevParseRef(fetchDir, ref)
	}
	return g.RevParseHEAD(fetchDir)
}

// ensureFetchDir prepares .awiki/template-cache/_fetch with the source
// content. Mirrors the bash Phase 1 fetch block.
func ensureFetchDir(g UpdateGit, fetchDir, source string) error {
	if isDir(filepath.Join(fetchDir, ".git")) {
		// Reuse: best-effort fetch.
		_ = g.FetchUpdate(fetchDir)
		return nil
	}
	if isDir(fetchDir) {
		// Stale dir without .git — wipe.
		if err := os.RemoveAll(fetchDir); err != nil {
			return err
		}
	}
	return g.FetchClone(source, fetchDir)
}

// rebuildAncestorCache snapshots originalRepo at commitOld into
// ancestorDir. Used when `.awiki/template-cache/<commit_old>` is
// missing. Mirrors the bash auto-recover block.
func rebuildAncestorCache(g UpdateGit, ancestorDir, originalRepo, commitOld string, w io.Writer) error {
	tmp, err := os.MkdirTemp("", "awiki-anc-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	orig := filepath.Join(tmp, "orig")
	if isDir(filepath.Join(originalRepo, ".git")) || strings.HasPrefix(originalRepo, "http") {
		if err := g.FetchClone(originalRepo, orig); err != nil {
			return fmt.Errorf("cannot reach original_repo %s: %w", originalRepo, err)
		}
		// Best-effort checkout of commit_old.
		if err := g.Checkout(commitOld); err != nil {
			// Try `git fetch <commit_old>` then checkout again.
			_ = g.FetchUpdate(orig)
			_ = g.Checkout(commitOld)
		}
		if err := os.MkdirAll(ancestorDir, 0o755); err != nil {
			return err
		}
		if err := g.ArchiveTreeToDir(orig, ancestorDir); err != nil {
			return err
		}
		fmt.Fprintln(w, "Re-built ancestor cache from pin")
		return nil
	}
	// Local non-git: snapshot copy.
	if err := os.MkdirAll(ancestorDir, 0o755); err != nil {
		return err
	}
	if err := CopyDirContents(originalRepo, ancestorDir); err != nil {
		return err
	}
	fmt.Fprintln(w, "Re-built ancestor cache from pin")
	return nil
}

// shortSHA returns the first 12 chars of a SHA.
func shortSHA(sha string) string {
	if len(sha) <= 12 {
		return sha
	}
	return sha[:12]
}

// updatePinFields rewrites commit/version/ref/repo on .awiki/template.json.
// Used by Commit D and --re-pin.
func updatePinFields(pj, commit, version, ref, repo string, persistSource bool) error {
	if commit != "" {
		if err := SetProvenanceField(pj, "commit", commit); err != nil {
			return err
		}
	}
	if version != "" {
		if err := SetProvenanceField(pj, "version", version); err != nil {
			return err
		}
	}
	if ref != "" {
		if err := SetProvenanceField(pj, "ref", ref); err != nil {
			return err
		}
	}
	if persistSource && repo != "" {
		if err := SetProvenanceField(pj, "repo", repo); err != nil {
			return err
		}
	}
	return nil
}

// errAbort is returned (wrapped) when an orchestrator stage chooses to
// halt with an explicit user-facing message.
type errAbort struct{ msg string }

func (e errAbort) Error() string { return e.msg }

// IsHalt reports whether err is a deliberate halt (vs an unexpected
// internal error). Callers map this to exit code 1 with the message
// already on stderr.
func IsHalt(err error) bool {
	var h errAbort
	return errors.As(err, &h)
}

// Halt returns an errAbort wrapper.
func Halt(msg string) error { return errAbort{msg: msg} }
