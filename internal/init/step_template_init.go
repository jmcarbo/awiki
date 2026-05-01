package initverb

import (
	"fmt"
	"os"
	"path/filepath"

	"awiki/internal/template"
)

// stepTemplateInit runs Step 12: seed .awiki/template.json from the
// repo's template.manifest.toml + current HEAD SHA, then iterate the
// init verb's own state and stamp bootstrap_steps_done[] entries with
// content_hash for each applied step.
//
// Skipped silently when template.manifest.toml is absent.
type stepTemplateInit struct{}

func (stepTemplateInit) ID() string          { return "template-init" }
func (stepTemplateInit) Description() string { return "seed template provenance" }
func (stepTemplateInit) Kind() Kind          { return KindMechanical }

// templateArchiver implements template.InitArchiver. It uses the same
// bash-equivalent `git archive | tar -x` pipe the production template
// orchestrator uses (TemplateOrchGit.ArchiveTreeToDir). Wiring it here
// directly lets us avoid threading another adapter through StepContext
// for a single call site.
type templateArchiver struct {
	git GitInit
	ctx StepContext
}

func (a templateArchiver) ArchiveTreeToDir(repoRoot, dst string) error {
	// Snapshot via `cp -r` of the repo root excluding .git/.awiki — we
	// can't drive `git archive | tar` from the GitInit adapter without
	// extending it. The production code path through TemplateOrchGit
	// already supports archive; the init verb composes a thinner
	// surface, so we use a simple copy that matches the shape the
	// resulting cache dir needs.
	return copyTreeForCache(repoRoot, dst)
}

// copyTreeForCache mirrors what `git archive HEAD | tar -x` would
// produce: tracked-tree contents minus the .git directory. We use a
// simpler "everything except .git" copy because the cache is consumed
// by template-update which only reads back tracked files anyway.
func copyTreeForCache(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		// Skip .git, .awiki, and worktree state dirs so we don't
		// mirror megabytes of cache into the snapshot.
		first := rel
		if i := indexByte(rel, filepath.Separator); i >= 0 {
			first = rel[:i]
		}
		switch first {
		case ".git", ".awiki", "node_modules", "public", "resources",
			".hugo_build.lock", ".obsidian", "bin", ".worktrees",
			".cache", ".qmd":
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		return copyOne(path, filepath.Join(dst, rel), info)
	})
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func copyOne(srcPath, dstPath string, info os.FileInfo) error {
	if info.IsDir() {
		return os.MkdirAll(dstPath, info.Mode().Perm())
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	in, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	buf := make([]byte, 32*1024)
	for {
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if rerr != nil {
			if rerr.Error() == "EOF" {
				return nil
			}
			break
		}
	}
	return nil
}

func (stepTemplateInit) Execute(ctx StepContext, _ *Answers) (Result, error) {
	manifestPath := filepath.Join(ctx.RepoRoot, "template.manifest.toml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return Result{Status: StatusSkipped, Note: "no template.manifest.toml at repo root"}, nil
	}
	m, err := template.LoadManifest(manifestPath)
	if err != nil {
		return Result{Status: StatusFailed}, fmt.Errorf("load manifest: %w", err)
	}
	// Pin "self" — the wiki repo is its own template source until the
	// user retargets via `awiki template update --source <upstream>`.
	repoURL := "self"
	ref := "main"
	commit := "0000000000000000000000000000000000000000"
	if ctx.Git != nil {
		if sha, err := ctx.Git.RevParseHEAD(ctx.Ctx, ctx.RepoRoot); err == nil && sha != "" {
			commit = sha
		}
	}
	in := template.InitInput{
		RepoRoot: ctx.RepoRoot,
		Repo:     repoURL,
		Ref:      ref,
		Version:  m.TemplateVersion,
		Commit:   commit,
	}
	if _, err := template.InitTemplate(templateArchiver{git: ctx.Git, ctx: ctx}, in); err != nil {
		return Result{Status: StatusFailed}, fmt.Errorf("template-init: %w", err)
	}

	// Stamp every applied init-state step with its content_hash. The
	// underlying InitTemplate already iterates BOOTSTRAP.md and
	// records each step with status="applied"; we re-use the file in
	// place and let the next template-update consume it. Re-running
	// template-init is idempotent against already-recorded IDs.
	state, err := LoadState(ctx.RepoRoot)
	if err == nil && state != nil {
		pj := filepath.Join(ctx.RepoRoot, ".awiki", "template.json")
		bp := filepath.Join(ctx.RepoRoot, "BOOTSTRAP.md")
		recorded, _ := loadRecordedStepIDs(pj)
		for _, s := range state.Steps {
			if s.Status != StatusApplied {
				continue
			}
			if recorded[s.ID] {
				continue
			}
			hash, err := template.HashBootstrapStepFile(bp, s.ID)
			if err != nil {
				// The step ID may not exist in BOOTSTRAP.md (init verb
				// IDs are a superset only when the manifest drifts).
				continue
			}
			_ = template.AppendProvenanceBootstrapStep(pj, s.ID, "applied", "", hash)
			recorded[s.ID] = true
		}
	}
	return Result{Status: StatusApplied,
		Note: fmt.Sprintf("template.json pinned at version=%s commit=%s", m.TemplateVersion, short(commit))}, nil
}

func loadRecordedStepIDs(pj string) (map[string]bool, error) {
	out := map[string]bool{}
	prov, err := template.LoadProvenance(pj)
	if err != nil {
		return out, err
	}
	for _, s := range prov.BootstrapStepsDone {
		out[s.ID] = true
	}
	return out, nil
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
