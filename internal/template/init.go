package template

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// InitArchiver captures the single git operation `template-init.sh`
// performs: snapshot the working tree to a cache directory. The bash
// oracle uses `git archive --format=tar HEAD | tar -x -C <cacheDir>`.
// Production wires `os/exec`; tests inject fakes that copy a fixture
// directory.
type InitArchiver interface {
	// ArchiveTreeToDir snapshots HEAD of repoRoot to destDir, mirroring
	// `git archive --format=tar HEAD | tar -x -C <destDir>`.
	ArchiveTreeToDir(repoRoot, destDir string) error
}

// InitInput bundles the args `template-init.sh` accepts.
type InitInput struct {
	RepoRoot string
	Repo     string
	Ref      string
	Version  string
	Commit   string
}

// InitTemplate mirrors scripts/template-init.sh end-to-end:
//
//  1. Verify template.manifest.toml + BOOTSTRAP.md exist at repo root.
//  2. Init or update .awiki/template.json with repo/ref/version/commit.
//  3. Snapshot HEAD into .awiki/template-cache/<commit>/.
//  4. Record bootstrap_steps_done with content_hash for any IDs not
//     already recorded.
//
// Returns the post-write Provenance for callers that want to inspect
// the seeded record.
func InitTemplate(arch InitArchiver, in InitInput) (*Provenance, error) {
	if in.RepoRoot == "" || in.Repo == "" || in.Ref == "" || in.Version == "" || in.Commit == "" {
		return nil, errors.New("template-init: --repo/--ref/--version/--commit all required")
	}
	if arch == nil {
		return nil, errors.New("template-init: archiver is nil")
	}
	manifestPath := filepath.Join(in.RepoRoot, "template.manifest.toml")
	bootstrapPath := filepath.Join(in.RepoRoot, "BOOTSTRAP.md")
	if !isFile(manifestPath) {
		return nil, fmt.Errorf("template.manifest.toml not found at repo root")
	}
	if !isFile(bootstrapPath) {
		return nil, fmt.Errorf("BOOTSTRAP.md not found at repo root")
	}

	pj := filepath.Join(in.RepoRoot, ".awiki", "template.json")
	exists := isFile(pj)
	if !exists {
		if err := InitProvenance(pj, in.Repo, in.Ref, in.Version, in.Commit); err != nil {
			return nil, err
		}
	} else {
		// Bash prints info to stderr; callers can mirror that line.
	}

	// Always update commit/ref/version to current values.
	if err := SetProvenanceField(pj, "commit", in.Commit); err != nil {
		return nil, err
	}
	if err := SetProvenanceField(pj, "ref", in.Ref); err != nil {
		return nil, err
	}
	if err := SetProvenanceField(pj, "version", in.Version); err != nil {
		return nil, err
	}

	// Snapshot tree to cache.
	cacheDir := filepath.Join(in.RepoRoot, ".awiki", "template-cache", in.Commit)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	if err := arch.ArchiveTreeToDir(in.RepoRoot, cacheDir); err != nil {
		return nil, err
	}

	// Record bootstrap_steps_done with content_hash for any IDs not
	// already recorded. Bash reads the existing IDs from template.json
	// then iterates the BOOTSTRAP.md step list.
	existingIDs := map[string]bool{}
	prov, err := LoadProvenance(pj)
	if err != nil {
		return nil, err
	}
	for _, s := range prov.BootstrapStepsDone {
		existingIDs[s.ID] = true
	}
	stepIDs, err := ListBootstrapStepIDsFile(bootstrapPath)
	if err != nil {
		return nil, err
	}
	for _, id := range stepIDs {
		if existingIDs[id] {
			continue
		}
		hash, err := HashBootstrapStepFile(bootstrapPath, id)
		if err != nil {
			return nil, err
		}
		if err := AppendProvenanceBootstrapStep(pj, id, "applied", "", hash); err != nil {
			return nil, err
		}
	}

	return LoadProvenance(pj)
}

// InitTemplatePreserveMessage returns the bash-oracle-equivalent stderr
// line callers should emit when template.json already exists. Kept here
// so the CLI driver doesn't have to hardcode the wording.
func InitTemplatePreserveMessage(pjPath string) string {
	return fmt.Sprintf("info: %s already exists; preserving applied_migrations + bootstrap_steps_done", pjPath)
}

// CopyDirContents recursively copies every regular file from srcRoot
// into dstRoot, preserving relative paths and mode bits. Used by the
// fake archiver in tests and by the local-source path in
// template-update Phase 1.
func CopyDirContents(srcRoot, dstRoot string) error {
	return filepath.Walk(srcRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		dst := filepath.Join(dstRoot, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, info.Mode().Perm())
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = in.Close() }()
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = out.Close()
			return err
		}
		return out.Close()
	})
}
