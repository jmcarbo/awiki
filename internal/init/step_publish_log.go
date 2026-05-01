package initverb

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// stepPublishLog runs Step 6: ask whether the log page should render
// on the public site. Sets the `draft:` flag in content/log.md
// frontmatter accordingly.
type stepPublishLog struct{}

func (stepPublishLog) ID() string          { return "publish-log" }
func (stepPublishLog) Description() string { return "publish log to rendered site?" }
func (stepPublishLog) Kind() Kind          { return KindHybrid }

// draftLineRe matches the YAML frontmatter `draft: <bool>` line that
// content/log.md ships with. We tolerate any indentation + arbitrary
// truthy/falsy values — only the line is rewritten.
var draftLineRe = regexp.MustCompile(`(?m)^(\s*draft\s*:\s*).*$`)

func (stepPublishLog) Execute(ctx StepContext, ans *Answers) (Result, error) {
	if !ans.PublishLog && !ctx.NonInteractive {
		if ctx.Confirm == nil {
			return Result{Status: StatusFailed}, fmt.Errorf("publish-log: Confirm is nil")
		}
		ok, err := ctx.Confirm("Publish log to rendered site?", false)
		if err != nil {
			return Result{Status: StatusFailed}, err
		}
		ans.PublishLog = ok
	}
	want := "true"
	if ans.PublishLog {
		want = "false" // draft:false renders
	}
	logPath := filepath.Join(ctx.RepoRoot, "content", "log.md")
	body, err := os.ReadFile(logPath)
	if err != nil {
		return Result{Status: StatusFailed}, fmt.Errorf("read content/log.md: %w", err)
	}
	rewritten := draftLineRe.ReplaceAllString(string(body), "${1}"+want)
	// If no draft: line existed (shouldn't happen with template), inject it.
	if !strings.Contains(rewritten, "draft:") {
		rewritten = injectDraftLine(rewritten, want)
	}
	if rewritten == string(body) {
		return Result{Status: StatusApplied, Note: fmt.Sprintf("publish_log=%t (already set)", ans.PublishLog)}, nil
	}
	if err := os.WriteFile(logPath, []byte(rewritten), 0o644); err != nil {
		return Result{Status: StatusFailed}, fmt.Errorf("write content/log.md: %w", err)
	}
	return Result{Status: StatusApplied, Note: fmt.Sprintf("publish_log=%t", ans.PublishLog)}, nil
}

// injectDraftLine inserts a `draft: <val>` line just before the
// closing `---` of the frontmatter. If no frontmatter is detected,
// returns the body unchanged.
func injectDraftLine(body, val string) string {
	lines := strings.Split(body, "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" {
		return body
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			injected := append([]string{}, lines[:i]...)
			injected = append(injected, "draft: "+val)
			injected = append(injected, lines[i:]...)
			return strings.Join(injected, "\n")
		}
	}
	return body
}
