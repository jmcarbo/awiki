package template

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeArchiver copies a fixture directory into destDir (mirroring `git
// archive` for tests). The archiver skips paths that look like they
// belong to the cache (.awiki/) to mirror `git archive HEAD` only
// seeing tracked files; this makes the fake safe even when the test
// points src at a directory that will later contain `.awiki/`.
type fakeArchiver struct {
	src string
}

func (f fakeArchiver) ArchiveTreeToDir(repoRoot, dst string) error {
	srcRoot := f.src
	if srcRoot == "" {
		srcRoot = repoRoot
	}
	return copyDirSkipAwiki(srcRoot, dst)
}

func copyDirSkipAwiki(srcRoot, dstRoot string) error {
	return walkDirCopying(srcRoot, dstRoot, func(rel string) bool {
		// Skip .awiki/ entirely (matches `git archive HEAD` exclusion
		// of untracked, since .awiki is .gitignored in template repos).
		return rel == ".awiki" || rel[:0] == "" && (len(rel) >= 6 && rel[:6] == ".awiki" && (len(rel) == 6 || rel[6] == '/'))
	})
}

func walkDirCopying(srcRoot, dstRoot string, skip func(rel string) bool) error {
	return walkDir(srcRoot, func(path, rel string, info os.FileInfo) error {
		if rel == "." {
			return nil
		}
		if skip != nil && skip(rel) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dst := filepath.Join(dstRoot, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, info.Mode().Perm())
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, info.Mode().Perm())
	})
}

func walkDir(root string, fn func(path, rel string, info os.FileInfo) error) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		return fn(path, rel, info)
	})
}

func writeBootstrappedRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "BOOTSTRAP.md"), []byte(fixtureBootstrapMD), 0o644); err != nil {
		t.Fatal(err)
	}
	mp := filepath.Join(repo, "template.manifest.toml")
	if err := os.WriteFile(mp, []byte(`schema_version = 1
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
ordered_steps = ["dep-check", "domain", "stage-commit"]
[bootstrap.dangerous]
ids = []
`), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestRetrofit_AlreadyHasProvenance_NoOp(t *testing.T) {
	repo := writeBootstrappedRepo(t)
	pj := filepath.Join(repo, ".awiki", "template.json")
	_ = os.MkdirAll(filepath.Dir(pj), 0o755)
	_ = os.WriteFile(pj, []byte(`{"schema_version":1}`), 0o644)
	res, err := Retrofit(fakeArchiver{src: repo}, RetrofitInput{
		RepoRoot: repo,
	})
	if err != nil {
		t.Fatalf("Retrofit: %v", err)
	}
	if !res.AlreadyHasProvenance {
		t.Errorf("expected AlreadyHasProvenance=true")
	}
	if res.InitRan {
		t.Errorf("InitTemplate must not run when provenance exists")
	}
}

func TestRetrofit_HeuristicsOnly(t *testing.T) {
	repo := writeBootstrappedRepo(t)
	_ = os.MkdirAll(filepath.Join(repo, ".awiki"), 0o755)
	_ = os.WriteFile(filepath.Join(repo, ".awiki", "qmd-status"), []byte("ok"), 0o644)
	res, err := Retrofit(fakeArchiver{src: repo}, RetrofitInput{
		RepoRoot: repo, HeuristicsOnly: true,
	})
	if err != nil {
		t.Fatalf("Retrofit: %v", err)
	}
	if res.InitRan {
		t.Errorf("InitTemplate must not run with HeuristicsOnly")
	}
	if !contains(res.HeuristicHits, "install-qmd") {
		t.Errorf("hits=%v missing install-qmd", res.HeuristicHits)
	}
}

func TestRetrofit_DetectsGitCrypt(t *testing.T) {
	repo := writeBootstrappedRepo(t)
	_ = os.WriteFile(filepath.Join(repo, ".gitattributes"),
		[]byte("secrets/** filter=git-crypt diff=git-crypt"), 0o644)
	res, err := Retrofit(fakeArchiver{src: repo}, RetrofitInput{
		RepoRoot: repo, HeuristicsOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.HeuristicLines, "\n")
	if !strings.Contains(joined, "privacy=git-crypt") {
		t.Errorf("lines=%q missing git-crypt detection", joined)
	}
}

func TestRetrofit_DetectsAge(t *testing.T) {
	repo := writeBootstrappedRepo(t)
	_ = os.MkdirAll(filepath.Join(repo, "secrets"), 0o755)
	_ = os.WriteFile(filepath.Join(repo, "secrets", "age-key.txt"), []byte("AGE-..."), 0o644)
	res, _ := Retrofit(fakeArchiver{src: repo}, RetrofitInput{
		RepoRoot: repo, HeuristicsOnly: true,
	})
	joined := strings.Join(res.HeuristicLines, "\n")
	if !strings.Contains(joined, "privacy=age") {
		t.Errorf("lines=%q missing age detection", joined)
	}
}

func TestRetrofit_DetectsMCP(t *testing.T) {
	repo := writeBootstrappedRepo(t)
	_ = os.WriteFile(filepath.Join(repo, ".mcp.json"),
		[]byte(`{"mcpServers": {"awiki-server": {"command": "x"}}}`), 0o644)
	res, _ := Retrofit(fakeArchiver{src: repo}, RetrofitInput{
		RepoRoot: repo, HeuristicsOnly: true,
	})
	joined := strings.Join(res.HeuristicLines, "\n")
	if !strings.Contains(joined, "wire-awiki-mcp") {
		t.Errorf("lines=%q missing wire-awiki-mcp detection", joined)
	}
}

func TestRetrofit_RunsInit(t *testing.T) {
	repo := writeBootstrappedRepo(t)
	res, err := Retrofit(fakeArchiver{src: repo}, RetrofitInput{
		RepoRoot: repo,
		Repo:     "https://x/y.git",
		Ref:      "main",
		Version:  "0.1.0",
		Commit:   "deadbeef",
	})
	if err != nil {
		t.Fatalf("Retrofit: %v", err)
	}
	if !res.InitRan {
		t.Errorf("InitRan=false")
	}
	pj := filepath.Join(repo, ".awiki", "template.json")
	if !isFile(pj) {
		t.Errorf("template.json not written")
	}
	prov, err := LoadProvenance(pj)
	if err != nil {
		t.Fatal(err)
	}
	if prov.Commit != "deadbeef" {
		t.Errorf("commit=%q", prov.Commit)
	}
	if len(prov.BootstrapStepsDone) != 3 {
		t.Errorf("steps=%d want 3", len(prov.BootstrapStepsDone))
	}
}

func TestRetrofit_MissingArgs(t *testing.T) {
	repo := writeBootstrappedRepo(t)
	_, err := Retrofit(fakeArchiver{src: repo}, RetrofitInput{
		RepoRoot: repo,
		// no repo/version/commit
	})
	if err == nil {
		t.Errorf("expected error for missing args")
	}
}

func TestMarkStepSkipped(t *testing.T) {
	dir := t.TempDir()
	pj := filepath.Join(dir, "template.json")
	body := `{
  "schema_version": 1,
  "bootstrap_steps_done": [{"id": "domain", "status": "applied", "content_hash": "sha256:abc"}]
}
`
	_ = os.WriteFile(pj, []byte(body), 0o644)
	if err := MarkStepSkipped(pj, "domain"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(pj)
	var d map[string]any
	_ = json.Unmarshal(data, &d)
	steps, _ := d["bootstrap_steps_done"].([]any)
	s := steps[0].(map[string]any)
	if s["status"].(string) != "skipped" {
		t.Errorf("status=%v", s["status"])
	}
	if s["reason"].(string) != "retrofit: user said no" {
		t.Errorf("reason=%v", s["reason"])
	}
}

// --- merge tests ------------------------------------------------------------

type fakeMergeGit struct {
	merges []mergeCall
	rc     int
	err    error
}

type mergeCall struct {
	cur, base, newFile string
}

func (f *fakeMergeGit) MergeFile(cur, base, newFile string) (int, error) {
	f.merges = append(f.merges, mergeCall{cur, base, newFile})
	return f.rc, f.err
}
func (f *fakeMergeGit) LsFilesIn(dir string) (string, int, error)            { return "", 0, nil }
func (f *fakeMergeGit) CheckAttrAll(attrFile, path string) (string, error)   { return "", nil }

func TestMergeFiles_ClearMissing(t *testing.T) {
	dir := t.TempDir()
	cur := filepath.Join(dir, "cur")
	base := filepath.Join(dir, "base")
	newF := filepath.Join(dir, "new")
	for _, p := range []string{cur, newF} {
		_ = os.WriteFile(p, []byte("x"), 0o644)
	}
	g := &fakeMergeGit{}
	_, err := MergeFiles(g, cur, base, newF)
	if err == nil || !strings.Contains(err.Error(), "base file not found") {
		t.Errorf("err=%v", err)
	}
}

func TestMergeFiles_DelegatesToGit(t *testing.T) {
	dir := t.TempDir()
	cur := filepath.Join(dir, "cur")
	base := filepath.Join(dir, "base")
	newF := filepath.Join(dir, "new")
	for _, p := range []string{cur, base, newF} {
		_ = os.WriteFile(p, []byte("x"), 0o644)
	}
	g := &fakeMergeGit{rc: 0}
	rc, err := MergeFiles(g, cur, base, newF)
	if err != nil {
		t.Fatal(err)
	}
	if rc != 0 || len(g.merges) != 1 {
		t.Errorf("rc=%d merges=%v", rc, g.merges)
	}
}

func TestMergeFiles_NilAdapter(t *testing.T) {
	_, err := MergeFiles(nil, "a", "b", "c")
	if err == nil {
		t.Errorf("nil adapter must return error")
	}
}

func TestRetrofit_NilArchiver(t *testing.T) {
	repo := writeBootstrappedRepo(t)
	_, err := Retrofit(nil, RetrofitInput{
		RepoRoot: repo,
		Repo:     "x", Ref: "main", Version: "0.1.0", Commit: "abc",
	})
	if err == nil {
		t.Errorf("nil archiver must return error")
	}
	if errors.Is(err, ErrStepNotFound) {
		t.Errorf("wrong error class: %v", err)
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
