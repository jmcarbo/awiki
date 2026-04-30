package template

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// fakeSyncGit is a deterministic stub for SyncGit.
type fakeSyncGit struct {
	mergeCode int
	mergeErr  error
	// captured inputs for assertions
	mergeCalls []string
	lsFiles    string
	checkAttr  func(attrFile, path string) (string, error)
}

func (f *fakeSyncGit) MergeFile(cur, base, newFile string) (int, error) {
	f.mergeCalls = append(f.mergeCalls, cur+"|"+base+"|"+newFile)
	return f.mergeCode, f.mergeErr
}
func (f *fakeSyncGit) LsFilesIn(dir string) (string, int, error) {
	return f.lsFiles, 0, nil
}
func (f *fakeSyncGit) CheckAttrAll(attrFile, path string) (string, error) {
	if f.checkAttr != nil {
		return f.checkAttr(attrFile, path)
	}
	return "", nil
}

func writeFile(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestApplyOverwrite_CopiesAndPreservesMode(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "new", "scripts", "x.sh")
	writeFile(t, src, "#!/bin/sh\necho hi\n")
	if err := os.Chmod(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ApplyOverwrite(filepath.Join(dir, "new"), filepath.Join(dir, "user"), "scripts/x.sh"); err != nil {
		t.Fatalf("ApplyOverwrite: %v", err)
	}
	dst := filepath.Join(dir, "user", "scripts", "x.sh")
	body, err := os.ReadFile(dst)
	if err != nil || string(body) != "#!/bin/sh\necho hi\n" {
		t.Errorf("body=%q err=%v", body, err)
	}
	fi, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o755 {
		t.Errorf("mode=%v want 0755", fi.Mode().Perm())
	}
}

func TestApplyOverwrite_MissingSource(t *testing.T) {
	dir := t.TempDir()
	err := ApplyOverwrite(filepath.Join(dir, "new"), filepath.Join(dir, "user"), "missing.txt")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestApplyThreeWay_NewTreeMissing_NoOp(t *testing.T) {
	dir := t.TempDir()
	g := &fakeSyncGit{}
	code, err := ApplyThreeWay(g, filepath.Join(dir, "old"), filepath.Join(dir, "new"), filepath.Join(dir, "user"), "x.md")
	if err != nil || code != 0 {
		t.Errorf("code=%d err=%v", code, err)
	}
	if len(g.mergeCalls) != 0 {
		t.Errorf("merge should not run: %v", g.mergeCalls)
	}
}

func TestApplyThreeWay_UserDeleted_TreatsAsNewFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "new", "x.md"), "newbody\n")
	g := &fakeSyncGit{}
	code, err := ApplyThreeWay(g, filepath.Join(dir, "old"), filepath.Join(dir, "new"), filepath.Join(dir, "user"), "x.md")
	if err != nil || code != 0 {
		t.Errorf("code=%d err=%v", code, err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "user", "x.md"))
	if err != nil || string(body) != "newbody\n" {
		t.Errorf("body=%q err=%v", body, err)
	}
}

func TestApplyThreeWay_NoBase_FallbackOverwrite(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "new", "x.md"), "newbody\n")
	writeFile(t, filepath.Join(dir, "user", "x.md"), "userbody\n")
	g := &fakeSyncGit{}
	code, err := ApplyThreeWay(g, filepath.Join(dir, "old"), filepath.Join(dir, "new"), filepath.Join(dir, "user"), "x.md")
	if err != nil || code != 0 {
		t.Errorf("code=%d err=%v", code, err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "user", "x.md"))
	if string(body) != "newbody\n" {
		t.Errorf("body=%q want newbody", body)
	}
	if len(g.mergeCalls) != 0 {
		t.Errorf("merge should not run when base missing: %v", g.mergeCalls)
	}
}

func TestApplyThreeWay_AllPresent_RunsMerge(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "old", "x.md"), "base\n")
	writeFile(t, filepath.Join(dir, "new", "x.md"), "new\n")
	writeFile(t, filepath.Join(dir, "user", "x.md"), "user\n")
	g := &fakeSyncGit{mergeCode: 0}
	code, err := ApplyThreeWay(g, filepath.Join(dir, "old"), filepath.Join(dir, "new"), filepath.Join(dir, "user"), "x.md")
	if err != nil || code != 0 {
		t.Errorf("code=%d err=%v", code, err)
	}
	if len(g.mergeCalls) != 1 {
		t.Errorf("expected 1 merge call, got %v", g.mergeCalls)
	}
}

func TestApplyAttributes_NoFlip_Clean(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "old", ".gitattributes"), "*.bin filter=git-crypt\n")
	writeFile(t, filepath.Join(dir, "new", ".gitattributes"), "*.bin filter=git-crypt\n")
	writeFile(t, filepath.Join(dir, "user", ".gitattributes"), "*.bin filter=git-crypt\n")
	g := &fakeSyncGit{
		lsFiles: "secrets/foo.bin\n",
		checkAttr: func(attrFile, path string) (string, error) {
			return "secrets/foo.bin: filter: git-crypt\n", nil
		},
	}
	res, err := ApplyAttributes(g, filepath.Join(dir, "old"), filepath.Join(dir, "new"), filepath.Join(dir, "user"), ".gitattributes", false)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if res.Flipped {
		t.Errorf("expected no flip, got %v", res.FlippedPaths)
	}
}

func TestApplyAttributes_FlipDetected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "old", ".gitattributes"), "*.bin -filter\n")
	writeFile(t, filepath.Join(dir, "new", ".gitattributes"), "*.bin filter=git-crypt\n")
	writeFile(t, filepath.Join(dir, "user", ".gitattributes"), "*.bin filter=git-crypt\n")
	g := &fakeSyncGit{
		lsFiles: "secrets/foo.bin\n",
		checkAttr: func(attrFile, path string) (string, error) {
			if filepath.Base(filepath.Dir(attrFile)) == "old" {
				return "secrets/foo.bin: filter: unspecified\n", nil
			}
			return "secrets/foo.bin: filter: git-crypt\n", nil
		},
	}
	_, err := ApplyAttributes(g, filepath.Join(dir, "old"), filepath.Join(dir, "new"), filepath.Join(dir, "user"), ".gitattributes", false)
	if !errors.Is(err, ErrAttributeChangeRejected) {
		t.Errorf("err=%v want ErrAttributeChangeRejected", err)
	}
}

func TestApplyAttributes_FlipAccepted(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "old", ".gitattributes"), "*.bin -filter\n")
	writeFile(t, filepath.Join(dir, "new", ".gitattributes"), "*.bin filter=git-crypt\n")
	writeFile(t, filepath.Join(dir, "user", ".gitattributes"), "*.bin filter=git-crypt\n")
	g := &fakeSyncGit{
		lsFiles: "secrets/foo.bin\n",
		checkAttr: func(attrFile, path string) (string, error) {
			if filepath.Base(filepath.Dir(attrFile)) == "old" {
				return "secrets/foo.bin: filter: unspecified\n", nil
			}
			return "secrets/foo.bin: filter: git-crypt\n", nil
		},
	}
	res, err := ApplyAttributes(g, filepath.Join(dir, "old"), filepath.Join(dir, "new"), filepath.Join(dir, "user"), ".gitattributes", true)
	if err != nil {
		t.Errorf("err=%v want nil with accept", err)
	}
	if !res.Flipped {
		t.Errorf("expected flip detected even when accepted")
	}
}

func TestApplyNewFile_Skip(t *testing.T) {
	dir := t.TempDir()
	if err := ApplyNewFile(filepath.Join(dir, "new"), filepath.Join(dir, "user"), "x.md", SyncDecisionSkip); err != nil {
		t.Errorf("err=%v", err)
	}
}

func TestApplyNewFile_Overwrite(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "new", "x.md"), "body\n")
	if err := ApplyNewFile(filepath.Join(dir, "new"), filepath.Join(dir, "user"), "x.md", SyncDecisionOverwrite); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "user", "x.md")); err != nil {
		t.Errorf("expected file copied: %v", err)
	}
}

func TestApplyNewFile_MarkAsUserDeleted_NoOp(t *testing.T) {
	dir := t.TempDir()
	if err := ApplyNewFile(filepath.Join(dir, "new"), filepath.Join(dir, "user"), "x.md", SyncDecisionMarkAsUserDeleted); err != nil {
		t.Errorf("err=%v", err)
	}
}

func TestApplyDeletion_Remove(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "user", "x.md"), "body\n")
	if err := ApplyDeletion(filepath.Join(dir, "user"), "x.md", SyncDecisionRemove); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "user", "x.md")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected removed: %v", err)
	}
}

func TestApplyDeletion_PreserveLocal(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "user", "x.md"), "body\n")
	if err := ApplyDeletion(filepath.Join(dir, "user"), "x.md", SyncDecisionPreserveLocal); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "user", "x.md")); err != nil {
		t.Errorf("expected file preserved: %v", err)
	}
}

func TestHasConflictMarkers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "clean.md"), "hello\n")
	writeFile(t, filepath.Join(dir, "dirty.md"), "<<<<<<< current\nfoo\n=======\nbar\n>>>>>>> new\n")
	hits, err := HasConflictMarkers(dir, []string{"clean.md", "dirty.md", "missing.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0] != "dirty.md" {
		t.Errorf("hits=%v want [dirty.md]", hits)
	}
}
