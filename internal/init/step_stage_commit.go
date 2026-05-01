package initverb

import (
	"fmt"
)

// stepStageCommit runs Step 11: stage everything, show the file list,
// and (on consent) commit with a canonical message.
//
// Skip path (NonInteractive without StageCommit=true): we still stage
// + show, but do not commit. That preserves the manual-flow option of
// "let me eyeball the diff before I commit" while making the
// non-interactive default safe (stage but not commit).
type stepStageCommit struct{}

func (stepStageCommit) ID() string          { return "stage-commit" }
func (stepStageCommit) Description() string { return "initial commit" }
func (stepStageCommit) Kind() Kind          { return KindHybrid }

func (stepStageCommit) Execute(ctx StepContext, ans *Answers) (Result, error) {
	if ctx.Git == nil {
		return Result{Status: StatusFailed}, fmt.Errorf("stage-commit: Git adapter is nil")
	}
	if _, err := ctx.Git.AddAll(ctx.Ctx, ctx.RepoRoot); err != nil {
		return Result{Status: StatusFailed}, fmt.Errorf("git add -A: %w", err)
	}
	statusOut, _, err := ctx.Git.Status(ctx.Ctx, ctx.RepoRoot)
	if err != nil {
		return Result{Status: StatusFailed}, fmt.Errorf("git status: %w", err)
	}
	if statusOut != "" {
		fmt.Fprintln(ctx.Stdout, "Files staged:")
		fmt.Fprint(ctx.Stdout, statusOut)
	}

	if !ans.StageCommit && !ctx.NonInteractive {
		if ctx.Confirm == nil {
			return Result{Status: StatusFailed}, fmt.Errorf("stage-commit: Confirm is nil")
		}
		ok, err := ctx.Confirm("Stage all and commit?", false)
		if err != nil {
			return Result{Status: StatusFailed}, err
		}
		ans.StageCommit = ok
	}
	if !ans.StageCommit {
		return Result{Status: StatusSkipped, Note: "stage only; no commit"}, nil
	}
	msg := fmt.Sprintf("chore: initialize wiki '%s'", ans.WikiName)
	out, code, err := ctx.Git.Commit(ctx.Ctx, ctx.RepoRoot, msg)
	if out != "" {
		fmt.Fprint(ctx.Stdout, out)
	}
	if err != nil || code != 0 {
		return Result{Status: StatusFailed},
			fmt.Errorf("git commit: code=%d err=%v", code, err)
	}
	return Result{Status: StatusApplied, Note: "committed: " + msg}, nil
}
