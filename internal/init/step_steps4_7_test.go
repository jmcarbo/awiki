package initverb

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Step 4: track-processed ---

func TestStepTrackProcessedNo(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"),
		[]byte("raw/processed/*\n!raw/processed/.gitkeep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ans := &Answers{} // false
	ctx := StepContext{
		RepoRoot:       dir,
		NonInteractive: true,
		Stdout:         &bytes.Buffer{},
	}
	res, err := (stepTrackProcessed{}).Execute(ctx, ans)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusApplied {
		t.Fatalf("status=%s", res.Status)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(body), "raw/processed/*") {
		t.Errorf(".gitignore should still contain raw/processed/*: %s", body)
	}
}

func TestStepTrackProcessedYesStripsLines(t *testing.T) {
	dir := t.TempDir()
	body := "# header\nraw/processed/*\n!raw/processed/.gitkeep\ncontent/private/**\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ans := &Answers{TrackProcessed: true}
	ctx := StepContext{
		RepoRoot:       dir,
		NonInteractive: true,
		Stdout:         &bytes.Buffer{},
	}
	res, err := (stepTrackProcessed{}).Execute(ctx, ans)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Note, "removed 2") {
		t.Fatalf("note=%q", res.Note)
	}
	updated, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if strings.Contains(string(updated), "raw/processed/*") {
		t.Errorf("raw/processed/* should be gone: %s", updated)
	}
	if strings.Contains(string(updated), "!raw/processed/.gitkeep") {
		t.Errorf("!raw/processed/.gitkeep should be gone: %s", updated)
	}
	if !strings.Contains(string(updated), "content/private/**") {
		t.Errorf("content/private/** should remain: %s", updated)
	}
}

func TestStepTrackProcessedYesIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"),
		[]byte("# header\ncontent/private/**\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ans := &Answers{TrackProcessed: true}
	ctx := StepContext{
		RepoRoot:       dir,
		NonInteractive: true,
		Stdout:         &bytes.Buffer{},
	}
	res, err := (stepTrackProcessed{}).Execute(ctx, ans)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Note, "already in .gitignore") {
		t.Fatalf("expected idempotent note, got %q", res.Note)
	}
}

// --- Step 5: theme ---

func TestStepThemeDefault(t *testing.T) {
	dir := t.TempDir()
	ans := &Answers{Theme: defaultTheme}
	ctx := StepContext{
		Ctx: context.Background(), RepoRoot: dir,
		NonInteractive: true,
		Stdout:         &bytes.Buffer{},
		Git:            &fakeGit{},
	}
	res, err := (stepTheme{}).Execute(ctx, ans)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Note, "default kept") {
		t.Fatalf("note=%q", res.Note)
	}
}

func TestStepThemeCustomRequiresURL(t *testing.T) {
	dir := t.TempDir()
	ans := &Answers{Theme: "papermod"} // no URL
	ctx := StepContext{
		Ctx: context.Background(), RepoRoot: dir,
		NonInteractive: true,
		Stdout:         &bytes.Buffer{},
		Git:            &fakeGit{},
	}
	if _, err := (stepTheme{}).Execute(ctx, ans); err == nil {
		t.Fatal("expected error for theme without URL")
	}
}

func TestStepThemeCustomRewrites(t *testing.T) {
	dir := t.TempDir()
	body := "baseURL = 'https://example.com/'\ntheme = 'hugo-book'\ntitle = 'awiki'\n"
	if err := os.WriteFile(filepath.Join(dir, "hugo.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ans := &Answers{Theme: "papermod=https://github.com/adityatelange/hugo-PaperMod"}
	ctx := StepContext{
		Ctx: context.Background(), RepoRoot: dir,
		NonInteractive: true,
		Stdout:         &bytes.Buffer{},
		Git:            &fakeGit{},
	}
	res, err := (stepTheme{}).Execute(ctx, ans)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusApplied {
		t.Fatalf("status=%s", res.Status)
	}
	updated, _ := os.ReadFile(filepath.Join(dir, "hugo.toml"))
	if !strings.Contains(string(updated), "theme = 'papermod'") {
		t.Errorf("hugo.toml should reference papermod: %s", updated)
	}
}

// --- Step 6: publish-log ---

func TestStepPublishLogToggle(t *testing.T) {
	dir := t.TempDir()
	cdir := filepath.Join(dir, "content")
	if err := os.MkdirAll(cdir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\ntitle: \"Log\"\ntype: log\ndraft: true\n---\n\n# Log\n"
	if err := os.WriteFile(filepath.Join(cdir, "log.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ans := &Answers{PublishLog: true}
	ctx := StepContext{
		RepoRoot: dir,
		NonInteractive: true,
		Stdout:         &bytes.Buffer{},
	}
	res, err := (stepPublishLog{}).Execute(ctx, ans)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusApplied {
		t.Fatalf("status=%s", res.Status)
	}
	updated, _ := os.ReadFile(filepath.Join(cdir, "log.md"))
	if !strings.Contains(string(updated), "draft: false") {
		t.Errorf("expected draft:false in log.md: %s", updated)
	}
}

// --- Step 7: patch-identity ---

func TestStepPatchIdentityWithoutAgentLeavesPlaceholder(t *testing.T) {
	dir := setupIdentityFixture(t)
	ans := &Answers{
		WikiName: "history-of-rome",
		Domain:   "research",
		Purpose:  "Tracking the Republic + early Empire",
	}
	ctx := StepContext{
		Ctx: context.Background(), RepoRoot: dir,
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		NonInteractive: true,
	}
	res, err := (stepPatchIdentity{}).Execute(ctx, ans)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Note, "TODO") {
		t.Errorf("expected landing=TODO note, got %q", res.Note)
	}
	wiki, _ := os.ReadFile(filepath.Join(dir, "WIKI.md"))
	if !strings.Contains(string(wiki), "`history-of-rome`") {
		t.Errorf("WIKI.md missing wiki_name: %s", wiki)
	}
	if !strings.Contains(string(wiki), "`research`") {
		t.Errorf("WIKI.md missing domain: %s", wiki)
	}
	idx, _ := os.ReadFile(filepath.Join(dir, "content", "_index.md"))
	if !strings.Contains(string(idx), "BOOTSTRAP-LANDING-START") {
		t.Errorf("content/_index.md missing landing block: %s", idx)
	}
	if !strings.Contains(string(idx), patchIdentityPlaceholder) {
		t.Errorf("content/_index.md missing placeholder: %s", idx)
	}
	hugo, _ := os.ReadFile(filepath.Join(dir, "hugo.toml"))
	if !strings.Contains(string(hugo), "title = 'history-of-rome'") {
		t.Errorf("hugo.toml title not patched: %s", hugo)
	}
}

func TestStepPatchIdentityWithAgentInjectsLanding(t *testing.T) {
	dir := setupIdentityFixture(t)
	ans := &Answers{WikiName: "rome", Domain: "research", Purpose: "p"}
	agent := &fakeAgent{out: "A wiki about ancient Rome focusing on the Republic.\n"}
	ctx := StepContext{
		Ctx: context.Background(), RepoRoot: dir,
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		NonInteractive: true,
		Agent:          agent,
		AgentCLI:       "claude",
	}
	if _, err := (stepPatchIdentity{}).Execute(ctx, ans); err != nil {
		t.Fatal(err)
	}
	if agent.calls != 1 {
		t.Fatalf("agent calls=%d", agent.calls)
	}
	if agent.lastCLI != "claude" {
		t.Fatalf("agent CLI=%q", agent.lastCLI)
	}
	if !strings.Contains(agent.lastPrompt, "rome") {
		t.Fatalf("agent prompt missing wiki name: %s", agent.lastPrompt)
	}
	idx, _ := os.ReadFile(filepath.Join(dir, "content", "_index.md"))
	if !strings.Contains(string(idx), "ancient Rome") {
		t.Errorf("agent landing not injected: %s", idx)
	}
}

func setupIdentityFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "content"), 0o755); err != nil {
		t.Fatal(err)
	}
	wiki := `# WIKI Schema

## 1. Identity
<!-- BOOTSTRAP fills this section. Do not edit manually after bootstrap. -->
- **wiki_name:** ` + "`<unset>`" + `
- **domain:** ` + "`<unset>`" + `
- **purpose:** ` + "`<unset>`" + `

## 2. Page Conventions
`
	if err := os.WriteFile(filepath.Join(dir, "WIKI.md"), []byte(wiki), 0o644); err != nil {
		t.Fatal(err)
	}
	hugo := "baseURL = 'https://example.com/'\ntitle = 'awiki'\ntheme = 'hugo-book'\n"
	if err := os.WriteFile(filepath.Join(dir, "hugo.toml"), []byte(hugo), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := "---\ntitle: \"awiki\"\ntype: section-index\n---\n\n# Welcome\n\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "content", "_index.md"), []byte(idx), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
