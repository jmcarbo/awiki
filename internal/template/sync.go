package template

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SyncGit captures the git operations Phase 3 Commit A needs. The bash +
// Python oracle drives `git merge-file --diff3`, `git ls-files`, `git
// init`, and `git check-attr`. Each method returns (code, err) for
// commands run for side-effects, or (out, code, err) for stdout-bearing
// commands. A non-nil err signals the binary is missing entirely.
type SyncGit interface {
	// MergeFile shells `git merge-file --diff3 -L current -L base -L
	// new <cur> <base> <new>`. The cur path is mutated in place. Exit
	// 0 = clean merge; >0 = number of conflicts.
	MergeFile(cur, base, newFile string) (code int, err error)
	// LsFilesIn shells `git -C <dir> ls-files`. Returns the raw newline-
	// separated stdout.
	LsFilesIn(dir string) (out string, code int, err error)
	// CheckAttrAll runs `git check-attr -a -- <path>` against an
	// isolated repo configured with core.attributesfile=<attrFile> and
	// GIT_ATTR_NOSYSTEM=1. The Python oracle creates a fresh tempdir
	// for this; production adapters should mirror that. attrFile may
	// be the literal "/dev/null" sentinel to indicate "no .gitattributes".
	CheckAttrAll(attrFile, path string) (out string, err error)
}

// SyncDecision is the user-supplied decision for new_file +
// deletion-in-template plan rows.
type SyncDecision string

const (
	SyncDecisionOverwrite          SyncDecision = "overwrite"
	SyncDecisionSkip               SyncDecision = "skip"
	SyncDecisionMarkAsUserDeleted  SyncDecision = "mark-as-user-deleted"
	SyncDecisionRemove             SyncDecision = "remove"
	SyncDecisionPreserveLocal      SyncDecision = "preserve-local"
)

// ApplyOverwrite copies newTree/rel to userTree/rel, preserving the
// source file mode. Mirrors `sync.py apply-overwrite`.
func ApplyOverwrite(newTree, userTree, rel string) error {
	src := filepath.Join(newTree, rel)
	dst := filepath.Join(userTree, rel)
	si, err := os.Stat(src)
	if err != nil || si.IsDir() {
		return fmt.Errorf("source not found: %s", src)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Chmod(dst, si.Mode().Perm())
}

// ApplyThreeWay performs the three-way merge for rel. Returns the
// merge-file exit code (0 = clean, >0 = number of conflicts) so callers
// can record / surface the marker check failure mode. Mirrors
// `sync.py apply-three-way`.
func ApplyThreeWay(g SyncGit, oldTree, newTree, userTree, rel string) (int, error) {
	base := filepath.Join(oldTree, rel)
	newFile := filepath.Join(newTree, rel)
	cur := filepath.Join(userTree, rel)
	if !isFile(newFile) {
		return 0, nil // not in new tree — nothing to do
	}
	if !isFile(cur) {
		// User deleted the file; treat as new-file-with-base.
		if err := os.MkdirAll(filepath.Dir(cur), 0o755); err != nil {
			return 0, err
		}
		return 0, copyFile(newFile, cur)
	}
	if !isFile(base) {
		// No ancestor — fall back to overwrite-with-warning. Caller
		// surfaces the warning to stderr.
		return 0, copyFile(newFile, cur)
	}
	if g == nil {
		return 0, errors.New("sync: git adapter is nil")
	}
	return g.MergeFile(cur, base, newFile)
}

// ApplyAttributesResult carries the cross-cutting state Phase-3-Commit-A
// inspects after `apply-attributes` completes.
type ApplyAttributesResult struct {
	// MergeCode is the underlying merge-file exit code, or 0 if no
	// merge ran (e.g. user deleted the file).
	MergeCode int
	// Flipped is true iff the OLD vs NEW .gitattributes encryption
	// filter changed for at least one tracked path.
	Flipped bool
	// FlippedPaths is the list of paths whose encryption filter
	// flipped (used to emit PLAN|attribute-change|... lines).
	FlippedPaths []string
}

// ApplyAttributes mirrors `sync.py apply-attributes`. It runs the
// three-way merge, then audits the OLD vs merged .gitattributes for
// flipped git-crypt filter assignments.
//
// When acceptAttrChanges is false and any flip is detected, the caller
// should treat that as the halt condition (matches Python `return 1`).
func ApplyAttributes(g SyncGit, oldTree, newTree, userTree, rel string, acceptAttrChanges bool) (*ApplyAttributesResult, error) {
	res := &ApplyAttributesResult{}
	base := filepath.Join(oldTree, rel)
	newFile := filepath.Join(newTree, rel)
	cur := filepath.Join(userTree, rel)
	if !isFile(newFile) {
		return res, nil
	}
	if !isFile(cur) {
		if err := os.MkdirAll(filepath.Dir(cur), 0o755); err != nil {
			return res, err
		}
		if err := copyFile(newFile, cur); err != nil {
			return res, err
		}
	} else if isFile(base) {
		if g == nil {
			return res, errors.New("sync: git adapter is nil")
		}
		code, err := g.MergeFile(cur, base, newFile)
		if err != nil {
			return res, err
		}
		res.MergeCode = code
		if code != 0 {
			// Conflict; let orchestrator's marker check halt.
			return res, nil
		}
	} else {
		if err := copyFile(newFile, cur); err != nil {
			return res, err
		}
	}

	if g == nil {
		return res, errors.New("sync: git adapter is nil")
	}

	// Tracked files audit.
	out, _, err := g.LsFilesIn(userTree)
	if err != nil {
		return res, err
	}
	tracked := splitLines(out)

	baseAttrs := base
	if !isFile(base) {
		baseAttrs = os.DevNull
	}
	newAttrs := cur

	for _, p := range tracked {
		if p == "" {
			continue
		}
		oldOut, err := g.CheckAttrAll(baseAttrs, p)
		if err != nil {
			return res, err
		}
		newOut, err := g.CheckAttrAll(newAttrs, p)
		if err != nil {
			return res, err
		}
		if oldOut != newOut {
			oldEnc := strings.Contains(oldOut, "filter: git-crypt")
			newEnc := strings.Contains(newOut, "filter: git-crypt")
			if oldEnc != newEnc {
				res.Flipped = true
				res.FlippedPaths = append(res.FlippedPaths, p)
			}
		}
	}
	if res.Flipped && !acceptAttrChanges {
		return res, ErrAttributeChangeRejected
	}
	return res, nil
}

// ErrAttributeChangeRejected is the sentinel ApplyAttributes returns
// when an encryption-filter flip is detected without
// --accept-attribute-changes. Callers should print the same hint Python
// does ("Re-run with --accept-attribute-changes.") and exit 1.
var ErrAttributeChangeRejected = errors.New("encryption pattern change detected; re-run with --accept-attribute-changes")

// ApplyNewFile mirrors `sync.py apply-new-file`. Decision must be one
// of SyncDecisionOverwrite / Skip / MarkAsUserDeleted.
func ApplyNewFile(newTree, userTree, rel string, decision SyncDecision) error {
	switch decision {
	case SyncDecisionSkip:
		return nil
	case SyncDecisionOverwrite:
		src := filepath.Join(newTree, rel)
		dst := filepath.Join(userTree, rel)
		si, err := os.Stat(src)
		if err != nil || si.IsDir() {
			return fmt.Errorf("source not found: %s", src)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := copyFile(src, dst); err != nil {
			return err
		}
		return os.Chmod(dst, si.Mode().Perm())
	case SyncDecisionMarkAsUserDeleted:
		// On-disk no-op. Caller records the deletion in template.json.
		return nil
	default:
		return fmt.Errorf("unknown decision: %s", decision)
	}
}

// ApplyDeletion mirrors `sync.py apply-deletion`. Decision must be
// SyncDecisionRemove or SyncDecisionPreserveLocal.
func ApplyDeletion(userTree, rel string, decision SyncDecision) error {
	switch decision {
	case SyncDecisionPreserveLocal:
		return nil
	case SyncDecisionRemove:
		target := filepath.Join(userTree, rel)
		if isFile(target) {
			return os.Remove(target)
		}
		return nil
	default:
		return fmt.Errorf("unknown decision: %s", decision)
	}
}

// HasConflictMarkers mirrors `sync.py has-conflict-markers`. It scans
// each tree/path file for the byte sequence "<<<<<<<" at line start.
// Returns the list of paths with markers; an empty slice means clean.
func HasConflictMarkers(tree string, paths []string) ([]string, error) {
	var hits []string
	for _, p := range paths {
		full := filepath.Join(tree, p)
		fi, err := os.Stat(full)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		if fi.IsDir() {
			continue
		}
		f, err := os.Open(full)
		if err != nil {
			return nil, err
		}
		// Match Python's iter-over-bytes-line approach.
		scanner := bufio.NewScanner(f)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			if strings.HasPrefix(string(line), "<<<<<<<") {
				hits = append(hits, p)
				break
			}
		}
		_ = f.Close()
	}
	return hits, nil
}

// copyFile is a streaming file copy that does NOT preserve mode bits;
// callers chmod the destination explicitly. Mirrors shutil.copyfile.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// splitLines splits s on '\n', dropping a trailing empty entry produced
// by an "...\n"-terminated string.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}
