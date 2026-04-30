package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStatePathRejectsTraversal mirrors the bash
// `awiki_git_state_validate_repo_key` rejections at
// scripts/lib/git-state.sh:11-20.
func TestStatePathRejectsTraversal(t *testing.T) {
	cases := []string{
		"",
		"/abs",
		"foo/bar",
		"foo..bar",
		"..parent",
	}
	for _, c := range cases {
		if _, err := StatePath("/tmp", c); err == nil {
			t.Errorf("StatePath(%q) expected error, got nil", c)
		}
	}
}

func TestStatePathHappy(t *testing.T) {
	got, err := StatePath("/repo", "github-com-foo-bar")
	if err != nil {
		t.Fatalf("StatePath: %v", err)
	}
	want := filepath.Join("/repo", ".awiki", "git-state", "github-com-foo-bar.json")
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestLoadStateMissing(t *testing.T) {
	tmp := t.TempDir()
	s, ok, err := LoadState(tmp, "repo-x")
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if ok {
		t.Errorf("ok = true on missing file")
	}
	if s.Schema != 0 || s.Files != nil {
		t.Errorf("expected zero-value State, got %+v", s)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	want := State{
		Schema:        1,
		RepoKey:       "github-com-foo-bar",
		RepoName:      "bar",
		URL:           "https://github.com/foo/bar",
		DefaultBranch: "main",
		HeadSHA:       "abcdef0123456789",
		IngestedAt:    "2026-04-30T12:34:56Z",
		Private:       false,
		Files: map[string]FileEntry{
			"README.md": {
				BlobSHA:      "1111111111111111",
				Slug:         "git-bar-readme",
				OutPath:      "content/sources/git-bar-readme.md",
				LastIngested: "2026-04-30T12:34:56Z",
			},
		},
	}
	if err := SaveState(tmp, "github-com-foo-bar", want); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	got, ok, err := LoadState(tmp, "github-com-foo-bar")
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if !ok {
		t.Fatal("ok = false after SaveState")
	}
	if got.Schema != want.Schema ||
		got.RepoKey != want.RepoKey ||
		got.RepoName != want.RepoName ||
		got.URL != want.URL ||
		got.DefaultBranch != want.DefaultBranch ||
		got.HeadSHA != want.HeadSHA ||
		got.IngestedAt != want.IngestedAt ||
		got.Private != want.Private {
		t.Errorf("scalar mismatch:\n got %+v\nwant %+v", got, want)
	}
	if len(got.Files) != 1 || got.Files["README.md"] != want.Files["README.md"] {
		t.Errorf("Files mismatch:\n got %+v\nwant %+v", got.Files, want.Files)
	}
}

// TestSaveStateAppendsTrailingNewline asserts the on-disk byte form
// matches the bash `printf '%s\n' "$json"` form at git-state.sh:46.
func TestSaveStateAppendsTrailingNewline(t *testing.T) {
	tmp := t.TempDir()
	s := State{
		Schema:        1,
		RepoKey:       "k",
		RepoName:      "k",
		URL:           "u",
		DefaultBranch: "main",
		HeadSHA:       "h",
		IngestedAt:    "t",
		Private:       false,
		Files:         map[string]FileEntry{},
	}
	if err := SaveState(tmp, "k", s); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, ".awiki", "git-state", "k.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Errorf("on-disk file does not end with newline: %q", data)
	}
	if !strings.HasPrefix(string(data), "{\n  \"schema\":") {
		t.Errorf("on-disk JSON does not look 2-space indented: %q", data)
	}
}

// TestSaveStateAtomicLeavesNoTemp asserts that on success the tmp file
// is renamed away, mirroring the bash `mv -f tmp path`.
func TestSaveStateAtomicLeavesNoTemp(t *testing.T) {
	tmp := t.TempDir()
	s := State{
		Schema:        1,
		RepoKey:       "k",
		RepoName:      "k",
		URL:           "u",
		DefaultBranch: "main",
		HeadSHA:       "h",
		IngestedAt:    "t",
		Private:       false,
		Files:         map[string]FileEntry{},
	}
	if err := SaveState(tmp, "k", s); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(tmp, ".awiki", "git-state"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp.") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}

func TestValidateState(t *testing.T) {
	good := State{
		Schema:        1,
		RepoKey:       "k",
		RepoName:      "k",
		URL:           "u",
		DefaultBranch: "m",
		HeadSHA:       "h",
		IngestedAt:    "t",
		Private:       false,
		Files:         map[string]FileEntry{},
	}
	if err := ValidateState(good); err != nil {
		t.Errorf("ValidateState(good) returned %v", err)
	}

	bad := good
	bad.Schema = 0
	if err := ValidateState(bad); err == nil {
		t.Errorf("ValidateState(schema=0) returned nil")
	}

	bad = good
	bad.Files = nil
	if err := ValidateState(bad); err == nil {
		t.Errorf("ValidateState(no files) returned nil")
	}
}
