package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTemplate_NoVerb(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run([]string{"template"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("code=%d want 2", code)
	}
}

func TestTemplate_UnknownVerb(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "nope"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("code=%d want 2", code)
	}
	if !strings.Contains(errBuf.String(), "unknown verb") {
		t.Errorf("stderr=%q", errBuf.String())
	}
}

func TestTemplate_StatusNoProvenance(t *testing.T) {
	root := t.TempDir()
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "status", "--root", root}, &out, &errBuf)
	if code != 1 {
		t.Errorf("code=%d want 1", code)
	}
	if !strings.Contains(errBuf.String(), "no .awiki/template.json") {
		t.Errorf("stderr=%q", errBuf.String())
	}
}

func TestTemplate_StatusBasic(t *testing.T) {
	root := t.TempDir()
	pj := filepath.Join(root, ".awiki", "template.json")
	writeFile(t, pj, `{
  "schema_version": 1,
  "repo": "https://x/y.git",
  "original_repo": "https://x/y.git",
  "ref": "main",
  "version": "0.2.0",
  "commit": "abc",
  "applied_migrations": [],
  "deleted": [],
  "bootstrap_steps_done": []
}`)
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "status", "--root", root}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "version: 0.2.0") {
		t.Errorf("stdout=%q", out.String())
	}
	if !strings.Contains(out.String(), "commit:  abc") {
		t.Errorf("stdout=%q", out.String())
	}
}

func TestTemplate_StatusSourceChange(t *testing.T) {
	root := t.TempDir()
	pj := filepath.Join(root, ".awiki", "template.json")
	writeFile(t, pj, `{
  "schema_version": 1,
  "repo": "https://x/y.git",
  "original_repo": "https://x/old.git",
  "ref": "main",
  "version": "0.2.0",
  "commit": "abc",
  "applied_migrations": [],
  "deleted": [],
  "bootstrap_steps_done": []
}`)
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "status", "--root", root}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(out.String(), "DIFFERS") {
		t.Errorf("stdout=%q", out.String())
	}
}

func TestTemplate_GC(t *testing.T) {
	root := t.TempDir()
	pj := filepath.Join(root, ".awiki", "template.json")
	writeFile(t, pj, `{
  "schema_version": 1,
  "repo": "https://x/y.git",
  "original_repo": "https://x/y.git",
  "ref": "main",
  "version": "0.2.0",
  "commit": "abc",
  "applied_migrations": [],
  "deleted": [],
  "bootstrap_steps_done": []
}`)
	cacheDir := filepath.Join(root, ".awiki", "template-cache")
	_ = os.MkdirAll(filepath.Join(cacheDir, "abc"), 0o755)
	_ = os.MkdirAll(filepath.Join(cacheDir, "old1"), 0o755)
	_ = os.MkdirAll(filepath.Join(cacheDir, "old2"), 0o755)
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "gc", "--root", root}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "cache GC complete") {
		t.Errorf("stdout=%q", out.String())
	}
	// Now we expect 2 dirs left: abc + most-recent of old1/old2
	entries, _ := os.ReadDir(cacheDir)
	if len(entries) != 2 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("entries=%v want 2", names)
	}
}

func TestTemplate_SourceCheck_Match(t *testing.T) {
	root := t.TempDir()
	pj := filepath.Join(root, ".awiki", "template.json")
	writeFile(t, pj, `{
  "schema_version": 1,
  "repo": "https://x/y.git",
  "original_repo": "https://x/y.git",
  "ref": "main",
  "version": "0.2.0",
  "commit": "abc",
  "applied_migrations": [],
  "deleted": [],
  "bootstrap_steps_done": []
}`)
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "source-check", "--provenance", pj, "--source", "https://x/y.git"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("code=%d stderr=%q", code, errBuf.String())
	}
}

func TestTemplate_SourceCheck_Mismatch_Halts(t *testing.T) {
	root := t.TempDir()
	pj := filepath.Join(root, ".awiki", "template.json")
	writeFile(t, pj, `{
  "schema_version": 1,
  "repo": "https://x/y.git",
  "original_repo": "https://x/y.git",
  "ref": "main",
  "version": "0.2.0",
  "commit": "abc",
  "applied_migrations": [],
  "deleted": [],
  "bootstrap_steps_done": []
}`)
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "source-check", "--provenance", pj, "--source", "https://other/z.git"}, &out, &errBuf)
	if code != 1 {
		t.Errorf("code=%d want 1", code)
	}
	if !strings.Contains(errBuf.String(), "Source change detected") {
		t.Errorf("stderr=%q", errBuf.String())
	}
}

func TestTemplate_SourceCheck_Mismatch_Accepted(t *testing.T) {
	root := t.TempDir()
	pj := filepath.Join(root, ".awiki", "template.json")
	writeFile(t, pj, `{
  "schema_version": 1,
  "repo": "https://x/y.git",
  "original_repo": "https://x/y.git",
  "ref": "main",
  "version": "0.2.0",
  "commit": "abc",
  "applied_migrations": [],
  "deleted": [],
  "bootstrap_steps_done": []
}`)
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "source-check", "--provenance", pj, "--source", "https://other/z.git", "--accept-source-change"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("code=%d stderr=%q", code, errBuf.String())
	}
}

func TestTemplate_Manifest_Load(t *testing.T) {
	root := t.TempDir()
	mp := filepath.Join(root, "template.manifest.toml")
	writeFile(t, mp, `schema_version = 1
template_version = "0.1.0"
[strategies]
overwrite = []
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = ["dep-check", "domain"]
[bootstrap.dangerous]
ids = ["domain"]
`)
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "manifest", "load", mp}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errBuf.String())
	}
	want := "schema_version=1\ntemplate_version=0.1.0\nnew_file_default=prompt\n"
	if out.String() != want {
		t.Errorf("stdout=%q want %q", out.String(), want)
	}
}

func TestTemplate_Manifest_BootstrapIDs(t *testing.T) {
	root := t.TempDir()
	mp := filepath.Join(root, "template.manifest.toml")
	writeFile(t, mp, `schema_version = 1
template_version = "0.1.0"
[strategies]
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = ["dep-check", "domain"]
[bootstrap.dangerous]
ids = []
`)
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "manifest", "bootstrap-ids", mp}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if out.String() != "dep-check\ndomain\n" {
		t.Errorf("stdout=%q", out.String())
	}
}

func TestTemplate_Manifest_Resolve(t *testing.T) {
	root := t.TempDir()
	mp := filepath.Join(root, "template.manifest.toml")
	writeFile(t, mp, `schema_version = 1
template_version = "0.1.0"
[strategies]
overwrite = ["scripts/**"]
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = []
[bootstrap.dangerous]
ids = []
`)
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "manifest", "resolve", mp, "scripts/foo.sh"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if strings.TrimSpace(out.String()) != "overwrite" {
		t.Errorf("stdout=%q", out.String())
	}
}

func TestTemplate_Provenance_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	pj := filepath.Join(dir, "template.json")
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "provenance", "init", pj, "https://x/y.git", "main", "0.1.0", "abcdef"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("init: code=%d stderr=%q", code, errBuf.String())
	}
	out.Reset()
	errBuf.Reset()
	code = Run([]string{"template", "provenance", "get", pj, "version"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("get: code=%d", code)
	}
	if strings.TrimSpace(out.String()) != "0.1.0" {
		t.Errorf("get version=%q", out.String())
	}
	out.Reset()
	errBuf.Reset()
	code = Run([]string{"template", "provenance", "set", pj, "original_repo", "https://other/z.git"}, &out, &errBuf)
	if code != 1 {
		t.Errorf("set immutable: code=%d want 1", code)
	}
	if !strings.Contains(errBuf.String(), "field is immutable") {
		t.Errorf("stderr=%q", errBuf.String())
	}
}

func TestTemplate_Config_Get(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	writeFile(t, cfg, "# comment\ndefault_branch=main\nno_template_check=true\n")
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "config", "get", cfg, "default_branch", "fallback"}, &out, &errBuf)
	if code != 0 || strings.TrimSpace(out.String()) != "main" {
		t.Errorf("code=%d stdout=%q", code, out.String())
	}
}

func TestTemplate_Config_GetMissingUsesDefault(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	writeFile(t, cfg, "default_branch=main\n")
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "config", "get", cfg, "no_template_check", "false"}, &out, &errBuf)
	if code != 0 || strings.TrimSpace(out.String()) != "false" {
		t.Errorf("code=%d stdout=%q", code, out.String())
	}
}

func TestTemplate_Config_GetMissingFile(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "config", "get", "/no/such/file", "key", "default"}, &out, &errBuf)
	if code != 0 || strings.TrimSpace(out.String()) != "default" {
		t.Errorf("code=%d stdout=%q", code, out.String())
	}
}

func TestTemplate_Config_Validate(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	writeFile(t, cfg, "default_branch=main\nbad line here\n")
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "config", "validate", cfg}, &out, &errBuf)
	if code != 1 {
		t.Errorf("code=%d want 1", code)
	}
	if !strings.Contains(errBuf.String(), "malformed line") {
		t.Errorf("stderr=%q", errBuf.String())
	}
}

func TestTemplate_Config_Validate_UnknownKeyWarns(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	writeFile(t, cfg, "default_branch=main\nrandom_key=value\n")
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "config", "validate", cfg}, &out, &errBuf)
	if code != 0 {
		t.Errorf("code=%d want 0 (warning only)", code)
	}
	if !strings.Contains(errBuf.String(), "unknown key random_key") {
		t.Errorf("stderr=%q", errBuf.String())
	}
}

func TestTemplate_Escape(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "escape", "a|b\nc%d"}, &out, &errBuf)
	if code != 0 {
		t.Errorf("code=%d", code)
	}
	if strings.TrimSpace(out.String()) != "a%7Cb%0Ac%25d" {
		t.Errorf("stdout=%q", out.String())
	}
}

func TestBootstrapStep_NoArgs(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run([]string{"bootstrap-step"}, &out, &errBuf)
	if code != 2 {
		t.Errorf("code=%d want 2", code)
	}
	if !strings.Contains(errBuf.String(), "usage: awiki bootstrap-step") {
		t.Errorf("stderr=%q", errBuf.String())
	}
}

func TestBootstrapStep_HaltsOnPendingPrompts(t *testing.T) {
	root := t.TempDir()
	pp := filepath.Join(root, ".awiki", "pending-prompts")
	_ = os.MkdirAll(pp, 0o755)
	_ = os.WriteFile(filepath.Join(pp, "x.md"), []byte("hi"), 0o644)
	t.Setenv("AWIKI_REPO_ROOT", root)
	var out, errBuf bytes.Buffer
	code := Run([]string{"bootstrap-step", "--non-interactive", "domain"}, &out, &errBuf)
	if code != 1 {
		t.Errorf("code=%d want 1", code)
	}
	if !strings.Contains(errBuf.String(), "pending-prompts present") {
		t.Errorf("stderr=%q", errBuf.String())
	}
}

func TestTemplate_Lint_MissingManifest(t *testing.T) {
	root := t.TempDir()
	var out, errBuf bytes.Buffer
	code := Run([]string{"template", "lint", "--root", root}, &out, &errBuf)
	if code != 2 {
		t.Errorf("code=%d want 2", code)
	}
	if !strings.Contains(out.String(), "LINT|error|template.manifest.toml|missing") {
		t.Errorf("stdout=%q", out.String())
	}
}
