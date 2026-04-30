package ops

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TaskInitOptions configures `awiki task-init`.
type TaskInitOptions struct {
	RepoRoot string
	// AssumeYes/AssumeNo control the encryption + pre-commit prompts.
	// They mirror AWIKI_TASK_INIT_ASSUME_YES / AWIKI_TASK_INIT_ASSUME_NO.
	AssumeYes bool
	AssumeNo  bool
	// Now overrides time.Now (tests).
	Now time.Time
}

// TaskInit runs the per-step idempotent task-layer enabler. Mirrors
// scripts/task-init.sh.
func TaskInit(opts TaskInitOptions, stdout, stderr io.Writer) int {
	if opts.RepoRoot == "" {
		opts.RepoRoot, _ = os.Getwd()
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	taskCount := filepath.Join(opts.RepoRoot, ".awiki", "task-count")
	firstRun := true
	if _, err := os.Stat(taskCount); err == nil {
		firstRun = false
	}

	fmt.Fprintln(stdout, "TASK-INIT|start")

	if rc := taskInitStepPages(opts, stdout, stderr); rc != 0 {
		return rc
	}
	if rc := taskInitStepWikiMD(opts, stdout, stderr); rc != 0 {
		return rc
	}
	if rc := taskInitStepEncryption(opts, stdout, stderr); rc != 0 {
		return rc
	}
	if rc := taskInitStepConfig(opts, stdout, stderr); rc != 0 {
		return rc
	}
	if rc := taskInitStepStateFiles(opts, stdout, stderr); rc != 0 {
		return rc
	}
	if rc := taskInitStepPreCommit(opts, stdout, stderr); rc != 0 {
		return rc
	}

	// Logging — best-effort, non-fatal.
	logFile := resolveLogFile(opts.RepoRoot)
	msg := "noop"
	if firstRun {
		msg = "enabled"
	}
	_, _ = Log(LogOptions{
		Action:  "task-init",
		Message: msg,
		Now:     opts.Now,
		LogFile: logFile,
	})

	fmt.Fprintln(stdout, "TASK-INIT|done")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Smoke test:")
	fmt.Fprintln(stdout, `  just capture "pick up groceries"`)
	fmt.Fprintln(stdout, "  cat content/inbox.md   # should show your line below the frontmatter")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Next:")
	fmt.Fprintln(stdout, "  just scan       # rebuild .awiki/maps/actions.tsv")
	fmt.Fprintln(stdout, "  just agenda     # regenerate content/agenda/* views")
	fmt.Fprintln(stdout, "  just triage     # walk inbox.md + raw/inbox/interactive/")
	fmt.Fprintln(stdout, "  just review     # weekly review chain")
	return 0
}

// taskInitNote is the `note` helper from bash: emits TASK-INIT|<msg>.
func taskInitNote(stdout io.Writer, msg string) {
	fmt.Fprintf(stdout, "TASK-INIT|%s\n", msg)
}

func taskInitWarn(stderr io.Writer, msg string) {
	fmt.Fprintf(stderr, "TASK-INIT|WARN|%s\n", msg)
}

// taskInitStepPages: inbox.md, section indexes, agenda views, review-log,
// starter contexts.
func taskInitStepPages(opts TaskInitOptions, stdout, stderr io.Writer) int {
	today := opts.Now.Format("2006-01-02")

	// 1. inbox.md
	inbox := filepath.Join(opts.RepoRoot, "content", "inbox.md")
	if _, err := os.Stat(inbox); os.IsNotExist(err) {
		_ = os.MkdirAll(filepath.Dir(inbox), 0o755)
		body := `---
title: "Inbox"
type: inbox
draft: true
---

`
		if err := os.WriteFile(inbox, []byte(body), 0o644); err != nil {
			fmt.Fprintf(stderr, "task-init: %v\n", err)
			return 1
		}
		taskInitNote(stdout, "created content/inbox.md")
	} else {
		taskInitNote(stdout, "skip content/inbox.md (exists)")
	}

	// 2. section indexes
	for _, sec := range []string{"projects", "contexts", "agenda"} {
		dir := filepath.Join(opts.RepoRoot, "content", sec)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(stderr, "task-init: %v\n", err)
			return 1
		}
		idx := filepath.Join(dir, "_index.md")
		if _, err := os.Stat(idx); os.IsNotExist(err) {
			body := fmt.Sprintf(`---
title: "%s"
type: section
draft: false
---

{{< page-list >}}
`, capitalize(sec))
			if err := os.WriteFile(idx, []byte(body), 0o644); err != nil {
				fmt.Fprintf(stderr, "task-init: %v\n", err)
				return 1
			}
			taskInitNote(stdout, "created "+filepath.Join("content", sec, "_index.md"))
		} else {
			taskInitNote(stdout, "skip "+filepath.Join("content", sec, "_index.md")+" (exists)")
		}
	}

	// 3. agenda views
	for _, view := range []string{"next-actions", "today", "waiting", "someday", "stuck-projects"} {
		f := filepath.Join(opts.RepoRoot, "content", "agenda", view+".md")
		if _, err := os.Stat(f); os.IsNotExist(err) {
			label := strings.ReplaceAll(view, "-", " ")
			body := fmt.Sprintf(`---
title: "Agenda — %s"
type: agenda
last_updated: %s
draft: false
---

<!-- BEGIN agenda:%s -->
<!-- END agenda:%s -->
`, label, today, view, view)
			if err := os.WriteFile(f, []byte(body), 0o644); err != nil {
				fmt.Fprintf(stderr, "task-init: %v\n", err)
				return 1
			}
			taskInitNote(stdout, "created content/agenda/"+view+".md")
		} else {
			taskInitNote(stdout, "skip content/agenda/"+view+".md (exists)")
		}
	}

	// 4. review-log (no managed region)
	rl := filepath.Join(opts.RepoRoot, "content", "agenda", "review-log.md")
	if _, err := os.Stat(rl); os.IsNotExist(err) {
		body := fmt.Sprintf(`---
title: "Review Log"
type: agenda
last_updated: %s
draft: false
---

# Review Log

Append-only history of weekly reviews. New entries are added by
`+"`scripts/review-status.sh mark_done`"+` (phase 19).
`, today)
		if err := os.WriteFile(rl, []byte(body), 0o644); err != nil {
			fmt.Fprintf(stderr, "task-init: %v\n", err)
			return 1
		}
		taskInitNote(stdout, "created content/agenda/review-log.md")
	} else {
		taskInitNote(stdout, "skip content/agenda/review-log.md (exists)")
	}

	// 5. starter context pages
	for _, ctx := range []string{"phone", "errands", "computer"} {
		f := filepath.Join(opts.RepoRoot, "content", "contexts", ctx+".md")
		if _, err := os.Stat(f); os.IsNotExist(err) {
			body := fmt.Sprintf(`---
title: "@%s"
date: %s
last_updated: %s
type: context
aliases: ['@%s']
tools: []
draft: false
---

Actions tagged `+"`@%s`"+` reference this page.
`, ctx, today, today, ctx, ctx)
			if err := os.WriteFile(f, []byte(body), 0o644); err != nil {
				fmt.Fprintf(stderr, "task-init: %v\n", err)
				return 1
			}
			taskInitNote(stdout, "created content/contexts/"+ctx+".md")
		} else {
			taskInitNote(stdout, "skip content/contexts/"+ctx+".md (exists)")
		}
	}

	return 0
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// taskInitStepWikiMD: append/idempotent the task-layer block.
func taskInitStepWikiMD(opts TaskInitOptions, stdout, stderr io.Writer) int {
	wiki := filepath.Join(opts.RepoRoot, "WIKI.md")
	tmpl := filepath.Join(opts.RepoRoot, "scripts", "templates", "wiki-task-layer.md")
	if _, err := os.Stat(wiki); os.IsNotExist(err) {
		taskInitWarn(stderr, "WIKI.md not found at repo root; skipping task-layer block patch")
		return 0
	}
	tmplBytes, err := os.ReadFile(tmpl)
	if err != nil {
		taskInitWarn(stderr, "template missing: "+tmpl+"; skipping")
		return 0
	}
	wikiBody, err := os.ReadFile(wiki)
	if err != nil {
		fmt.Fprintf(stderr, "task-init: %v\n", err)
		return 1
	}
	begin := "<!-- BEGIN task-layer -->"
	end := "<!-- END task-layer -->"
	if strings.Contains(string(wikiBody), begin) {
		// Compare body with template body.
		tmplBody := extractMarkedBlock(string(tmplBytes), begin, end)
		curBody := extractMarkedBlock(string(wikiBody), begin, end)
		if tmplBody != curBody {
			taskInitWarn(stderr, "WIKI.md task-layer block has been customised; skipping (manual reconciliation required)")
		} else {
			taskInitNote(stdout, "skip WIKI.md (block already up to date)")
		}
		return 0
	}
	// Append.
	f, err := os.OpenFile(wiki, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(stderr, "task-init: %v\n", err)
		return 1
	}
	defer f.Close()
	_, _ = f.WriteString("\n")
	_, _ = f.Write(tmplBytes)
	taskInitNote(stdout, "appended task-layer block to WIKI.md")
	return 0
}

// extractMarkedBlock returns the body between BEGIN and END markers
// inclusive (mirrors awk /BEGIN/{f=1} f{print} /END/{f=0}).
func extractMarkedBlock(text, begin, end string) string {
	var out []string
	in := false
	for _, line := range strings.Split(text, "\n") {
		if !in && strings.Contains(line, begin) {
			in = true
		}
		if in {
			out = append(out, line)
		}
		if in && strings.Contains(line, end) {
			break
		}
	}
	return strings.Join(out, "\n")
}

// taskInitStepEncryption: append git-crypt patterns when accepted.
func taskInitStepEncryption(opts TaskInitOptions, stdout, stderr io.Writer) int {
	ga := filepath.Join(opts.RepoRoot, ".gitattributes")
	if _, err := os.Stat(ga); os.IsNotExist(err) {
		taskInitNote(stdout, "skip encryption (no .gitattributes — encrypt-init has not run)")
		return 0
	}
	body, err := os.ReadFile(ga)
	if err != nil {
		fmt.Fprintf(stderr, "task-init: %v\n", err)
		return 1
	}
	if !strings.Contains(string(body), "filter=git-crypt diff=git-crypt") {
		taskInitNote(stdout, "skip encryption (no git-crypt section in .gitattributes)")
		return 0
	}
	needInbox := !strings.Contains(string(body), "content/inbox.md filter=git-crypt diff=git-crypt")
	needAgenda := !strings.Contains(string(body), "content/agenda/** filter=git-crypt diff=git-crypt")
	if !needInbox && !needAgenda {
		taskInitNote(stdout, "skip encryption (patterns already present)")
		return 0
	}

	answer := resolveAssumePrompt(opts.AssumeYes, opts.AssumeNo,
		"AWIKI_TASK_INIT_ASSUME_YES", "AWIKI_TASK_INIT_ASSUME_NO")
	if answer == "n" {
		taskInitWarn(stderr, "encryption patterns declined; agenda pages will exclude private actions (placeholder count only)")
		return 0
	}
	if answer == "" {
		// No assume flag set + no interactive prompt available — bash defaults
		// to "y" via `read -r answer || answer="y"`. Match that.
		answer = "y"
	}
	f, err := os.OpenFile(ga, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(stderr, "task-init: %v\n", err)
		return 1
	}
	defer f.Close()
	if needInbox {
		_, _ = f.WriteString("\ncontent/inbox.md filter=git-crypt diff=git-crypt\n")
	}
	if needAgenda {
		_, _ = f.WriteString("content/agenda/** filter=git-crypt diff=git-crypt\n")
	}
	taskInitNote(stdout, "added git-crypt patterns for inbox + agenda")
	return 0
}

// resolveAssumePrompt reads AssumeYes/AssumeNo and falls back to env vars.
// Returns "y", "n", or "" (no decision).
func resolveAssumePrompt(yes, no bool, yesEnv, noEnv string) string {
	if yes {
		return "y"
	}
	if no {
		return "n"
	}
	if os.Getenv(yesEnv) == "1" {
		return "y"
	}
	if os.Getenv(noEnv) == "1" {
		return "n"
	}
	return ""
}

// taskInitStepConfig: ensure key/value pairs in .awiki/config.
func taskInitStepConfig(opts TaskInitOptions, stdout, stderr io.Writer) int {
	cfg := filepath.Join(opts.RepoRoot, ".awiki", "config")
	if _, err := os.Stat(cfg); os.IsNotExist(err) {
		_ = os.MkdirAll(filepath.Dir(cfg), 0o755)
		if err := os.WriteFile(cfg, nil, 0o644); err != nil {
			fmt.Fprintf(stderr, "task-init: %v\n", err)
			return 1
		}
	}
	for _, kv := range []struct{ k, v string }{
		{"AWIKI_AGENDA_AFTER_N", "5"},
		{"AWIKI_TASK_LAYER", "on"},
	} {
		body, _ := os.ReadFile(cfg)
		if hasConfigKey(string(body), kv.k) {
			taskInitNote(stdout, "skip "+kv.k+" (already set)")
			continue
		}
		f, err := os.OpenFile(cfg, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintf(stderr, "task-init: %v\n", err)
			return 1
		}
		_, _ = fmt.Fprintf(f, "%s=%s\n", kv.k, kv.v)
		_ = f.Close()
		taskInitNote(stdout, "appended "+kv.k+"="+kv.v)
	}
	return 0
}

func hasConfigKey(body, key string) bool {
	prefix := key + "="
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

// taskInitStepStateFiles: .awiki/last-review (today), .awiki/task-count (0).
func taskInitStepStateFiles(opts TaskInitOptions, stdout, stderr io.Writer) int {
	dir := filepath.Join(opts.RepoRoot, ".awiki")
	_ = os.MkdirAll(dir, 0o755)
	lr := filepath.Join(dir, "last-review")
	if _, err := os.Stat(lr); os.IsNotExist(err) {
		today := opts.Now.Format("2006-01-02")
		if err := os.WriteFile(lr, []byte(today+"\n"), 0o644); err != nil {
			fmt.Fprintf(stderr, "task-init: %v\n", err)
			return 1
		}
		taskInitNote(stdout, "created .awiki/last-review")
	} else {
		taskInitNote(stdout, "skip .awiki/last-review (exists)")
	}
	tc := filepath.Join(dir, "task-count")
	if _, err := os.Stat(tc); os.IsNotExist(err) {
		if err := os.WriteFile(tc, []byte("0\n"), 0o644); err != nil {
			fmt.Fprintf(stderr, "task-init: %v\n", err)
			return 1
		}
		taskInitNote(stdout, "created .awiki/task-count")
	} else {
		taskInitNote(stdout, "skip .awiki/task-count (exists)")
	}
	return 0
}

// taskInitStepPreCommit: install task-layer hook (idempotent via marker).
func taskInitStepPreCommit(opts TaskInitOptions, stdout, stderr io.Writer) int {
	gitDir := filepath.Join(opts.RepoRoot, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		taskInitNote(stdout, "skip pre-commit (not a git repo)")
		return 0
	}
	hookDir := filepath.Join(gitDir, "hooks")
	_ = os.MkdirAll(hookDir, 0o755)
	hook := filepath.Join(hookDir, "pre-commit")
	const marker = "# task-layer"

	if data, err := os.ReadFile(hook); err == nil {
		if hasMarkerLine(string(data), marker) {
			taskInitNote(stdout, "skip pre-commit (# task-layer marker already present)")
			return 0
		}
	}

	answer := resolveAssumePrompt(opts.AssumeYes, opts.AssumeNo,
		"AWIKI_TASK_INIT_ASSUME_YES", "AWIKI_TASK_INIT_ASSUME_NO")
	if answer == "n" {
		taskInitNote(stdout, "skip pre-commit (declined)")
		return 0
	}
	if answer == "" {
		answer = "y"
	}

	addition := `
# task-layer
# (Inserted by awiki task-init — do not remove the marker line above.)
awiki lint --alias-build-only
awiki scan
`
	if _, err := os.Stat(hook); os.IsNotExist(err) {
		body := "#!/usr/bin/env bash\nset -e\n" + addition
		if err := os.WriteFile(hook, []byte(body), 0o755); err != nil {
			fmt.Fprintf(stderr, "task-init: %v\n", err)
			return 1
		}
	} else {
		// Append, ensuring trailing newline first.
		existing, _ := os.ReadFile(hook)
		if !strings.HasSuffix(string(existing), "\n") {
			existing = append(existing, '\n')
			_ = os.WriteFile(hook, existing, 0o755)
		}
		f, err := os.OpenFile(hook, os.O_APPEND|os.O_WRONLY, 0o755)
		if err != nil {
			fmt.Fprintf(stderr, "task-init: %v\n", err)
			return 1
		}
		_, _ = f.WriteString(addition)
		_ = f.Close()
	}
	_ = os.Chmod(hook, 0o755)
	taskInitNote(stdout, "installed task-layer pre-commit hook at "+hook)
	return 0
}

// hasMarkerLine returns true if any standalone line of <body> equals <marker>.
func hasMarkerLine(body, marker string) bool {
	for _, line := range strings.Split(body, "\n") {
		if line == marker {
			return true
		}
	}
	return false
}

// TaskInitCLI parses argv and dispatches TaskInit.
func TaskInitCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: awiki task-init")
		return 2
	}
	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	return TaskInit(TaskInitOptions{RepoRoot: repoRoot}, stdout, stderr)
}
