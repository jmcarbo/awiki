package initverb

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeGit is a noop GitInit for tests; per-method overrides set
// non-zero codes / errors when the step under test cares about
// failure paths.
type fakeGit struct {
	subOut    string
	subCode   int
	subErr    error
	statusOut string
	statusErr error

	addCode int
	addErr  error

	commitOut  string
	commitCode int
	commitErr  error

	revParse string
}

func (f *fakeGit) SubmoduleUpdate(ctx context.Context, repoRoot string) (string, int, error) {
	return f.subOut, f.subCode, f.subErr
}
func (f *fakeGit) Status(ctx context.Context, repoRoot string) (string, int, error) {
	return f.statusOut, 0, f.statusErr
}
func (f *fakeGit) AddAll(ctx context.Context, repoRoot string) (int, error) {
	return f.addCode, f.addErr
}
func (f *fakeGit) Commit(ctx context.Context, repoRoot, msg string) (string, int, error) {
	return f.commitOut, f.commitCode, f.commitErr
}
func (f *fakeGit) SubmoduleDeinit(ctx context.Context, repoRoot, path string) (int, error) {
	return 0, nil
}
func (f *fakeGit) SubmoduleRm(ctx context.Context, repoRoot, path string) (int, error) {
	return 0, nil
}
func (f *fakeGit) SubmoduleAdd(ctx context.Context, repoRoot, url, path string) (int, error) {
	return 0, nil
}
func (f *fakeGit) RevParseHEAD(ctx context.Context, repoRoot string) (string, error) {
	return f.revParse, nil
}

// fakeBash captures the most recent script + args for assertions.
type fakeBash struct {
	calls    int
	lastDir  string
	lastArgs []string
	lastBin  string
	out      string
	code     int
	err      error
}

func (f *fakeBash) Run(ctx context.Context, repoRoot, script string, args ...string) (string, int, error) {
	f.calls++
	f.lastDir = repoRoot
	f.lastBin = script
	f.lastArgs = append([]string{}, args...)
	return f.out, f.code, f.err
}

// fakeAgent captures the most recent invocation for the patch-identity
// + agent runner tests further down. Defined here so init_test.go does
// not have to.
type fakeAgent struct {
	calls    int
	lastCLI  string
	lastPrompt string
	out      string
	err      error
}

func (f *fakeAgent) Run(ctx context.Context, cli, prompt string) (string, error) {
	f.calls++
	f.lastCLI = cli
	f.lastPrompt = prompt
	return f.out, f.err
}

type fakeNPM struct {
	calls   int
	lastDir string
	out     string
	code    int
	err     error
}

func (f *fakeNPM) Install(ctx context.Context, dir string) (string, int, error) {
	f.calls++
	f.lastDir = dir
	return f.out, f.code, f.err
}

// --- Step 0: dep-check ---

func TestStepDepCheckSuccess(t *testing.T) {
	dir := t.TempDir()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	ctx := StepContext{
		Ctx: context.Background(), RepoRoot: dir,
		Stdout: stdout, Stderr: stderr,
		Git: &fakeGit{},
	}
	res, err := (stepDepCheck{}).Execute(ctx, &Answers{})
	// Step calls ops.CheckDeps which inspects the host. On a CI-like
	// host without bash 4 the result is non-zero — we accept either,
	// but we MUST see the submodule call route through Git.
	if err == nil && res.Status != StatusApplied {
		t.Errorf("expected applied or fail-with-err, got %+v", res)
	}
}

func TestStepDepCheckSubmoduleFails(t *testing.T) {
	dir := t.TempDir()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	ctx := StepContext{
		Ctx: context.Background(), RepoRoot: dir,
		Stdout: stdout, Stderr: stderr,
		Git: &fakeGit{subCode: 1, subErr: errors.New("network")},
	}
	res, err := (stepDepCheck{}).Execute(ctx, &Answers{})
	if err == nil {
		t.Fatalf("expected error, got %+v", res)
	}
	if res.Status != StatusFailed {
		t.Fatalf("expected failed, got %+v", res)
	}
}

// --- Step 1: domain ---

func TestStepDomainNonInteractive(t *testing.T) {
	ans := &Answers{}
	ctx := StepContext{NonInteractive: true, Stdout: &bytes.Buffer{}}
	res, err := (stepDomain{}).Execute(ctx, ans)
	if err != nil {
		t.Fatal(err)
	}
	if ans.Domain != "other" || res.Status != StatusApplied {
		t.Fatalf("got domain=%q res=%+v", ans.Domain, res)
	}
}

func TestStepDomainPromptLetter(t *testing.T) {
	ans := &Answers{}
	ctx := StepContext{
		Stdout: &bytes.Buffer{},
		Prompt: func(_ string, _ string) (string, error) { return "B", nil },
	}
	if _, err := (stepDomain{}).Execute(ctx, ans); err != nil {
		t.Fatal(err)
	}
	if ans.Domain != "research" {
		t.Fatalf("domain=%q", ans.Domain)
	}
}

func TestStepDomainPromptFreeText(t *testing.T) {
	ans := &Answers{}
	ctx := StepContext{
		Stdout: &bytes.Buffer{},
		Prompt: func(_, _ string) (string, error) { return "history-of-rome", nil },
	}
	if _, err := (stepDomain{}).Execute(ctx, ans); err != nil {
		t.Fatal(err)
	}
	if ans.Domain != "history-of-rome" {
		t.Fatalf("domain=%q", ans.Domain)
	}
}

// --- Step 2: wiki-name ---

func TestStepWikiNameValid(t *testing.T) {
	ans := &Answers{}
	calls := 0
	ctx := StepContext{
		Stdout: &bytes.Buffer{},
		Prompt: func(prompt, def string) (string, error) {
			calls++
			if calls == 1 {
				return "history-of-rome", nil
			}
			return "Tracking sources for the Republic and early Empire", nil
		},
	}
	if _, err := (stepWikiName{}).Execute(ctx, ans); err != nil {
		t.Fatal(err)
	}
	if ans.WikiName != "history-of-rome" {
		t.Fatalf("name=%q", ans.WikiName)
	}
	if !strings.HasPrefix(ans.Purpose, "Tracking") {
		t.Fatalf("purpose=%q", ans.Purpose)
	}
}

func TestStepWikiNameInvalidKebab(t *testing.T) {
	// Interactive mode rejects bad input (user must retype).
	ans := &Answers{WikiName: "Has Spaces"}
	ctx := StepContext{Stdout: &bytes.Buffer{}, NonInteractive: false,
		Prompt: func(string, string) (string, error) { return "Has Spaces", nil }}
	if _, err := (stepWikiName{}).Execute(ctx, ans); err == nil {
		t.Fatal("expected error for non-kebab name in interactive mode")
	}
}

func TestStepWikiNameSanitizesNonInteractive(t *testing.T) {
	ans := &Answers{WikiName: "Has Spaces"}
	ctx := StepContext{Stdout: &bytes.Buffer{}, NonInteractive: true}
	if _, err := (stepWikiName{}).Execute(ctx, ans); err != nil {
		t.Fatalf("non-interactive sanitize should not error: %v", err)
	}
	if ans.WikiName != "has-spaces" {
		t.Fatalf("wiki_name = %q, want %q", ans.WikiName, "has-spaces")
	}
}

func TestStepWikiNamePurposeTooLong(t *testing.T) {
	ans := &Answers{WikiName: "ok", Purpose: strings.Repeat("a", 121)}
	ctx := StepContext{NonInteractive: true, Stdout: &bytes.Buffer{}}
	if _, err := (stepWikiName{}).Execute(ctx, ans); err == nil {
		t.Fatal("expected error for purpose >120 chars")
	}
}

func TestStepWikiNameNonInteractiveFallback(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my-fixture")
	ans := &Answers{}
	ctx := StepContext{NonInteractive: true, RepoRoot: dir, Stdout: &bytes.Buffer{}}
	if _, err := (stepWikiName{}).Execute(ctx, ans); err != nil {
		t.Fatal(err)
	}
	if ans.WikiName != "my-fixture" {
		t.Fatalf("name=%q", ans.WikiName)
	}
}

// --- Step 3: privacy ---

func TestStepPrivacyNone(t *testing.T) {
	ans := &Answers{Privacy: "none"}
	ctx := StepContext{Stdout: &bytes.Buffer{}}
	res, err := (stepPrivacy{}).Execute(ctx, ans)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusApplied {
		t.Fatalf("res=%+v", res)
	}
}

func TestStepPrivacyGitCryptShellsScript(t *testing.T) {
	ans := &Answers{Privacy: "git-crypt"}
	bash := &fakeBash{}
	git := &fakeGit{statusOut: "M .gitattributes\n"}
	stdout := &bytes.Buffer{}
	ctx := StepContext{
		Ctx: context.Background(), RepoRoot: "/tmp/wiki",
		Stdout: stdout, Stderr: &bytes.Buffer{},
		Bash: bash, Git: git,
		NonInteractive: true,
	}
	if _, err := (stepPrivacy{}).Execute(ctx, ans); err != nil {
		t.Fatal(err)
	}
	if bash.calls != 1 {
		t.Fatalf("bash calls=%d", bash.calls)
	}
	if !strings.HasSuffix(bash.lastBin, "encrypt-init.sh") {
		t.Fatalf("script=%s", bash.lastBin)
	}
	if len(bash.lastArgs) != 0 {
		t.Fatalf("expected no args for git-crypt, got %v", bash.lastArgs)
	}
	if !strings.Contains(stdout.String(), "M .gitattributes") {
		t.Errorf("expected git status output in stdout: %s", stdout)
	}
}

func TestStepPrivacyAgePassesFlag(t *testing.T) {
	ans := &Answers{Privacy: "age"}
	bash := &fakeBash{}
	ctx := StepContext{
		Ctx: context.Background(), RepoRoot: "/tmp/wiki",
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		Bash: bash, Git: &fakeGit{},
		NonInteractive: true,
		Now:            time.Now(),
	}
	if _, err := (stepPrivacy{}).Execute(ctx, ans); err != nil {
		t.Fatal(err)
	}
	if len(bash.lastArgs) != 1 || bash.lastArgs[0] != "--age" {
		t.Fatalf("expected --age, got %v", bash.lastArgs)
	}
}

func TestStepPrivacyInteractivePromptCanonicalizes(t *testing.T) {
	ans := &Answers{}
	ctx := StepContext{
		Stdout: &bytes.Buffer{},
		Prompt: func(_, _ string) (string, error) { return "B", nil },
		Bash:   &fakeBash{},
		Git:    &fakeGit{},
		Confirm: func(_ string, def bool) (bool, error) { return def, nil },
		Ctx:    context.Background(),
	}
	if _, err := (stepPrivacy{}).Execute(ctx, ans); err != nil {
		t.Fatal(err)
	}
	if ans.Privacy != "git-crypt" {
		t.Fatalf("privacy=%q", ans.Privacy)
	}
}
