package initverb

import (
	"fmt"
	"path/filepath"

	"awiki/internal/ops"
)

// stepLogInit runs Step 10: append the canonical "wiki '<name>'
// initialized for domain '<domain>'" line to content/log.md.
type stepLogInit struct{}

func (stepLogInit) ID() string          { return "log-init" }
func (stepLogInit) Description() string { return "log init entry" }
func (stepLogInit) Kind() Kind          { return KindMechanical }

func (stepLogInit) Execute(ctx StepContext, ans *Answers) (Result, error) {
	if ans.WikiName == "" {
		return Result{Status: StatusFailed}, fmt.Errorf("log-init: wiki_name not set")
	}
	domain := ans.Domain
	if domain == "" {
		domain = "<unset>"
	}
	msg := fmt.Sprintf("wiki '%s' initialized for domain '%s'", ans.WikiName, domain)
	logFile := filepath.Join(ctx.RepoRoot, "content", "log.md")
	if _, err := ops.Log(ops.LogOptions{
		Action:  "init",
		Message: msg,
		LogFile: logFile,
		Now:     ctx.Now,
	}); err != nil {
		return Result{Status: StatusFailed}, fmt.Errorf("log: %w", err)
	}
	return Result{Status: StatusApplied, Note: "appended init entry to content/log.md"}, nil
}
