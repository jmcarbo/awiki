package template

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeBSGit struct {
	hasBranch  bool
	checkouts  []string
	newBranch  string
	deleted    []string
	added      bool
	clean      bool
	commitMsgs []string
}

func (f *fakeBSGit) HasUpdateBranch() (bool, error) { return f.hasBranch, nil }
func (f *fakeBSGit) Checkout(ref string) error {
	f.checkouts = append(f.checkouts, ref)
	return nil
}
func (f *fakeBSGit) CheckoutNewBranch(b string) error {
	f.newBranch = b
	return nil
}
func (f *fakeBSGit) DeleteBranch(b string) error {
	f.deleted = append(f.deleted, b)
	return nil
}
func (f *fakeBSGit) AddAll() error                  { f.added = true; return nil }
func (f *fakeBSGit) DiffCachedQuiet() (bool, error) { return f.clean, nil }
func (f *fakeBSGit) Commit(msg string) error {
	f.commitMsgs = append(f.commitMsgs, msg)
	return nil
}

type fakeBSBash struct {
	calls []string
}

func (f *fakeBSBash) RunBashScript(script string, env []string) (int, error) {
	data, err := os.ReadFile(script)
	if err == nil {
		f.calls = append(f.calls, string(data))
	}
	_ = env
	return 0, nil
}

type fakeBSConfirm struct {
	answer bool
}

func (f *fakeBSConfirm) Confirm(prompt string) (bool, error) { return f.answer, nil }

func writeProvenanceForRerun(t *testing.T, dir, stepID, oldHash string) string {
	t.Helper()
	pj := filepath.Join(dir, ".awiki", "template.json")
	if err := os.MkdirAll(filepath.Dir(pj), 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{
  "schema_version": 1,
  "repo": "https://x/y.git",
  "original_repo": "https://x/y.git",
  "ref": "main",
  "version": "0.1.0",
  "commit": "abc",
  "applied_migrations": [],
  "deleted": [],
  "bootstrap_steps_done": [{"id": %q, "status": "applied", "content_hash": %q}]
}
`, stepID, oldHash)
	if err := os.WriteFile(pj, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return pj
}

func TestRerunBootstrapStep_HappyPath_NonInteractive(t *testing.T) {
	repo := t.TempDir()
	bs := filepath.Join(repo, "BOOTSTRAP.md")
	if err := os.WriteFile(bs, []byte(fixtureBootstrapMD), 0o644); err != nil {
		t.Fatal(err)
	}
	pj := writeProvenanceForRerun(t, repo, "domain", "sha256:OLD")

	g := &fakeBSGit{clean: false}
	b := &fakeBSBash{}

	res, err := RerunBootstrapStep(g, b, nil, BootstrapStepInput{
		RepoRoot:       repo,
		StepID:         "domain",
		NonInteractive: true,
		Now:            func() int64 { return 12345 },
	})
	if err != nil {
		t.Fatalf("RerunBootstrapStep: %v", err)
	}
	if res.Branch != "awiki-template-update/rerun-domain-12345" {
		t.Errorf("branch=%q", res.Branch)
	}
	if g.newBranch != res.Branch {
		t.Errorf("newBranch=%q want %q", g.newBranch, res.Branch)
	}
	if !strings.Contains(res.StepBody, "personal | research") {
		t.Errorf("body=%q missing expected", res.StepBody)
	}
	if len(b.calls) != 1 {
		t.Fatalf("bash calls=%d", len(b.calls))
	}
	if !strings.Contains(b.calls[0], "personal | research") {
		t.Errorf("bash invocation body did not contain step body")
	}
	if !strings.HasPrefix(res.NewHash, "sha256:") {
		t.Errorf("hash=%q", res.NewHash)
	}
	if !g.added {
		t.Errorf("git add -A not called")
	}
	if len(g.commitMsgs) != 1 || !strings.Contains(g.commitMsgs[0], "re-run bootstrap step domain") {
		t.Errorf("commitMsgs=%v", g.commitMsgs)
	}

	// template.json should now have the new hash.
	data, err := os.ReadFile(pj)
	if err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err := json.Unmarshal(data, &d); err != nil {
		t.Fatal(err)
	}
	steps, _ := d["bootstrap_steps_done"].([]any)
	if len(steps) != 1 {
		t.Fatalf("steps=%v", steps)
	}
	s := steps[0].(map[string]any)
	if s["content_hash"].(string) == "sha256:OLD" {
		t.Errorf("hash not updated")
	}
}

func TestRerunBootstrapStep_DeclinesCleansUp(t *testing.T) {
	repo := t.TempDir()
	bs := filepath.Join(repo, "BOOTSTRAP.md")
	if err := os.WriteFile(bs, []byte(fixtureBootstrapMD), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = writeProvenanceForRerun(t, repo, "domain", "sha256:OLD")

	g := &fakeBSGit{}
	b := &fakeBSBash{}
	c := &fakeBSConfirm{answer: false}

	res, err := RerunBootstrapStep(g, b, c, BootstrapStepInput{
		RepoRoot:      repo,
		StepID:        "domain",
		DefaultBranch: "main",
		Now:           func() int64 { return 1 },
	})
	if err != nil {
		t.Fatalf("RerunBootstrapStep: %v", err)
	}
	if !res.Declined {
		t.Errorf("expected Declined=true")
	}
	if res.Branch != "" {
		t.Errorf("Branch=%q want empty", res.Branch)
	}
	if len(g.checkouts) != 1 || g.checkouts[0] != "main" {
		t.Errorf("checkouts=%v", g.checkouts)
	}
	if len(g.deleted) != 1 || g.deleted[0] != "awiki-template-update/rerun-domain-1" {
		t.Errorf("deleted=%v", g.deleted)
	}
	if len(b.calls) != 0 {
		t.Errorf("bash should not have run when declined")
	}
}

func TestRerunBootstrapStep_HaltsOnUpdateInProgress(t *testing.T) {
	repo := t.TempDir()
	state := filepath.Join(repo, ".awiki", "template-cache", "_fetch", ".update-state.json")
	if err := os.MkdirAll(filepath.Dir(state), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := RerunBootstrapStep(&fakeBSGit{}, &fakeBSBash{}, nil, BootstrapStepInput{
		RepoRoot: repo, StepID: "domain", NonInteractive: true,
	})
	if !errors.Is(err, ErrUpdateInProgress) {
		t.Errorf("err=%v want ErrUpdateInProgress", err)
	}
}

func TestRerunBootstrapStep_HaltsOnPendingPrompts(t *testing.T) {
	repo := t.TempDir()
	pp := filepath.Join(repo, ".awiki", "pending-prompts")
	if err := os.MkdirAll(pp, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pp, "x.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := RerunBootstrapStep(&fakeBSGit{}, &fakeBSBash{}, nil, BootstrapStepInput{
		RepoRoot: repo, StepID: "domain", NonInteractive: true,
	})
	if !errors.Is(err, ErrPendingPrompts) {
		t.Errorf("err=%v want ErrPendingPrompts", err)
	}
}

func TestRerunBootstrapStep_HaltsOnUpdateBranch(t *testing.T) {
	repo := t.TempDir()
	bs := filepath.Join(repo, "BOOTSTRAP.md")
	_ = os.WriteFile(bs, []byte(fixtureBootstrapMD), 0o644)

	g := &fakeBSGit{hasBranch: true}
	_, err := RerunBootstrapStep(g, &fakeBSBash{}, nil, BootstrapStepInput{
		RepoRoot: repo, StepID: "domain", NonInteractive: true,
	})
	if !errors.Is(err, ErrUpdateBranchPresent) {
		t.Errorf("err=%v want ErrUpdateBranchPresent", err)
	}
}

func TestRerunBootstrapStep_MissingBootstrapMD(t *testing.T) {
	repo := t.TempDir()
	_, err := RerunBootstrapStep(&fakeBSGit{}, &fakeBSBash{}, nil, BootstrapStepInput{
		RepoRoot: repo, StepID: "domain", NonInteractive: true,
	})
	if err == nil || !strings.Contains(err.Error(), "BOOTSTRAP.md not found") {
		t.Errorf("err=%v want BOOTSTRAP.md not found", err)
	}
}

func TestRerunBootstrapStep_MissingStepID(t *testing.T) {
	repo := t.TempDir()
	bs := filepath.Join(repo, "BOOTSTRAP.md")
	_ = os.WriteFile(bs, []byte(fixtureBootstrapMD), 0o644)
	_, err := RerunBootstrapStep(&fakeBSGit{}, &fakeBSBash{}, nil, BootstrapStepInput{
		RepoRoot: repo, StepID: "nonexistent", NonInteractive: true,
	})
	if !errors.Is(err, ErrStepNotFound) {
		t.Errorf("err=%v want ErrStepNotFound", err)
	}
}

// Golden parity: the body returned from RerunBootstrapStep must match
// what the bash oracle (bootstrap_replay.py body) prints byte-for-byte.
func TestRerunBootstrapStep_BodyMatchesPythonByteForByte(t *testing.T) {
	// Use the v0 fixture (same on disk).
	v0 := "../../tests/fixtures/template-update/v0/BOOTSTRAP.md"
	data, err := os.ReadFile(v0)
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	bodies := ParseBootstrapBodies(string(data))
	domain, ok := bodies["domain"]
	if !ok {
		t.Fatal("domain not in fixture")
	}
	repo := t.TempDir()
	bs := filepath.Join(repo, "BOOTSTRAP.md")
	if err := os.WriteFile(bs, data, 0o644); err != nil {
		t.Fatal(err)
	}
	_ = writeProvenanceForRerun(t, repo, "domain", "sha256:OLD")
	g := &fakeBSGit{}
	b := &fakeBSBash{}
	res, err := RerunBootstrapStep(g, b, nil, BootstrapStepInput{
		RepoRoot:       repo,
		StepID:         "domain",
		NonInteractive: true,
		Now:            func() int64 { return 1 },
	})
	if err != nil {
		t.Fatalf("RerunBootstrapStep: %v", err)
	}
	if res.StepBody != domain {
		t.Errorf("body mismatch:\n  go=%q\n  py=%q", res.StepBody, domain)
	}
}
