package ops

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTaskInitRepo seeds a minimal worktree with WIKI.md, .awiki/config,
// and a copy of scripts/templates/wiki-task-layer.md so taskInitStepWikiMD
// can find its template.
func setupTaskInitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".awiki"))
	mustMkdir(t, filepath.Join(dir, "content"))
	mustMkdir(t, filepath.Join(dir, "scripts", "templates"))

	// Copy the actual template — we need the same content the bash script
	// would have used so the body-comparison branch behaves identically.
	repoRoot := findRepoRoot(t)
	tmpl, err := os.ReadFile(filepath.Join(repoRoot, "scripts", "templates", "wiki-task-layer.md"))
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	mustWrite(t, filepath.Join(dir, "scripts", "templates", "wiki-task-layer.md"), string(tmpl))

	mustWrite(t, filepath.Join(dir, ".awiki", "config"),
		"AWIKI_LINT_AFTER_N=5\nAWIKI_STALE_DAYS=90\nAWIKI_LOG_QUERIES=0\n")
	mustWrite(t, filepath.Join(dir, "WIKI.md"),
		"---\ntitle: \"WIKI\"\n---\n\n# WIKI\n")
	t.Setenv("AWIKI_REPO_ROOT", dir)
	return dir
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func TestTaskInitCreatesStateFiles(t *testing.T) {
	dir := setupTaskInitRepo(t)
	rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &bytes.Buffer{}, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if _, err := os.Stat(filepath.Join(dir, ".awiki", "last-review")); err != nil {
		t.Fatalf("missing last-review: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".awiki", "task-count")); err != nil {
		t.Fatalf("missing task-count: %v", err)
	}
	cnt := strings.TrimSpace(mustRead(t, filepath.Join(dir, ".awiki", "task-count")))
	if cnt != "0" {
		t.Fatalf("task-count=%q want 0", cnt)
	}
}

func TestTaskInitAppendsConfigKeys(t *testing.T) {
	dir := setupTaskInitRepo(t)
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	body := mustRead(t, filepath.Join(dir, ".awiki", "config"))
	if !strings.Contains(body, "AWIKI_AGENDA_AFTER_N=5") {
		t.Fatalf("missing AWIKI_AGENDA_AFTER_N: %s", body)
	}
	if !strings.Contains(body, "AWIKI_TASK_LAYER=on") {
		t.Fatalf("missing AWIKI_TASK_LAYER: %s", body)
	}
}

func TestTaskInitPreservesUserSetConfig(t *testing.T) {
	dir := setupTaskInitRepo(t)
	mustWrite(t, filepath.Join(dir, ".awiki", "config"), "AWIKI_AGENDA_AFTER_N=10\n")
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	body := mustRead(t, filepath.Join(dir, ".awiki", "config"))
	if strings.Count(body, "AWIKI_AGENDA_AFTER_N=") != 1 {
		t.Fatalf("duplicated key: %s", body)
	}
	if !strings.Contains(body, "AWIKI_AGENDA_AFTER_N=10") {
		t.Fatalf("user value overwritten: %s", body)
	}
}

func TestTaskInitIdempotentRerun(t *testing.T) {
	dir := setupTaskInitRepo(t)
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("first rc=%d", rc)
	}
	first := mustRead(t, filepath.Join(dir, ".awiki", "last-review"))
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("second rc=%d", rc)
	}
	second := mustRead(t, filepath.Join(dir, ".awiki", "last-review"))
	if first != second {
		t.Fatalf("last-review mutated: %q vs %q", first, second)
	}
}

func TestTaskInitCreatesInboxAndIndexes(t *testing.T) {
	dir := setupTaskInitRepo(t)
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	inbox := mustRead(t, filepath.Join(dir, "content", "inbox.md"))
	if !strings.Contains(inbox, "type: inbox") {
		t.Fatalf("inbox missing type: inbox:\n%s", inbox)
	}
	for _, sec := range []string{"projects", "contexts", "agenda"} {
		if _, err := os.Stat(filepath.Join(dir, "content", sec, "_index.md")); err != nil {
			t.Fatalf("missing %s/_index.md", sec)
		}
	}
}

func TestTaskInitCreatesAgendaPagesWithRegions(t *testing.T) {
	dir := setupTaskInitRepo(t)
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	for _, view := range []string{"next-actions", "today", "waiting", "someday", "stuck-projects"} {
		body := mustRead(t, filepath.Join(dir, "content", "agenda", view+".md"))
		if !strings.Contains(body, "<!-- BEGIN agenda:"+view+" -->") {
			t.Fatalf("%s missing BEGIN: %s", view, body)
		}
		if !strings.Contains(body, "<!-- END agenda:"+view+" -->") {
			t.Fatalf("%s missing END: %s", view, body)
		}
		if !strings.Contains(body, "type: agenda") {
			t.Fatalf("%s missing type:", view)
		}
	}
}

func TestTaskInitCreatesReviewLogWithoutRegion(t *testing.T) {
	dir := setupTaskInitRepo(t)
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	body := mustRead(t, filepath.Join(dir, "content", "agenda", "review-log.md"))
	if !strings.Contains(body, "type: agenda") {
		t.Fatalf("missing type:")
	}
	if strings.Contains(body, "<!-- BEGIN agenda:") {
		t.Fatalf("review-log should not have managed region: %s", body)
	}
}

func TestTaskInitCreatesContextPages(t *testing.T) {
	dir := setupTaskInitRepo(t)
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	for _, ctx := range []string{"phone", "errands", "computer"} {
		body := mustRead(t, filepath.Join(dir, "content", "contexts", ctx+".md"))
		if !strings.Contains(body, "type: context") {
			t.Fatalf("%s missing type:", ctx)
		}
		if !strings.Contains(body, "@"+ctx) {
			t.Fatalf("%s missing alias:", ctx)
		}
	}
}

func TestTaskInitDoesNotOverwriteCustomPage(t *testing.T) {
	dir := setupTaskInitRepo(t)
	mustMkdir(t, filepath.Join(dir, "content", "contexts"))
	mustWrite(t, filepath.Join(dir, "content", "contexts", "phone.md"),
		"---\ntype: context\ncustom: yes\n---\n\nMy own thing.\n")
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	body := mustRead(t, filepath.Join(dir, "content", "contexts", "phone.md"))
	if !strings.Contains(body, "custom: yes") {
		t.Fatalf("custom file overwritten: %s", body)
	}
}

func TestTaskInitAppendsTaskLayerToWIKI(t *testing.T) {
	dir := setupTaskInitRepo(t)
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	body := mustRead(t, filepath.Join(dir, "WIKI.md"))
	if !strings.Contains(body, "<!-- BEGIN task-layer -->") {
		t.Fatalf("missing BEGIN: %s", body)
	}
	if !strings.Contains(body, "<!-- END task-layer -->") {
		t.Fatalf("missing END: %s", body)
	}
}

func TestTaskInitDoesNotDoubleAppendWIKI(t *testing.T) {
	dir := setupTaskInitRepo(t)
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	body := mustRead(t, filepath.Join(dir, "WIKI.md"))
	if strings.Count(body, "<!-- BEGIN task-layer -->") != 1 {
		t.Fatalf("duplicated BEGIN: %s", body)
	}
}

func TestTaskInitWarnsOnCustomisedBlock(t *testing.T) {
	dir := setupTaskInitRepo(t)
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	body := mustRead(t, filepath.Join(dir, "WIKI.md"))
	body = strings.Replace(body,
		"<!-- BEGIN task-layer -->",
		"<!-- BEGIN task-layer -->\nMY CUSTOM TEXT", 1)
	mustWrite(t, filepath.Join(dir, "WIKI.md"), body)
	var stderr bytes.Buffer
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &bytes.Buffer{}, &stderr); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if !strings.Contains(stderr.String(), "WARN") {
		t.Fatalf("missing WARN: %s", stderr.String())
	}
	body2 := mustRead(t, filepath.Join(dir, "WIKI.md"))
	if !strings.Contains(body2, "MY CUSTOM TEXT") {
		t.Fatalf("user customisation lost")
	}
}

func TestTaskInitSkipsEncryptionWhenNoGitattributes(t *testing.T) {
	dir := setupTaskInitRepo(t)
	var stdout bytes.Buffer
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &stdout, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if !strings.Contains(stdout.String(), "skip encryption") {
		t.Fatalf("missing skip encryption: %s", stdout.String())
	}
}

func TestTaskInitAppendsGitCryptWhenAssumeYes(t *testing.T) {
	dir := setupTaskInitRepo(t)
	mustWrite(t, filepath.Join(dir, ".gitattributes"),
		"content/private/** filter=git-crypt diff=git-crypt\n")
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir, AssumeYes: true}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	body := mustRead(t, filepath.Join(dir, ".gitattributes"))
	if !strings.Contains(body, "content/inbox.md filter=git-crypt diff=git-crypt") {
		t.Fatalf("missing inbox pattern: %s", body)
	}
	if !strings.Contains(body, "content/agenda/** filter=git-crypt diff=git-crypt") {
		t.Fatalf("missing agenda pattern: %s", body)
	}
}

func TestTaskInitDeclinesGitCryptWhenAssumeNo(t *testing.T) {
	dir := setupTaskInitRepo(t)
	mustWrite(t, filepath.Join(dir, ".gitattributes"),
		"content/private/** filter=git-crypt diff=git-crypt\n")
	var stderr bytes.Buffer
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir, AssumeNo: true}, &bytes.Buffer{}, &stderr); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	body := mustRead(t, filepath.Join(dir, ".gitattributes"))
	if strings.Contains(body, "content/inbox.md filter=git-crypt") {
		t.Fatalf("pattern unexpectedly added: %s", body)
	}
	if !strings.Contains(stderr.String(), "WARN") &&
		!strings.Contains(stderr.String(), "declined") {
		t.Fatalf("missing diag: %s", stderr.String())
	}
}

func TestTaskInitInstallsPreCommitWhenAssumeYes(t *testing.T) {
	dir := setupTaskInitRepo(t)
	// fake .git dir so taskInitStepPreCommit fires.
	mustMkdir(t, filepath.Join(dir, ".git", "hooks"))
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir, AssumeYes: true}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	hook := filepath.Join(dir, ".git", "hooks", "pre-commit")
	body := mustRead(t, hook)
	if !strings.Contains(body, "# task-layer") {
		t.Fatalf("missing marker: %s", body)
	}
	if !strings.Contains(body, "awiki scan") {
		t.Fatalf("missing scan invocation: %s", body)
	}
	if !strings.Contains(body, "awiki lint --alias-build-only") {
		t.Fatalf("missing lint --alias-build-only: %s", body)
	}
	info, _ := os.Stat(hook)
	if info.Mode()&0o111 == 0 {
		t.Fatalf("hook not executable")
	}
}

func TestTaskInitPreCommitIdempotent(t *testing.T) {
	dir := setupTaskInitRepo(t)
	mustMkdir(t, filepath.Join(dir, ".git", "hooks"))
	for range 2 {
		if rc := TaskInit(TaskInitOptions{RepoRoot: dir, AssumeYes: true}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
			t.Fatalf("rc=%d", rc)
		}
	}
	body := mustRead(t, filepath.Join(dir, ".git", "hooks", "pre-commit"))
	if strings.Count(body, "# task-layer") != 1 {
		t.Fatalf("marker duplicated: %s", body)
	}
}

func TestTaskInitPreCommitAppendsToExisting(t *testing.T) {
	dir := setupTaskInitRepo(t)
	mustMkdir(t, filepath.Join(dir, ".git", "hooks"))
	mustWrite(t, filepath.Join(dir, ".git", "hooks", "pre-commit"),
		"#!/usr/bin/env bash\nset -e\njust lint\n")
	_ = os.Chmod(filepath.Join(dir, ".git", "hooks", "pre-commit"), 0o755)
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir, AssumeYes: true}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	body := mustRead(t, filepath.Join(dir, ".git", "hooks", "pre-commit"))
	if !strings.Contains(body, "just lint") {
		t.Fatalf("original line lost: %s", body)
	}
	if !strings.Contains(body, "# task-layer") {
		t.Fatalf("addition missing: %s", body)
	}
}

func TestTaskInitPreCommitSkipsWithoutGit(t *testing.T) {
	dir := setupTaskInitRepo(t)
	var stdout bytes.Buffer
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir, AssumeYes: true}, &stdout, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if !strings.Contains(stdout.String(), "skip pre-commit") {
		t.Fatalf("missing skip diag: %s", stdout.String())
	}
}

func TestTaskInitPrintsSmokeInstructions(t *testing.T) {
	dir := setupTaskInitRepo(t)
	var stdout bytes.Buffer
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &stdout, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	got := stdout.String()
	for _, want := range []string{"just capture", "just scan", "just review"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in output: %s", want, got)
		}
	}
}

func TestTaskInitLogsFirstRunAndNoop(t *testing.T) {
	dir := setupTaskInitRepo(t)
	logFile := filepath.Join(dir, "content", "log.md")
	t.Setenv("AWIKI_LOG_FILE", logFile)
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("first rc=%d", rc)
	}
	if rc := TaskInit(TaskInitOptions{RepoRoot: dir}, &bytes.Buffer{}, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("second rc=%d", rc)
	}
	body := mustRead(t, logFile)
	if !strings.Contains(body, "task-init | enabled") {
		t.Fatalf("missing first-run log: %s", body)
	}
	if !strings.Contains(body, "task-init | noop") {
		t.Fatalf("missing noop log: %s", body)
	}
}
