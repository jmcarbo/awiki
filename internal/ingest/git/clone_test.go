package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"awiki/internal/adapters"
)

// fakeGitExt is a minimal in-package GitExt fake. The shared
// testutil.FakeGitExt lands in commit 4 (top-level driver wiring); for
// the clone-helper tests we keep the fake local to avoid an import
// cycle and to keep the helper tests deterministic.
type fakeGitExt struct {
	cloneCalled []string // captures (url, dest) pairs as "url|dest".
	cloneErr    error
	cloneCode   int

	fetchErr  error
	fetchCode int
	resetErr  error
	resetCode int

	revParse    map[string]string // ref -> sha (per checkout, not modeled separately for v1).
	revParseErr error

	symbolicRef    map[string]string // name -> result
	symbolicRefErr error
}

func (f *fakeGitExt) Clone(_ context.Context, url, dest string) (int, error) {
	f.cloneCalled = append(f.cloneCalled, url+"|"+dest)
	if f.cloneErr != nil || f.cloneCode != 0 {
		return f.cloneCode, f.cloneErr
	}
	if err := os.MkdirAll(filepath.Join(dest, ".git"), 0o755); err != nil {
		return 1, err
	}
	return 0, nil
}

func (f *fakeGitExt) RevParse(_ context.Context, _, ref string) (string, int, error) {
	if f.revParseErr != nil {
		return "", 1, f.revParseErr
	}
	if v, ok := f.revParse[ref]; ok {
		return v, 0, nil
	}
	return "", 0, nil
}

func (f *fakeGitExt) Init(_ context.Context, _ string) (int, error) { return 0, nil }

func (f *fakeGitExt) Fetch(_ context.Context, _, _ string) (int, error) {
	return f.fetchCode, f.fetchErr
}

func (f *fakeGitExt) Reset(_ context.Context, _, _ string) (int, error) {
	return f.resetCode, f.resetErr
}

func (f *fakeGitExt) SymbolicRef(_ context.Context, _, name string) (string, int, error) {
	if f.symbolicRefErr != nil {
		return "", 1, f.symbolicRefErr
	}
	if v, ok := f.symbolicRef[name]; ok {
		return v, 0, nil
	}
	return "", 1, errors.New("not a symbolic ref")
}

func (f *fakeGitExt) LsTree(_ context.Context, _, _ string) (string, int, error) {
	return "", 0, nil
}

var _ adapters.GitExt = (*fakeGitExt)(nil)

func TestIsSSH(t *testing.T) {
	if !IsSSH("git@github.com:foo/bar") {
		t.Error("expected SSH true")
	}
	if IsSSH("https://github.com/foo/bar") {
		t.Error("expected SSH false for https")
	}
	if IsSSH("/local/path") {
		t.Error("expected SSH false for local")
	}
}

func TestRepoKeyHTTPSWithDotGit(t *testing.T) {
	got, err := RepoKey("/anywhere", "https://github.com/foo/bar.git")
	if err != nil {
		t.Fatal(err)
	}
	want := "github-com-foo-bar"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestRepoKeyHTTPSDeep(t *testing.T) {
	got, err := RepoKey("/anywhere", "https://example.org/team/sub/repo")
	if err != nil {
		t.Fatal(err)
	}
	want := "example-org-team-sub-repo"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestRepoKeySSH(t *testing.T) {
	got, err := RepoKey("/anywhere", "git@gitlab.com:org/repo.git")
	if err != nil {
		t.Fatal(err)
	}
	want := "gitlab-com-org-repo"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestRepoKeyLocalAbsolute(t *testing.T) {
	tmp := t.TempDir()
	got, err := RepoKey("/anywhere", tmp)
	if err != nil {
		t.Fatal(err)
	}
	want := "local-" + filepath.Base(tmp)
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestRepoKeyLocalRelative(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "sub-repo")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := RepoKey(tmp, "sub-repo")
	if err != nil {
		t.Fatal(err)
	}
	want := "local-sub-repo"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestRepoKeyFileURL(t *testing.T) {
	got, err := RepoKey("/anywhere", "file:///tmp/myrepo.git")
	if err != nil {
		t.Fatal(err)
	}
	want := "local-myrepo"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestRepoKeyEmptyError(t *testing.T) {
	if _, err := RepoKey("/anywhere", ""); err == nil {
		t.Error("expected error on empty spec")
	}
}

func TestRepoKeyUnrecognized(t *testing.T) {
	if _, err := RepoKey("/anywhere", "ftp://nope.example.com/x"); err == nil {
		t.Error("expected error on unrecognized scheme")
	}
}

func TestResolveCheckoutLocalPath(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	ext := &fakeGitExt{
		revParse: map[string]string{"HEAD": "deadbeef"},
		symbolicRef: map[string]string{
			"HEAD": "main",
		},
	}
	res, err := ResolveCheckout(context.Background(), tmp, ext, src, "local-src")
	if err != nil {
		t.Fatalf("ResolveCheckout: %v", err)
	}
	if res.Checkout != src {
		t.Errorf("Checkout = %q, want %q", res.Checkout, src)
	}
	if res.HeadSHA != "deadbeef" {
		t.Errorf("HeadSHA = %q", res.HeadSHA)
	}
	if res.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q", res.DefaultBranch)
	}
	if len(ext.cloneCalled) != 0 {
		t.Errorf("Clone should not be called for local path; got %v", ext.cloneCalled)
	}
}

func TestResolveCheckoutRemoteFreshClone(t *testing.T) {
	tmp := t.TempDir()
	ext := &fakeGitExt{
		revParse:    map[string]string{"HEAD": "abcdef"},
		symbolicRef: map[string]string{"HEAD": "main"},
	}
	res, err := ResolveCheckout(context.Background(), tmp, ext, "https://github.com/foo/bar.git", "github-com-foo-bar")
	if err != nil {
		t.Fatalf("ResolveCheckout: %v", err)
	}
	wantCheckout := filepath.Join(tmp, "raw", "_git-cache", "github-com-foo-bar")
	if res.Checkout != wantCheckout {
		t.Errorf("Checkout = %q, want %q", res.Checkout, wantCheckout)
	}
	if len(ext.cloneCalled) != 1 {
		t.Fatalf("Clone called %d times, want 1", len(ext.cloneCalled))
	}
	if ext.cloneCalled[0] != "https://github.com/foo/bar.git|"+wantCheckout {
		t.Errorf("Clone called with %q", ext.cloneCalled[0])
	}
}

func TestResolveCheckoutRemoteCloneFailure(t *testing.T) {
	tmp := t.TempDir()
	ext := &fakeGitExt{cloneErr: errors.New("boom"), cloneCode: 128}
	_, err := ResolveCheckout(context.Background(), tmp, ext, "https://github.com/foo/bar.git", "github-com-foo-bar")
	if err == nil {
		t.Fatal("expected clone failure error")
	}
	var re *ResolveError
	if !errors.As(err, &re) || re.Code != 10 {
		t.Errorf("expected ResolveError code 10, got %v", err)
	}
}

func TestResolveCheckoutDetachedHEADFallsBackToRemote(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	ext := &fakeGitExt{
		revParse: map[string]string{"HEAD": "abc"},
		symbolicRef: map[string]string{
			// HEAD lookup fails (detached) — symbolicRef map is missing
			// "HEAD" so SymbolicRef returns its error.
			"refs/remotes/origin/HEAD": "origin/develop",
		},
	}
	res, err := ResolveCheckout(context.Background(), tmp, ext, src, "local-src")
	if err != nil {
		t.Fatalf("ResolveCheckout: %v", err)
	}
	if res.DefaultBranch != "develop" {
		t.Errorf("DefaultBranch = %q, want 'develop'", res.DefaultBranch)
	}
}

func TestResolveCheckoutBranchFallsBackToMain(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	ext := &fakeGitExt{
		revParse:    map[string]string{"HEAD": "abc"},
		symbolicRef: map[string]string{},
	}
	res, err := ResolveCheckout(context.Background(), tmp, ext, src, "local-src")
	if err != nil {
		t.Fatalf("ResolveCheckout: %v", err)
	}
	if res.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want 'main'", res.DefaultBranch)
	}
}
