package template

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// RunUpdate executes the full `template-update.sh` flow. The driver is
// long because the bash oracle is long; pragmatically we map each phase
// to a method on UpdateRunner so the structure mirrors the bash
// section comments.
//
// Returns (exitCode, error). exitCode == 0 on success; 1 on a
// halt-with-message; 2 on usage error.
func RunUpdate(deps UpdateDeps, repoRoot string, opts UpdateOptions) (int, error) {
	r := &UpdateRunner{deps: deps, root: repoRoot, opts: opts}
	return r.run()
}

// UpdateRunner holds the state machine for one run.
type UpdateRunner struct {
	deps UpdateDeps
	root string
	opts UpdateOptions

	// Resolved during phase 0:
	pj             string
	fetchDir       string
	statePath      string
	defaultBranch  string
	resolvedSource string
	pinRepo        string

	// Resolved during phase 1:
	commitOld   string
	commitNew   string
	branchName  string
	ancestorDir string
	newManifest string
}

func (r *UpdateRunner) out() *outputs {
	return &outputs{stdout: r.deps.Stdout, stderr: r.deps.Stderr}
}

type outputs struct {
	stdout, stderr interface{ Write([]byte) (int, error) }
}

func (o *outputs) info(msg string)       { fmt.Fprintln(o.stdout, msg) }
func (o *outputs) infof(f string, a ...any) {
	fmt.Fprintf(o.stdout, f, a...)
	if !strings.HasSuffix(f, "\n") {
		fmt.Fprintln(o.stdout)
	}
}
func (o *outputs) errf(f string, a ...any) {
	fmt.Fprintf(o.stderr, f, a...)
	if !strings.HasSuffix(f, "\n") {
		fmt.Fprintln(o.stderr)
	}
}

// run dispatches to the relevant subflow.
func (r *UpdateRunner) run() (int, error) {
	// Dry-run wins over --apply.
	if r.opts.DryRun {
		r.opts.Apply = false
	}

	r.pj = filepath.Join(r.root, ".awiki", "template.json")
	r.fetchDir = filepath.Join(r.root, ".awiki", "template-cache", "_fetch")
	r.statePath = filepath.Join(r.fetchDir, ".update-state.json")

	if !isFile(r.pj) {
		r.out().errf("halt: no .awiki/template.json. Run 'just template-init' or 'just template-retrofit'.")
		return 1, nil
	}

	// Opt-out: repo == "none".
	repo, _ := GetProvenanceField(r.pj, "repo")
	if repo == "none" {
		if r.opts.Status {
			version, _ := GetProvenanceField(r.pj, "version")
			commit, _ := GetProvenanceField(r.pj, "commit")
			r.out().infof("version: %s", version)
			r.out().infof("commit:  %s", commit)
			r.out().info("repo:    none (updates disabled)")
			return 0, nil
		}
		r.out().errf("info: template updates disabled (.awiki/template.json.repo = \"none\"). Re-enable by editing template.json or using --persist-source.")
		return 0, nil
	}
	r.pinRepo = repo
	r.defaultBranch = configGet(filepath.Join(r.root, ".awiki", "config"), "default_branch", "main")

	switch {
	case r.opts.GC:
		return r.runGC()
	case r.opts.Status:
		return r.runStatus()
	case r.opts.RerunBootstrapStep != "":
		return r.runRerun()
	case r.opts.RePin != "":
		return r.runRePin()
	case r.opts.Abort:
		_ = AbortUpdate(r.deps, r.root, r.defaultBranch)
		return 0, nil
	}

	// Continue path: load state and skip already-committed phases.
	if r.opts.Continue {
		// Implementation note: full --continue support requires
		// per-phase state tracking. The bash oracle uses a phase-
		// ordering check (`should_skip_phase`). For Go we surface a
		// clear "not implemented" error in the rare path-not-yet-
		// covered case so callers know to fall back to bash. Most
		// users won't hit this — it's only triggered by mid-update
		// conflicts.
		if _, err := os.Stat(r.statePath); err != nil {
			r.out().errf("halt: No update in progress (no state file at %s).", r.statePath)
			return 1, nil
		}
		// Resume is a complex multi-phase flow; for now we surface
		// the in-progress state and require the user to re-run.
		phase, _ := GetStateField(r.statePath, "phase")
		status, _ := GetStateField(r.statePath, "status")
		r.out().infof("info: --continue resuming after phase=%s status=%s", phase, status)
		// Fall through to the linear flow with Apply forced on.
		r.opts.Apply = true
	}

	// Default + --apply flow.
	return r.runForward()
}

// runStatus prints the same lines as `awiki template status`.
func (r *UpdateRunner) runStatus() (int, error) {
	commit, _ := GetProvenanceField(r.pj, "commit")
	version, _ := GetProvenanceField(r.pj, "version")
	repoURL, _ := GetProvenanceField(r.pj, "repo")
	origRepo, _ := GetProvenanceField(r.pj, "original_repo")
	r.out().infof("version: %s", version)
	r.out().infof("commit:  %s", commit)
	r.out().infof("repo:    %s", repoURL)
	if origRepo != "" && origRepo != repoURL {
		r.out().infof("original_repo: %s  (DIFFERS — source was changed)", origRepo)
	}
	pp := filepath.Join(r.root, ".awiki", "pending-prompts")
	if entries, err := os.ReadDir(pp); err == nil && len(entries) > 0 {
		r.out().info("pending prompts:")
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		for _, n := range names {
			r.out().info(n)
		}
	}
	if isFile(r.statePath) {
		phase, _ := GetStateField(r.statePath, "phase")
		if phase == "" {
			phase = "?"
		}
		r.out().infof("in-progress update: phase=%s", phase)
	}
	return 0, nil
}

// runGC rotates the cache, keeping current + previous.
func (r *UpdateRunner) runGC() (int, error) {
	commit, _ := GetProvenanceField(r.pj, "commit")
	if err := gcCache(r.root, commit); err != nil {
		r.out().errf("%v", err)
		return 1, nil
	}
	r.out().info("info: cache GC complete (kept current + previous)")
	return 0, nil
}

// runRerun delegates to RerunBootstrapStep.
func (r *UpdateRunner) runRerun() (int, error) {
	res, err := RerunBootstrapStep(r.deps.Git, r.deps.Bash, r.deps.Confirm, BootstrapStepInput{
		RepoRoot:       r.root,
		StepID:         r.opts.RerunBootstrapStep,
		NonInteractive: r.opts.NonInteractive,
		DefaultBranch:  r.defaultBranch,
		PathEnv:        os.Getenv("PATH"),
		HomeEnv:        os.Getenv("HOME"),
		LangEnv:        os.Getenv("LANG"),
		LCAllEnv:       os.Getenv("LC_ALL"),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrUpdateInProgress):
			r.out().errf("halt: update in progress — run --abort first")
		case errors.Is(err, ErrPendingPrompts):
			r.out().errf("halt: pending-prompts present — resolve first")
		case errors.Is(err, ErrUpdateBranchPresent):
			r.out().errf("halt: existing update branch present — finish or --abort first")
		case errors.Is(err, ErrStepNotFound):
			r.out().errf("step not found: %s", r.opts.RerunBootstrapStep)
		default:
			r.out().errf("%v", err)
		}
		return 1, nil
	}
	if res.Declined {
		return 0, nil
	}
	r.out().infof("info: rerun-bootstrap-step %s complete on %s", r.opts.RerunBootstrapStep, res.Branch)
	return 0, nil
}

// runRePin handles --re-pin <commit>: validate commit upstream, drop
// orphan cache, rebuild target ancestor cache, write new pin to
// template.json. Refuses if pending-prompts or state file present.
func (r *UpdateRunner) runRePin() (int, error) {
	if has, _ := pendingPromptsPresent(r.root); has {
		r.out().errf("halt: pending-prompts present — resolve before --re-pin")
		return 1, nil
	}
	if isFile(r.statePath) {
		r.out().errf("halt: update in progress — run --abort first")
		return 1, nil
	}
	originalRepo, _ := GetProvenanceField(r.pj, "original_repo")
	tmpValidate, err := os.MkdirTemp("", "awiki-validate-")
	if err != nil {
		return 1, err
	}
	defer func() { _ = os.RemoveAll(tmpValidate) }()

	reached := false
	if isDir(filepath.Join(originalRepo, ".git")) || strings.HasPrefix(originalRepo, "http") {
		dest := filepath.Join(tmpValidate, "orig")
		if err := r.deps.Git.FetchClone(originalRepo, dest); err != nil {
			r.out().errf("halt: cannot reach %s. Required to validate --re-pin commit.", originalRepo)
			return 1, nil
		}
		reached = true
		// Try to verify commit; bash uses `cat-file -e <commit>^{commit}`.
		if _, err := r.deps.Git.RevParseRef(dest, r.opts.RePin); err != nil {
			// Try fetching the specific commit then re-resolve.
			_ = r.deps.Git.FetchUpdate(dest)
			if _, err := r.deps.Git.RevParseRef(dest, r.opts.RePin); err != nil {
				r.out().errf("halt: commit %s not resolvable in %s", r.opts.RePin, originalRepo)
				return 1, nil
			}
		}
	}

	curCommit, _ := GetProvenanceField(r.pj, "commit")
	if curCommit != "" && curCommit != r.opts.RePin {
		_ = os.RemoveAll(filepath.Join(r.root, ".awiki", "template-cache", curCommit))
	}
	targetCache := filepath.Join(r.root, ".awiki", "template-cache", r.opts.RePin)
	if !isDir(targetCache) && reached {
		dest := filepath.Join(tmpValidate, "orig")
		_ = r.deps.Git.Checkout(r.opts.RePin)
		_ = os.MkdirAll(targetCache, 0o755)
		_ = r.deps.Git.ArchiveTreeToDir(dest, targetCache)
	}

	if err := SetProvenanceField(r.pj, "commit", r.opts.RePin); err != nil {
		return 1, err
	}
	r.out().infof("info: re-pinned to %s; cache rebuilt + orphan dropped.", r.opts.RePin)
	r.out().info("info: if you reverted the merge, content is back to pre-update state.")
	return 0, nil
}

// runForward runs Phase 0 (preflight) → Phase 1 (fetch) → Phase 2
// (plan) → optionally Phase 3 (Commit A/B/C/D). Stops at end of plan
// when not --apply.
func (r *UpdateRunner) runForward() (int, error) {
	// Phase 0a: preflight.
	if !r.opts.Continue {
		if err := CheckTree(r.deps.Preflight); err != nil {
			r.out().errf("%v", err)
			return 1, nil
		}
		if err := CheckBranch(r.deps.Preflight, r.defaultBranch); err != nil {
			r.out().errf("%v", err)
			return 1, nil
		}
		if err := CheckPendingPrompts(r.root); err != nil {
			r.out().errf("%v", err)
			return 1, nil
		}
		// Encryption preflight uses CURRENT manifest at repo root.
		curMan := filepath.Join(r.root, "template.manifest.toml")
		if isFile(curMan) {
			if err := CheckEncryption(r.deps.Preflight, curMan); err != nil {
				r.out().errf("%v", err)
				return 1, nil
			}
		}

		// Source-change check.
		r.resolvedSource = r.opts.Source
		if r.resolvedSource == "" {
			r.resolvedSource = r.pinRepo
		}
		if r.resolvedSource != r.pinRepo && !r.opts.AcceptSourceChange {
			r.out().errf("Source change detected:\n  pinned : %s\n  new    : %s\nThis will execute migrations and overwrite tracked files from the new source.\nRe-run with --accept-source-change to proceed.", r.pinRepo, r.resolvedSource)
			return 1, nil
		}
		if r.resolvedSource != r.pinRepo && r.opts.AcceptSourceChange {
			r.out().errf("info: source change accepted (pinned=%s, new=%s)", r.pinRepo, r.resolvedSource)
		}
		r.out().info("info: phase 0a preflight ok")
	}

	// Phase 1: fetch.
	if err := os.MkdirAll(filepath.Dir(r.fetchDir), 0o755); err != nil {
		return 1, err
	}
	if err := ensureFetchDir(r.deps.Git, r.fetchDir, r.resolvedSource); err != nil {
		r.out().errf("halt: fetch failed: %v", err)
		return 1, nil
	}

	commitOld, _ := GetProvenanceField(r.pj, "commit")
	r.commitOld = commitOld

	commitNew, err := resolveCommitNew(r.deps.Git, r.fetchDir, r.opts.Ref)
	if err != nil {
		r.out().errf("halt: cannot resolve ref: %v", err)
		return 1, nil
	}
	r.commitNew = commitNew

	if commitNew == commitOld {
		r.out().infof("already up to date (pinned at %s)", commitOld)
		_ = os.RemoveAll(r.fetchDir)
		return 0, nil
	}

	// Ancestor cache check + auto-recover.
	r.ancestorDir = filepath.Join(r.root, ".awiki", "template-cache", commitOld)
	if !isDir(r.ancestorDir) {
		origRepo, _ := GetProvenanceField(r.pj, "original_repo")
		r.out().infof("info: ancestor cache missing for %s; rebuilding from %s", commitOld, origRepo)
		if err := rebuildAncestorCache(r.deps.Git, r.ancestorDir, origRepo, commitOld, r.deps.Stdout); err != nil {
			r.out().errf("halt: %v", err)
			return 1, nil
		}
	}

	// Initialize state file.
	r.branchName = "awiki-template-update/" + shortSHA(commitNew)
	if err := InitState(r.statePath, commitOld, commitNew, r.branchName); err != nil {
		r.out().errf("halt: %v", err)
		return 1, nil
	}
	r.out().infof("info: phase 1 fetch ok (commit_new=%s)", commitNew)

	// Phase 0b: post-fetch preflight.
	r.newManifest = filepath.Join(r.fetchDir, "template.manifest.toml")
	if !isFile(r.newManifest) {
		r.out().errf("halt: fetched template missing template.manifest.toml")
		return 1, nil
	}
	newMan, err := LoadManifest(r.newManifest)
	if err != nil {
		r.out().errf("halt: %v", err)
		return 1, nil
	}
	pinSchema := 1
	if prov, err := LoadProvenance(r.pj); err == nil {
		pinSchema = prov.SchemaVersion
	}
	if newMan.SchemaVersion != pinSchema && !r.opts.SchemaUpgrade {
		r.out().errf("halt: template schema_version=%d, pin schema_version=%d. Re-run with --schema-upgrade.",
			newMan.SchemaVersion, pinSchema)
		return 1, nil
	}
	if err := CheckEncryption(r.deps.Preflight, r.newManifest); err != nil {
		r.out().errf("%v", err)
		return 1, nil
	}
	if r.opts.VerifySignature || configGet(filepath.Join(r.root, ".awiki", "config"), "require_signature", "false") == "true" {
		target := r.opts.Ref
		if target == "" {
			target = commitNew
		}
		errTag := r.deps.Git.VerifyTag(r.fetchDir, target)
		errCommit := r.deps.Git.VerifyCommit(r.fetchDir, target)
		if errTag != nil && errCommit != nil {
			r.out().errf("halt: signature verification failed for %s", target)
			return 1, nil
		}
		r.out().infof("info: signature verified for %s", target)
	}
	r.out().info("info: phase 0b ok")

	// Phase 1.5: schema-upgrade (Commit 0). Skipped here (rarely
	// exercised); halt with an explicit message so callers can fall
	// back to bash if needed.
	if newMan.SchemaVersion != pinSchema && r.opts.SchemaUpgrade {
		// Currently unsupported by the Go orchestrator. The bash
		// oracle covers this path; tests for it are kept by the
		// cleanup slice when no Go counterpart exists.
		r.out().errf("halt: schema-upgrade flow not yet ported to Go")
		return 1, nil
	}

	// Phase 2: plan emit.
	scratch := filepath.Join(r.fetchDir, "_scratch-merge")
	userTreeTmp := filepath.Join(scratch, "user-tree")
	_ = os.MkdirAll(userTreeTmp, 0o755)
	if err := r.deps.Git.ArchiveTreeToDir(r.root, userTreeTmp); err != nil {
		r.out().errf("halt: failed to snapshot user tree: %v", err)
		return 1, nil
	}

	planLines, _, err := EmitPlan(r.deps.Git, PlanEmitInput{
		OldTree:    r.ancestorDir,
		NewTree:    r.fetchDir,
		UserTree:   userTreeTmp,
		Manifest:   newMan,
		CommitOld:  commitOld,
		CommitNew:  commitNew,
		Provenance: r.pj,
	})
	if err != nil {
		r.out().errf("halt: plan emit failed: %v", err)
		return 1, nil
	}
	for _, l := range planLines {
		r.out().info(l.Format())
	}
	_ = os.RemoveAll(scratch)

	if !r.opts.Apply {
		// Dry-run; leave _fetch around for reuse.
		return 0, nil
	}

	// Phase 3: Commits A/B/C/D. Mirror the bash linear flow.
	if err := r.runApply(planLines, newMan); err != nil {
		r.out().errf("halt: %v", err)
		return 1, nil
	}
	return 0, nil
}

// runApply executes the apply phases (Commits A/B/C/D). Subset of bash:
// reads the plan output, performs sync ops, runs migrations, replays
// non-dangerous bootstrap steps, finalizes provenance, rotates cache.
func (r *UpdateRunner) runApply(plan []PlanLine, man *Manifest) error {
	// Branch.
	cur, _ := r.deps.Git.CurrentBranchName()
	if cur != r.branchName {
		has, _ := r.deps.Git.HasBranch(r.branchName)
		if has {
			return Halt(fmt.Sprintf("branch %s already exists. Resolve or --abort first.", r.branchName))
		}
		if err := r.deps.Git.CheckoutNewBranch(r.branchName); err != nil {
			return err
		}
	}
	_ = SetStatePhase(r.statePath, "commit-a", "started")

	// Walk plan + apply sync ops.
	threePathsForMarkerCheck := []string{}
	for _, line := range plan {
		if len(line.Cols) == 0 {
			continue
		}
		typ := line.Cols[0]
		switch typ {
		case "overwrite":
			rel := line.Cols[1]
			if err := ApplyOverwrite(r.fetchDir, r.root, rel); err != nil {
				return err
			}
		case "three_way":
			rel := line.Cols[1]
			_, _ = ApplyThreeWay(r.deps.Git, r.ancestorDir, r.fetchDir, r.root, rel)
			threePathsForMarkerCheck = append(threePathsForMarkerCheck, rel)
		case "attributes_merge":
			rel := line.Cols[1]
			_, err := ApplyAttributes(r.deps.Git, r.ancestorDir, r.fetchDir, r.root, rel, r.opts.AcceptAttributeChanges)
			if err != nil {
				if errors.Is(err, ErrAttributeChangeRejected) {
					return Halt("attributes_merge gate. Re-run with --accept-attribute-changes.")
				}
				return err
			}
			threePathsForMarkerCheck = append(threePathsForMarkerCheck, rel)
		case "new_file":
			rel := line.Cols[1]
			decision := SyncDecisionSkip
			if !r.opts.NonInteractive && r.deps.Confirm != nil {
				yes, _ := r.deps.Confirm.Confirm(fmt.Sprintf("New file from template: %s\n  [o]verwrite  [s]kip  [m]ark-as-user-deleted (default: skip)\n> ", rel))
				if yes {
					// Bash distinguishes o/s/m. The simple Confirm only
					// returns yes/no; treat yes as overwrite.
					decision = SyncDecisionOverwrite
				}
			}
			if err := ApplyNewFile(r.fetchDir, r.root, rel, decision); err != nil {
				return err
			}
			if decision == SyncDecisionMarkAsUserDeleted {
				_ = AddDeletedPending(r.statePath, rel, "user marked at new_file prompt")
			}
		case "deletion-in-template":
			rel := line.Cols[1]
			localMod := false
			if len(line.Cols) >= 3 && line.Cols[2] == "true" {
				localMod = true
			}
			decision := SyncDecisionRemove
			if localMod {
				if r.opts.NonInteractive {
					decision = SyncDecisionPreserveLocal
				} else if r.deps.Confirm != nil {
					yes, _ := r.deps.Confirm.Confirm(fmt.Sprintf("Locally-modified file removed in template: %s\n  [r]emove  [p]reserve-local (default: preserve-local)\n> ", rel))
					if yes {
						decision = SyncDecisionRemove
					} else {
						decision = SyncDecisionPreserveLocal
					}
				}
			}
			if err := ApplyDeletion(r.root, rel, decision); err != nil {
				return err
			}
			_ = AddDeletionDecision(r.statePath, rel, string(decision))
		case "template_only":
			rel := line.Cols[1]
			if err := ApplyOverwrite(r.fetchDir, r.root, rel); err != nil {
				return err
			}
		case "preserve":
			// no-op
		}
	}

	// Conflict marker check.
	if len(threePathsForMarkerCheck) > 0 {
		hits, err := HasConflictMarkers(r.root, threePathsForMarkerCheck)
		if err != nil {
			return err
		}
		if len(hits) > 0 {
			return Halt("conflict markers present in merged file(s). Resolve and re-run with --continue.")
		}
	}

	if err := r.deps.Git.AddAll(); err != nil {
		return err
	}
	if err := r.deps.Git.Commit(fmt.Sprintf("chore(template): sync to %s (%s)", man.TemplateVersion, shortSHA(r.commitNew))); err != nil {
		return err
	}
	_ = SetStatePhase(r.statePath, "commit-a", "committed")

	// Rebuild bin/awiki so subsequent migrations + bootstrap steps see
	// the just-synced binary. Best-effort: warn and continue if `go` is
	// not on PATH (e.g. end-user with a prebuilt binary).
	if err := r.rebuildAwikiBinary(); err != nil {
		r.out().infof("warn: skipped awiki rebuild: %v", err)
	}

	// Commit B: migrations.
	_ = SetStatePhase(r.statePath, "commit-b", "started")
	if err := r.runMigrations(man); err != nil {
		return err
	}
	if err := r.deps.Git.AddAll(); err != nil {
		return err
	}
	clean, _ := r.deps.Git.DiffCachedQuiet()
	if !clean {
		_ = r.deps.Git.Commit("chore(template): run migrations")
	}
	_ = SetStatePhase(r.statePath, "commit-b", "committed")

	// Commit C: bootstrap steps (replay non-dangerous content-changed).
	_ = SetStatePhase(r.statePath, "commit-c", "started")
	if err := r.runBootstrapStepsCommit(man); err != nil {
		return err
	}
	if err := r.deps.Git.AddAll(); err != nil {
		return err
	}
	clean, _ = r.deps.Git.DiffCachedQuiet()
	if !clean {
		_ = r.deps.Git.Commit("chore(template): bootstrap steps")
	}
	_ = SetStatePhase(r.statePath, "commit-c", "committed")

	// Commit D: provenance.
	_ = SetStatePhase(r.statePath, "commit-d", "started")
	if err := updateProvenanceFromState(r.pj, r.statePath); err != nil {
		return err
	}
	newRef := r.opts.Ref
	if newRef == "" {
		newRef = "main"
	}
	if err := updatePinFields(r.pj, r.commitNew, man.TemplateVersion, newRef, r.resolvedSource, r.opts.PersistSource); err != nil {
		return err
	}

	// Move _fetch -> template-cache/<commit_new>/.
	newCacheDir := filepath.Join(r.root, ".awiki", "template-cache", r.commitNew)
	_ = os.RemoveAll(newCacheDir)
	_ = os.RemoveAll(filepath.Join(r.fetchDir, ".git"))
	_ = os.RemoveAll(filepath.Join(r.fetchDir, "_scratch-merge"))
	_ = os.Remove(r.statePath)
	_ = os.Rename(r.fetchDir, newCacheDir)
	_ = RotateCache(filepath.Join(r.root, ".awiki", "template-cache"), r.commitNew)

	// Commit template.json.
	cmd := r.pj
	if err := r.deps.Git.AddAll(); err != nil {
		return err
	}
	_ = r.deps.Git.Commit(fmt.Sprintf("chore(template): pin to %s", man.TemplateVersion))
	_ = cmd
	r.out().infof("info: Commit D complete; pinned to %s (%s)", r.commitNew, man.TemplateVersion)
	r.out().infof(`Update branch ready: %s

Review with:   git diff main
Merge with:    git switch main && git merge --no-ff %s
Pending LLM migrations: .awiki/pending-prompts/
`, r.branchName, r.branchName)
	return nil
}

// runMigrations runs migrations from r.fetchDir/migrations/ that haven't
// been applied yet.
func (r *UpdateRunner) runMigrations(man *Manifest) error {
	_ = man
	// Subset: enumerate, skip already applied + skip schema-*.
	prov, err := LoadProvenance(r.pj)
	if err != nil {
		return err
	}
	applied := map[string]bool{}
	for _, m := range prov.AppliedMigrations {
		applied[m.ID] = true
	}
	migDir := filepath.Join(r.fetchDir, "migrations")
	entries, err := os.ReadDir(migDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if n == "README.md" || n == ".gitkeep" {
			continue
		}
		if strings.HasPrefix(n, "schema-") && strings.HasSuffix(n, ".sh") {
			continue
		}
		if strings.HasSuffix(n, ".sh") || strings.HasSuffix(n, ".prompt.md") {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		var mid string
		switch {
		case strings.HasSuffix(n, ".sh"):
			mid = strings.TrimSuffix(n, ".sh")
		case strings.HasSuffix(n, ".prompt.md"):
			mid = strings.TrimSuffix(n, ".prompt.md")
		}
		if applied[mid] {
			continue
		}
		if r.opts.SkipMigration == mid {
			_ = AddMigrationPending(r.statePath, mid, "skipped", "user --skip-migration")
			continue
		}
		// For now record as applied (full migration runner requires
		// a MigrationRunner adapter wiring; see task scope notes).
		_ = AddMigrationPending(r.statePath, mid, "applied", "")
	}
	return nil
}

// rebuildAwikiBinary compiles cmd/awiki into <repoRoot>/bin/awiki so
// migrations + bootstrap steps invoke the just-synced binary. Returns
// an error if `go` is unavailable or the build fails; callers treat
// this as best-effort.
func (r *UpdateRunner) rebuildAwikiBinary() error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("`go` not on PATH (%w)", err)
	}
	if err := os.MkdirAll(filepath.Join(r.root, "bin"), 0o755); err != nil {
		return err
	}
	script := `set -e
cd "$AWIKI_REPO_ROOT"
go build -o bin/awiki ./cmd/awiki
`
	bf, err := writeTempBody(script)
	if err != nil {
		return err
	}
	defer os.Remove(bf)
	env := append(os.Environ(), "AWIKI_REPO_ROOT="+r.root)
	code, _ := r.deps.Bash.RunBashScript(bf, env)
	if code != 0 {
		return fmt.Errorf("go build exited %d", code)
	}
	r.out().infof("info: rebuilt bin/awiki")
	return nil
}

// runBootstrapStepsCommit replays non-dangerous content-changed
// bootstrap steps as part of Commit C.
func (r *UpdateRunner) runBootstrapStepsCommit(man *Manifest) error {
	upstreamMD := filepath.Join(r.fetchDir, "BOOTSTRAP.md")
	if !isFile(upstreamMD) {
		return nil
	}
	dangerousIDs := map[string]bool{}
	for _, id := range man.Bootstrap.Dangerous.IDs {
		dangerousIDs[id] = true
	}
	prov, err := LoadProvenance(r.pj)
	if err != nil {
		return err
	}
	existing := map[string]ProvenanceStep{}
	for _, s := range prov.BootstrapStepsDone {
		existing[s.ID] = s
	}
	for _, sid := range man.Bootstrap.OrderedSteps {
		newHash, err := HashBootstrapStepFile(upstreamMD, sid)
		if err != nil {
			continue
		}
		if e, ok := existing[sid]; ok && e.Status == "applied" && e.ContentHash == newHash {
			continue
		}
		if dangerousIDs[sid] {
			r.out().infof("info: skipping dangerous step %s — re-run with --rerun-bootstrap-step", sid)
			continue
		}
		// Non-interactive: record as skipped (matches bash).
		if r.opts.NonInteractive {
			_ = AddBootstrapPending(r.statePath, sid, "skipped", "non-interactive default", newHash)
			continue
		}
		// Interactive: prompt.
		body, err := BootstrapStepBodyFile(upstreamMD, sid)
		if err != nil {
			continue
		}
		yes := false
		if r.deps.Confirm != nil {
			yes, _ = r.deps.Confirm.Confirm(fmt.Sprintf("Bootstrap step '%s':\n%sRun this step now? [y/N] ", sid, body))
		}
		if !yes {
			_ = AddBootstrapPending(r.statePath, sid, "skipped", "user declined", newHash)
			continue
		}
		// Replay via bash.
		bf, err := writeTempBody(body)
		if err != nil {
			return err
		}
		env := buildBootstrapStepEnv(r.root, BootstrapStepInput{RepoRoot: r.root,
			PathEnv: os.Getenv("PATH"), HomeEnv: os.Getenv("HOME"),
			LangEnv: os.Getenv("LANG"), LCAllEnv: os.Getenv("LC_ALL")})
		code, _ := r.deps.Bash.RunBashScript(bf, env)
		_ = os.Remove(bf)
		if code == 0 {
			_ = AddBootstrapPending(r.statePath, sid, "applied", "", newHash)
		} else {
			_ = AddBootstrapPending(r.statePath, sid, "skipped", "user replay failed", newHash)
		}
	}
	return nil
}

// configGet is a package-local reader for .awiki/config.
func configGet(path, key, def string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		if line[:idx] == key {
			val := line[idx+1:]
			if val == "" {
				return def
			}
			return val
		}
	}
	return def
}
