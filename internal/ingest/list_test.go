package ingest

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"awiki/internal/testutil"
)

// listFixtureRoot resolves the slice 3 fixture tree for a given verb
// (`batch-list` or `git-list`). Mirrors captureFixtureRoot.
func listFixtureRoot(t *testing.T, verb string) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
	return filepath.Join(repoRoot, "tests", "fixtures", "ingest", verb)
}

// runListFixture is the shared driver: it copies input/ into a temp
// RepoRoot, invokes the verb, and asserts stdout + the expected_files/
// tree. invoke is the per-verb call that writes to stdout.
func runListFixture(t *testing.T, verb string, invoke func(*Runner, *bytes.Buffer) error) {
	t.Helper()
	fixture := listFixtureRoot(t, verb)

	tmp := t.TempDir()
	testutil.CopyTree(t, filepath.Join(fixture, "input"), tmp)

	r := &Runner{RepoRoot: tmp}
	var stdout bytes.Buffer
	if err := invoke(r, &stdout); err != nil {
		t.Fatalf("invoke %s: %v", verb, err)
	}

	wantStdout, err := os.ReadFile(filepath.Join(fixture, "expected_stdout"))
	if err != nil {
		t.Fatalf("read expected_stdout: %v", err)
	}
	if got := stdout.String(); got != string(wantStdout) {
		t.Errorf("%s stdout mismatch:\n got: %q\nwant: %q", verb, got, string(wantStdout))
	}

	expectedRoot := filepath.Join(fixture, "expected_files")
	if err := filepath.Walk(expectedRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(expectedRoot, path)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(filepath.Join(tmp, rel))
		if err != nil {
			t.Errorf("missing expected file %s: %v", rel, err)
			return nil
		}
		want, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			t.Errorf("file %s mismatch\n got: %q\nwant: %q", rel, got, want)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk expected_files: %v", err)
	}
}

func TestListBatchFixture(t *testing.T) {
	runListFixture(t, "batch-list", func(r *Runner, out *bytes.Buffer) error {
		return r.ListBatch(out)
	})
}

func TestListGitFixture(t *testing.T) {
	runListFixture(t, "git-list", func(r *Runner, out *bytes.Buffer) error {
		return r.ListGit(out)
	})
}

// --------------------------------------------------------------------
// Behavioral tests covering the bash-tolerance edges. These pin the
// "missing dir returns no error, no output" rule for both verbs and the
// sort order against an unsorted on-disk layout.
// --------------------------------------------------------------------

func TestListBatchMissingDir(t *testing.T) {
	tmp := t.TempDir()
	r := &Runner{RepoRoot: tmp}
	var out bytes.Buffer
	if err := r.ListBatch(&out); err != nil {
		t.Fatalf("ListBatch on missing dir: unexpected error %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("ListBatch on missing dir: expected empty stdout, got %q", out.String())
	}
}

func TestListGitMissingDir(t *testing.T) {
	tmp := t.TempDir()
	r := &Runner{RepoRoot: tmp}
	var out bytes.Buffer
	if err := r.ListGit(&out); err != nil {
		t.Fatalf("ListGit on missing dir: unexpected error %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("ListGit on missing dir: expected empty stdout, got %q", out.String())
	}
}

func TestListBatchEmptyDir(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "raw", "inbox", "batch"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &Runner{RepoRoot: tmp}
	var out bytes.Buffer
	if err := r.ListBatch(&out); err != nil {
		t.Fatalf("ListBatch on empty dir: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("ListBatch on empty dir: expected empty stdout, got %q", out.String())
	}
}

func TestListBatchSortOrder(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "raw", "inbox", "batch")
	if err := os.MkdirAll(filepath.Join(root, "z-sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Create files in non-sorted order to confirm sort.Strings runs.
	for _, p := range []string{"z-sub/inner.txt", "a.txt", "m.md"} {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := &Runner{RepoRoot: tmp}
	var out bytes.Buffer
	if err := r.ListBatch(&out); err != nil {
		t.Fatal(err)
	}
	want := "raw/inbox/batch/a.txt\nraw/inbox/batch/m.md\nraw/inbox/batch/z-sub/inner.txt\n"
	if got := out.String(); got != want {
		t.Errorf("sort order mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestListGitStripsJSONSuffix(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, ".awiki", "git-state")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"zeta.json", "alpha.json", "mid.json"} {
		if err := os.WriteFile(filepath.Join(root, n), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := &Runner{RepoRoot: tmp}
	var out bytes.Buffer
	if err := r.ListGit(&out); err != nil {
		t.Fatal(err)
	}
	want := "alpha\nmid\nzeta\n"
	if got := out.String(); got != want {
		t.Errorf("git-list mismatch\n got: %q\nwant: %q", got, want)
	}
}
