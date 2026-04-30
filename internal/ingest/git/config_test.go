package git

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fixtureRepoPath(t *testing.T, name string) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", "..", ".."))
	return filepath.Join(repoRoot, "tests", "fixtures", name)
}

func TestValidateName(t *testing.T) {
	good := []string{"foo", "foo-bar", "f1", "lib-2"}
	for _, n := range good {
		if err := ValidateName(n); err != nil {
			t.Errorf("ValidateName(%q) = %v; want nil", n, err)
		}
	}
	bad := []string{"", "Foo", "1foo", "foo_bar", "foo bar", "-x"}
	for _, n := range bad {
		if err := ValidateName(n); err == nil {
			t.Errorf("ValidateName(%q) = nil; want error", n)
		}
	}
}

func TestValidatePaths(t *testing.T) {
	if err := ValidatePaths([]string{"README.md", "docs/", "deep/nest"}); err != nil {
		t.Errorf("good paths: %v", err)
	}
	if err := ValidatePaths([]string{"../escape"}); err == nil {
		t.Error("expected error on '..' path")
	}
	if err := ValidatePaths([]string{"/abs/path"}); err == nil {
		t.Error("expected error on leading slash")
	}
}

func TestLoadRepoConfigGood(t *testing.T) {
	path := fixtureRepoPath(t, "git-sources-good.yml")
	cfg, err := LoadRepoConfig(path, "example-repo")
	if err != nil {
		t.Fatalf("LoadRepoConfig: %v", err)
	}
	if cfg.Name != "example-repo" {
		t.Errorf("Name = %q", cfg.Name)
	}
	if cfg.URL != "https://github.com/example/example.git" {
		t.Errorf("URL = %q", cfg.URL)
	}
	if !equalSlices(cfg.Paths, []string{"README.md", "docs/"}) {
		t.Errorf("Paths = %v", cfg.Paths)
	}
	if !equalSlices(cfg.Exclude, []string{"**/CHANGELOG.md"}) {
		t.Errorf("Exclude = %v", cfg.Exclude)
	}
	if cfg.Private {
		t.Error("Private should be false (https + private:false)")
	}
	if !cfg.PrivateExplicit {
		t.Error("PrivateExplicit should be true (private: false in yaml)")
	}
	if cfg.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q", cfg.DefaultBranch)
	}
}

func TestLoadRepoConfigSSHFlipsPrivate(t *testing.T) {
	path := fixtureRepoPath(t, "git-sources-good.yml")
	cfg, err := LoadRepoConfig(path, "private-runbooks")
	if err != nil {
		t.Fatalf("LoadRepoConfig: %v", err)
	}
	if !cfg.Private {
		t.Error("Private should be true (ssh + explicit private:true)")
	}
	if !cfg.PrivateExplicit {
		t.Error("PrivateExplicit should be true (private: true in yaml)")
	}
	if cfg.URL != "git@github.com:corp/runbooks.git" {
		t.Errorf("URL = %q", cfg.URL)
	}
}

func TestLoadRepoConfigUnknownRepo(t *testing.T) {
	path := fixtureRepoPath(t, "git-sources-good.yml")
	_, err := LoadRepoConfig(path, "missing")
	if err == nil {
		t.Fatal("expected error for missing repo, got nil")
	}
	if !strings.Contains(err.Error(), "no repo named") {
		t.Errorf("error %v missing 'no repo named'", err)
	}
}

func TestLoadRepoConfigMissingFile(t *testing.T) {
	tmp := t.TempDir()
	_, err := LoadRepoConfig(filepath.Join(tmp, "nope.yml"), "x")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "config not found") {
		t.Errorf("error %v missing 'config not found'", err)
	}
}

func TestLoadRepoConfigDefaultsPaths(t *testing.T) {
	tmp := t.TempDir()
	yamlPath := filepath.Join(tmp, "src.yml")
	if err := writeFile(yamlPath, `schema: 1
repos:
  - name: minimal
    url: https://example.com/foo/bar.git
`); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadRepoConfig(yamlPath, "minimal")
	if err != nil {
		t.Fatalf("LoadRepoConfig: %v", err)
	}
	want := []string{"README.md", "docs/", "rfcs/", "adr/"}
	if !equalSlices(cfg.Paths, want) {
		t.Errorf("default paths mismatch: got %v want %v", cfg.Paths, want)
	}
	if cfg.PrivateExplicit {
		t.Error("PrivateExplicit should be false when field is absent")
	}
}

func TestLoadRepoConfigBadSchema(t *testing.T) {
	tmp := t.TempDir()
	yamlPath := filepath.Join(tmp, "src.yml")
	if err := writeFile(yamlPath, `schema: 9
repos:
  - name: x
    url: u
`); err != nil {
		t.Fatal(err)
	}
	_, err := LoadRepoConfig(yamlPath, "x")
	if err == nil {
		t.Fatal("expected schema error")
	}
	if !strings.Contains(err.Error(), "schema") {
		t.Errorf("error %v missing 'schema'", err)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
