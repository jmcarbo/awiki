package testutil

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"awiki/internal/adapters"
)

// FakeGitExt is a test double for adapters.GitExt. It does not shell
// `git`; instead it simulates clone/fetch/rev-parse/symbolic-ref/
// ls-tree against caller-supplied canned data so the slice 8 ingest-git
// driver can be exercised without git on PATH.
//
// Typical wiring:
//
//	ext := &testutil.FakeGitExt{
//	  CloneStage: tmpDir, // clone copies this tree into the dest path
//	  Refs: map[string]string{
//	    "HEAD":     "abcdef0123456789",
//	    "HEAD:README.md": "blob1",
//	  },
//	  SymbolicRefs: map[string]string{"HEAD": "main"},
//	  LsTreeOutput: "README.md\ndocs/intro.md\n",
//	}
//
// Call mode for each operation:
//   - Clone copies CloneStage (when set) recursively into dest +
//     creates a `.git` placeholder so subsequent ResolveCheckout calls
//     see an existing checkout.
//   - Fetch / Reset return (FetchCode/FetchErr) and (ResetCode/ResetErr).
//   - RevParse looks up Refs[ref]; missing key returns "" and exit 0
//     (matches `git rev-parse missing` short-circuit in tests where
//     specific tree paths may not exist yet).
//   - SymbolicRef looks up SymbolicRefs[name]; missing key returns
//     ("", 1, errors.New("not a symbolic ref")).
//   - LsTree returns LsTreeOutput verbatim.
type FakeGitExt struct {
	CloneStage   string // src tree to copy on Clone
	CloneErr     error
	CloneCode    int
	FetchErr     error
	FetchCode    int
	ResetErr     error
	ResetCode    int
	InitErr      error
	InitCode     int
	Refs         map[string]string
	SymbolicRefs map[string]string
	LsTreeOutput string

	// Callers captured for assertions.
	CloneCalls       []string // "url|dest"
	FetchCalls       []string
	ResetCalls       []string
	SymbolicRefCalls []string
	RevParseCalls    []string
	LsTreeCalls      []string
}

func (f *FakeGitExt) Clone(_ context.Context, url, dest string) (int, error) {
	f.CloneCalls = append(f.CloneCalls, url+"|"+dest)
	if f.CloneErr != nil || f.CloneCode != 0 {
		return f.CloneCode, f.CloneErr
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return 1, err
	}
	if f.CloneStage != "" {
		if err := copyTreeForFake(f.CloneStage, dest); err != nil {
			return 1, err
		}
	}
	if err := os.MkdirAll(filepath.Join(dest, ".git"), 0o755); err != nil {
		return 1, err
	}
	return 0, nil
}

func (f *FakeGitExt) RevParse(_ context.Context, _, ref string) (string, int, error) {
	f.RevParseCalls = append(f.RevParseCalls, ref)
	if v, ok := f.Refs[ref]; ok {
		return v, 0, nil
	}
	return "", 0, nil
}

func (f *FakeGitExt) Init(_ context.Context, _ string) (int, error) {
	if f.InitErr != nil || f.InitCode != 0 {
		return f.InitCode, f.InitErr
	}
	return 0, nil
}

func (f *FakeGitExt) Fetch(_ context.Context, dir, remote string) (int, error) {
	f.FetchCalls = append(f.FetchCalls, dir+"|"+remote)
	return f.FetchCode, f.FetchErr
}

func (f *FakeGitExt) Reset(_ context.Context, dir, ref string) (int, error) {
	f.ResetCalls = append(f.ResetCalls, dir+"|"+ref)
	return f.ResetCode, f.ResetErr
}

func (f *FakeGitExt) SymbolicRef(_ context.Context, _, name string) (string, int, error) {
	f.SymbolicRefCalls = append(f.SymbolicRefCalls, name)
	if v, ok := f.SymbolicRefs[name]; ok {
		return v, 0, nil
	}
	return "", 1, errors.New("not a symbolic ref")
}

func (f *FakeGitExt) LsTree(_ context.Context, dir, ref string) (string, int, error) {
	f.LsTreeCalls = append(f.LsTreeCalls, dir+"|"+ref)
	return f.LsTreeOutput, 0, nil
}

// copyTreeForFake duplicates a directory tree. Used by Clone to stage
// canned upstream content under a fake checkout.
func copyTreeForFake(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

// Compile-time interface check.
var _ adapters.GitExt = (*FakeGitExt)(nil)
