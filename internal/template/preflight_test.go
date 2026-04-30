package template

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakePreflightGit is a deterministic stub for PreflightGit.
type fakePreflightGit struct {
	diffIndexCode int
	diffIndexErr  error
	lsFilesOut    string
	lsFilesCode   int
	lsFilesErr    error
	subStatOut    string
	subStatCode   int
	subStatErr    error
	branchOut     string
	branchCode    int
	branchErr     error
	cryptEncOut   string
	cryptEncCode  int
	cryptEncErr   error
	cryptStatOut  string
	cryptStatCode int
	cryptStatErr  error
}

func (f *fakePreflightGit) DiffIndexQuiet() (int, error) {
	return f.diffIndexCode, f.diffIndexErr
}
func (f *fakePreflightGit) LsFilesUntracked() (string, int, error) {
	return f.lsFilesOut, f.lsFilesCode, f.lsFilesErr
}
func (f *fakePreflightGit) SubmoduleStatus() (string, int, error) {
	return f.subStatOut, f.subStatCode, f.subStatErr
}
func (f *fakePreflightGit) CurrentBranch() (string, int, error) {
	return f.branchOut, f.branchCode, f.branchErr
}
func (f *fakePreflightGit) GitCryptStatusEncrypted() (string, int, error) {
	return f.cryptEncOut, f.cryptEncCode, f.cryptEncErr
}
func (f *fakePreflightGit) GitCryptStatus() (string, int, error) {
	return f.cryptStatOut, f.cryptStatCode, f.cryptStatErr
}

func TestCheckTree_Clean(t *testing.T) {
	g := &fakePreflightGit{
		diffIndexCode: 0,
		lsFilesOut:    "",
		lsFilesCode:   0,
		subStatOut:    "",
		subStatCode:   0,
	}
	if err := CheckTree(g); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestCheckTree_DirtyDiff(t *testing.T) {
	g := &fakePreflightGit{diffIndexCode: 1}
	err := CheckTree(g)
	if err == nil || !strings.Contains(err.Error(), "staged or unstaged tracked changes") {
		t.Errorf("err=%v", err)
	}
}

func TestCheckTree_Untracked(t *testing.T) {
	g := &fakePreflightGit{lsFilesOut: "foo.txt\n"}
	err := CheckTree(g)
	if err == nil || !strings.Contains(err.Error(), "untracked files") {
		t.Errorf("err=%v", err)
	}
}

func TestCheckTree_DirtySubmodule(t *testing.T) {
	g := &fakePreflightGit{subStatOut: "+abcd123 sub (rev)\n"}
	err := CheckTree(g)
	if err == nil || !strings.Contains(err.Error(), "submodule") {
		t.Errorf("err=%v", err)
	}
}

func TestCheckBranch_Match(t *testing.T) {
	g := &fakePreflightGit{branchOut: "main\n"}
	if err := CheckBranch(g, "main"); err != nil {
		t.Errorf("err=%v", err)
	}
}

func TestCheckBranch_OnUpdateBranchHalts(t *testing.T) {
	g := &fakePreflightGit{branchOut: "awiki-template-update/abc\n"}
	err := CheckBranch(g, "main")
	if err == nil || !strings.Contains(err.Error(), "on update branch") {
		t.Errorf("err=%v", err)
	}
}

func TestCheckBranch_Mismatch(t *testing.T) {
	g := &fakePreflightGit{branchOut: "feat/x\n"}
	err := CheckBranch(g, "main")
	if err == nil || !strings.Contains(err.Error(), "expected main") {
		t.Errorf("err=%v", err)
	}
}

func TestCheckBranch_NotInRepo(t *testing.T) {
	g := &fakePreflightGit{branchCode: 128}
	err := CheckBranch(g, "main")
	if !errors.Is(err, ErrPreflightNotInRepo) {
		t.Errorf("err=%v want ErrPreflightNotInRepo", err)
	}
}

func TestCheckPendingPrompts_NoDir(t *testing.T) {
	root := t.TempDir()
	if err := CheckPendingPrompts(root); err != nil {
		t.Errorf("err=%v want nil for missing dir", err)
	}
}

func TestCheckPendingPrompts_Empty(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".awiki", "pending-prompts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := CheckPendingPrompts(root); err != nil {
		t.Errorf("err=%v want nil for empty dir", err)
	}
}

func TestCheckPendingPrompts_HasFiles(t *testing.T) {
	root := t.TempDir()
	pp := filepath.Join(root, ".awiki", "pending-prompts")
	if err := os.MkdirAll(pp, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pp, "0001-foo.prompt.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pp, "0002-bar.prompt.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := CheckPendingPrompts(root)
	if err == nil || !strings.Contains(err.Error(), "halt: pending LLM migrations") {
		t.Errorf("err=%v", err)
	}
	// Sorted output: 0001-foo before 0002-bar.
	idx1 := strings.Index(err.Error(), "0001-foo")
	idx2 := strings.Index(err.Error(), "0002-bar")
	if idx1 < 0 || idx2 < 0 || idx1 > idx2 {
		t.Errorf("expected sorted output; err=%q", err.Error())
	}
}

func TestCheckEncryption_NoCryptBinary(t *testing.T) {
	g := &fakePreflightGit{cryptEncCode: 127}
	if err := CheckEncryption(g, "/dev/null"); err != nil {
		t.Errorf("err=%v want nil silent skip", err)
	}
}

func TestCheckEncryption_NoEncryptedPaths(t *testing.T) {
	g := &fakePreflightGit{cryptEncOut: ""}
	if err := CheckEncryption(g, "/dev/null"); err != nil {
		t.Errorf("err=%v", err)
	}
}

func TestCheckEncryption_LockedAndMergeRequired(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "template.manifest.toml")
	body := `schema_version = 1
template_version = "0.0.0"
[strategies]
overwrite = []
preserve = []
three_way = []
attributes_merge = ["secrets/**"]
template_only = []
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = []
[bootstrap.dangerous]
ids = []
`
	if err := os.WriteFile(manifest, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &fakePreflightGit{
		cryptEncOut:  "secrets/db.env\n",
		cryptStatOut: "Some paths are locked\n",
	}
	err := CheckEncryption(g, manifest)
	if err == nil || !strings.Contains(err.Error(), "halt: encrypted path secrets/db.env") {
		t.Errorf("err=%v", err)
	}
}

func TestCheckEncryption_UnlockedSkips(t *testing.T) {
	// Python oracle considers a status output "locked" when the literal
	// word "locked" appears (case-insensitive). The unlocked-without-
	// any-"locked"-substring path is what skips. We mirror the oracle
	// exactly: status text containing only the word "Encrypted." (no
	// "locked") should skip.
	dir := t.TempDir()
	manifest := filepath.Join(dir, "template.manifest.toml")
	body := `schema_version = 1
template_version = "0.0.0"
[strategies]
attributes_merge = ["secrets/**"]
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = []
[bootstrap.dangerous]
ids = []
`
	if err := os.WriteFile(manifest, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &fakePreflightGit{
		cryptEncOut:  "secrets/db.env\n",
		cryptStatOut: "Encrypted: yes\n",
	}
	if err := CheckEncryption(g, manifest); err != nil {
		t.Errorf("err=%v want nil for unlocked status (no 'locked' substring)", err)
	}
}
