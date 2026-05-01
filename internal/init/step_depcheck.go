package initverb

import (
	"fmt"

	"awiki/internal/ops"
)

// stepDepCheck runs Step 0: submodules + dependency check.
//
// Mirrors BOOTSTRAP.md §0:
//  1. `git submodule update --init --recursive`
//  2. `awiki check-deps` (the Go port; the deleted bash script lives
//     in template provenance only).
//
// A non-zero check-deps exit halts the verb so the user can install
// missing tools and re-run with --continue.
type stepDepCheck struct{}

func (stepDepCheck) ID() string          { return "dep-check" }
func (stepDepCheck) Description() string { return "submodules + dependency check" }
func (stepDepCheck) Kind() Kind          { return KindMechanical }

func (stepDepCheck) Execute(ctx StepContext, _ *Answers) (Result, error) {
	if ctx.Git == nil {
		return Result{Status: StatusFailed}, fmt.Errorf("dep-check: Git adapter is nil")
	}
	out, code, err := ctx.Git.SubmoduleUpdate(ctx.Ctx, ctx.RepoRoot)
	if err != nil || code != 0 {
		fmt.Fprintln(ctx.Stderr, out)
		return Result{Status: StatusFailed},
			fmt.Errorf("git submodule update --init --recursive: code=%d err=%v", code, err)
	}
	if out != "" {
		fmt.Fprint(ctx.Stdout, out)
	}
	rc := ops.CheckDeps(ops.CheckDepsOptions{RepoRoot: ctx.RepoRoot}, ctx.Stdout, ctx.Stderr)
	if rc != 0 {
		return Result{Status: StatusFailed, Note: "check-deps reported missing tools"},
			fmt.Errorf("dep-check: install missing tools and re-run with --continue")
	}
	return Result{Status: StatusApplied, Note: "submodules + check-deps OK"}, nil
}
