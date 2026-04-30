package template

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeUpdateGit aggregates every UpdateGit method into a single fake.
// Tests that don't exercise a method leave it returning the zero value.
type fakeUpdateGit struct {
	fakeBSGit  // for HasUpdateBranch / Checkout / etc.
	mergeRC int
	currentBranch string
	hasBranch     map[string]bool
	headSHA       string
	refToSHA      map[string]string
	archived      map[string]string // dst -> src
	clones        []cloneCall
}

type cloneCall struct{ source, dest string }

func (f *fakeUpdateGit) MergeFile(cur, base, newF string) (int, error) {
	return f.mergeRC, nil
}
func (f *fakeUpdateGit) LsFilesIn(dir string) (string, int, error)            { return "", 0, nil }
func (f *fakeUpdateGit) CheckAttrAll(attrFile, path string) (string, error)   { return "", nil }
func (f *fakeUpdateGit) MergeFileDryRun(cur, base, newB []byte) (int, error)  { return 0, nil }
func (f *fakeUpdateGit) FetchClone(source, dest string) error {
	f.clones = append(f.clones, cloneCall{source, dest})
	if f.archived == nil {
		f.archived = map[string]string{}
	}
	// Deposit source contents at dest if source is a directory.
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		_ = os.MkdirAll(dest, 0o755)
		_ = CopyDirContents(source, dest)
	}
	return nil
}
func (f *fakeUpdateGit) FetchUpdate(dir string) error                  { return nil }
func (f *fakeUpdateGit) RevParseHEAD(dir string) (string, error)       { return f.headSHA, nil }
func (f *fakeUpdateGit) RevParseRef(dir, ref string) (string, error) {
	if v, ok := f.refToSHA[ref]; ok {
		return v, nil
	}
	return f.headSHA, nil
}
func (f *fakeUpdateGit) CurrentBranchName() (string, error)            { return f.currentBranch, nil }
func (f *fakeUpdateGit) VerifyTag(dir, target string) error            { return errors.New("no tag") }
func (f *fakeUpdateGit) VerifyCommit(dir, target string) error         { return errors.New("no commit sig") }
func (f *fakeUpdateGit) ResetHard(ref string) error                    { return nil }
func (f *fakeUpdateGit) HasBranch(branch string) (bool, error) {
	if f.hasBranch == nil {
		return false, nil
	}
	return f.hasBranch[branch], nil
}
func (f *fakeUpdateGit) ArchiveTreeToDir(repoRoot, dst string) error {
	if f.archived == nil {
		f.archived = map[string]string{}
	}
	f.archived[dst] = repoRoot
	_ = os.MkdirAll(dst, 0o755)
	// Best-effort copy excluding .awiki/ and .git/.
	return walkSkip(repoRoot, dst, []string{".awiki", ".git"})
}

func walkSkip(srcRoot, dstRoot string, skipNames []string) error {
	skip := map[string]bool{}
	for _, s := range skipNames {
		skip[s] = true
	}
	return filepath.Walk(srcRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(srcRoot, path)
		if rel == "." {
			return nil
		}
		// Check first path component.
		first := strings.SplitN(rel, string(filepath.Separator), 2)[0]
		if skip[first] {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dst := filepath.Join(dstRoot, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, info.Mode().Perm())
		}
		_ = os.MkdirAll(filepath.Dir(dst), 0o755)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, info.Mode().Perm())
	})
}

type fakePreflight struct{}

func (fakePreflight) DiffIndexQuiet() (int, error)              { return 0, nil }
func (fakePreflight) LsFilesUntracked() (string, int, error)    { return "", 0, nil }
func (fakePreflight) SubmoduleStatus() (string, int, error)     { return "", 0, nil }
func (fakePreflight) CurrentBranch() (string, int, error)       { return "main", 0, nil }
func (fakePreflight) GitCryptStatusEncrypted() (string, int, error) { return "", 127, nil }
func (fakePreflight) GitCryptStatus() (string, int, error)      { return "", 0, nil }

func writeProvenance(t *testing.T, root, repo, commit string) {
	t.Helper()
	pj := filepath.Join(root, ".awiki", "template.json")
	_ = os.MkdirAll(filepath.Dir(pj), 0o755)
	body := `{
  "schema_version": 1,
  "repo": "` + repo + `",
  "original_repo": "` + repo + `",
  "ref": "main",
  "version": "0.1.0",
  "commit": "` + commit + `",
  "applied_migrations": [],
  "deleted": [],
  "bootstrap_steps_done": []
}
`
	if err := os.WriteFile(pj, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunUpdate_NoProvenance(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	deps := UpdateDeps{
		Stdout:    &stdout,
		Stderr:    &stderr,
		Git:       &fakeUpdateGit{},
		Bash:      &fakeBSBash{},
		Preflight: fakePreflight{},
	}
	code, err := RunUpdate(deps, root, UpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Errorf("code=%d want 1", code)
	}
	if !strings.Contains(stderr.String(), "no .awiki/template.json") {
		t.Errorf("stderr=%q", stderr.String())
	}
}

func TestRunUpdate_OptOutNone(t *testing.T) {
	root := t.TempDir()
	writeProvenance(t, root, "none", "abc")
	var stdout, stderr bytes.Buffer
	deps := UpdateDeps{
		Stdout:    &stdout,
		Stderr:    &stderr,
		Git:       &fakeUpdateGit{},
		Bash:      &fakeBSBash{},
		Preflight: fakePreflight{},
	}
	code, err := RunUpdate(deps, root, UpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("code=%d want 0", code)
	}
	if !strings.Contains(stderr.String(), "updates disabled") {
		t.Errorf("stderr=%q", stderr.String())
	}
}

func TestRunUpdate_Status(t *testing.T) {
	root := t.TempDir()
	writeProvenance(t, root, "https://x/y.git", "abc")
	var stdout, stderr bytes.Buffer
	deps := UpdateDeps{Stdout: &stdout, Stderr: &stderr, Git: &fakeUpdateGit{}, Bash: &fakeBSBash{}, Preflight: fakePreflight{}}
	code, err := RunUpdate(deps, root, UpdateOptions{Status: true})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("code=%d", code)
	}
	if !strings.Contains(stdout.String(), "version: 0.1.0") {
		t.Errorf("stdout=%q", stdout.String())
	}
}

func TestRunUpdate_GC(t *testing.T) {
	root := t.TempDir()
	writeProvenance(t, root, "https://x/y.git", "abc")
	cacheDir := filepath.Join(root, ".awiki", "template-cache")
	_ = os.MkdirAll(filepath.Join(cacheDir, "abc"), 0o755)
	_ = os.MkdirAll(filepath.Join(cacheDir, "old1"), 0o755)
	_ = os.MkdirAll(filepath.Join(cacheDir, "old2"), 0o755)
	var stdout, stderr bytes.Buffer
	deps := UpdateDeps{Stdout: &stdout, Stderr: &stderr, Git: &fakeUpdateGit{}, Bash: &fakeBSBash{}, Preflight: fakePreflight{}}
	code, _ := RunUpdate(deps, root, UpdateOptions{GC: true})
	if code != 0 {
		t.Errorf("code=%d", code)
	}
	if !strings.Contains(stdout.String(), "cache GC complete") {
		t.Errorf("stdout=%q", stdout.String())
	}
}

func TestRunUpdate_Abort_NoState(t *testing.T) {
	root := t.TempDir()
	writeProvenance(t, root, "https://x/y.git", "abc")
	var stdout, stderr bytes.Buffer
	deps := UpdateDeps{Stdout: &stdout, Stderr: &stderr, Git: &fakeUpdateGit{}, Bash: &fakeBSBash{}, Preflight: fakePreflight{}}
	code, _ := RunUpdate(deps, root, UpdateOptions{Abort: true})
	if code != 0 {
		t.Errorf("code=%d", code)
	}
	if !strings.Contains(stdout.String(), "no in-progress update") {
		t.Errorf("stdout=%q", stdout.String())
	}
}

func TestRunUpdate_AlreadyUpToDate(t *testing.T) {
	root := t.TempDir()
	src := t.TempDir()
	// Set up source dir with manifest + bootstrap.
	_ = os.WriteFile(filepath.Join(src, "template.manifest.toml"), []byte(`schema_version = 1
template_version = "0.1.0"
[strategies]
overwrite = []
preserve = []
three_way = []
attributes_merge = []
template_only = []
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = []
[bootstrap.dangerous]
ids = []
`), 0o644)
	_ = os.WriteFile(filepath.Join(src, "BOOTSTRAP.md"), []byte(fixtureBootstrapMD), 0o644)
	writeProvenance(t, root, src, "samesha")
	// Set up current ancestor cache.
	_ = os.MkdirAll(filepath.Join(root, ".awiki", "template-cache", "samesha"), 0o755)

	var stdout, stderr bytes.Buffer
	gFake := &fakeUpdateGit{headSHA: "samesha"}
	deps := UpdateDeps{
		Stdout: &stdout, Stderr: &stderr,
		Git: gFake, Bash: &fakeBSBash{}, Preflight: fakePreflight{},
	}
	code, err := RunUpdate(deps, root, UpdateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "already up to date") {
		t.Errorf("stdout=%q", stdout.String())
	}
}

func TestRunUpdate_SourceMismatchHalts(t *testing.T) {
	root := t.TempDir()
	writeProvenance(t, root, "https://x/y.git", "abc")
	var stdout, stderr bytes.Buffer
	deps := UpdateDeps{
		Stdout: &stdout, Stderr: &stderr,
		Git: &fakeUpdateGit{}, Bash: &fakeBSBash{}, Preflight: fakePreflight{},
	}
	code, _ := RunUpdate(deps, root, UpdateOptions{Source: "/elsewhere"})
	if code != 1 {
		t.Errorf("code=%d want 1", code)
	}
	if !strings.Contains(stderr.String(), "Source change detected") {
		t.Errorf("stderr=%q", stderr.String())
	}
}

func TestAbortUpdate_DeletesBranchAndFetchDir(t *testing.T) {
	root := t.TempDir()
	fetchDir := filepath.Join(root, ".awiki", "template-cache", "_fetch")
	_ = os.MkdirAll(fetchDir, 0o755)
	state := filepath.Join(fetchDir, ".update-state.json")
	body := `{"phase":"commit-a","status":"started","branch":"awiki-template-update/zzz","commit_old":"x","commit_new":"y","started_at":"2026-04-27T00:00:00Z","last_completed_commit":null,"applied_migrations_pending":[],"bootstrap_steps_pending":[],"deletions_user_decisions":{},"deleted_pending":[]}`
	_ = os.WriteFile(state, []byte(body), 0o644)
	g := &fakeUpdateGit{hasBranch: map[string]bool{"awiki-template-update/zzz": true}}
	var stdout, stderr bytes.Buffer
	deps := UpdateDeps{Stdout: &stdout, Stderr: &stderr, Git: g, Bash: &fakeBSBash{}, Preflight: fakePreflight{}}
	if err := AbortUpdate(deps, root, "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fetchDir); err == nil {
		t.Errorf("fetchDir still exists")
	}
	if len(g.fakeBSGit.deleted) != 1 {
		t.Errorf("deleted=%v", g.fakeBSGit.deleted)
	}
}

func TestRunUpdate_RerunBootstrapStep(t *testing.T) {
	root := t.TempDir()
	writeProvenance(t, root, "https://x/y.git", "abc")
	bs := filepath.Join(root, "BOOTSTRAP.md")
	_ = os.WriteFile(bs, []byte(fixtureBootstrapMD), 0o644)
	var stdout, stderr bytes.Buffer
	g := &fakeUpdateGit{}
	deps := UpdateDeps{Stdout: &stdout, Stderr: &stderr, Git: g, Bash: &fakeBSBash{}, Preflight: fakePreflight{}}
	code, _ := RunUpdate(deps, root, UpdateOptions{
		RerunBootstrapStep: "domain",
		NonInteractive:     true,
	})
	if code != 0 {
		t.Errorf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "rerun-bootstrap-step domain complete") {
		t.Errorf("stdout=%q", stdout.String())
	}
}
