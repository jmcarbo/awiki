package initverb

import (
	"fmt"
	"path/filepath"
	"strings"
)

// stepPrivacyImpl runs Step 3: encryption decision.
//
// Three answers: none / git-crypt / age. For the latter two, shells
// scripts/encrypt-init.sh (with --age when applicable). After the
// script runs, we report `git status` so the user can verify the
// expected encrypted-vs-cleartext patterns.
//
// We keep the bash script (encrypt-init.sh) rather than re-deriving
// gpg/git-crypt logic in Go: it's interactive, runs once, and ships
// in the template.
type stepPrivacy struct{}

func (stepPrivacy) ID() string          { return "privacy" }
func (stepPrivacy) Description() string { return "encryption decision" }
func (stepPrivacy) Kind() Kind          { return KindHybrid }

func (stepPrivacy) Execute(ctx StepContext, ans *Answers) (Result, error) {
	if ans.Privacy == "" {
		if ctx.NonInteractive {
			ans.Privacy = "none"
		} else {
			if ctx.Prompt == nil {
				return Result{Status: StatusFailed}, fmt.Errorf("privacy: Prompt is nil")
			}
			fmt.Fprintln(ctx.Stdout, "Step 3 — Privacy. Pick one:")
			fmt.Fprintln(ctx.Stdout, "  A. None")
			fmt.Fprintln(ctx.Stdout, "  B. git-crypt")
			fmt.Fprintln(ctx.Stdout, "  C. age")
			val, err := ctx.Prompt("Choice", "A")
			if err != nil {
				return Result{Status: StatusFailed}, err
			}
			ans.Privacy = canonPrivacy(val)
		}
	}
	switch ans.Privacy {
	case "none", "git-crypt", "age":
	default:
		return Result{Status: StatusFailed}, fmt.Errorf("privacy: invalid value %q", ans.Privacy)
	}

	if ans.Privacy == "none" {
		return Result{Status: StatusApplied, Note: "privacy=none"}, nil
	}
	if ctx.Bash == nil {
		return Result{Status: StatusFailed}, fmt.Errorf("privacy: Bash adapter is nil")
	}
	script := filepath.Join(ctx.RepoRoot, "scripts", "encrypt-init.sh")
	args := []string{}
	if ans.Privacy == "age" {
		args = append(args, "--age")
	}
	out, code, err := ctx.Bash.Run(ctx.Ctx, ctx.RepoRoot, script, args...)
	if out != "" {
		fmt.Fprint(ctx.Stdout, out)
	}
	if err != nil || code != 0 {
		return Result{Status: StatusFailed},
			fmt.Errorf("encrypt-init.sh: code=%d err=%v", code, err)
	}

	// Show git status as the user-facing verification step.
	if ctx.Git != nil {
		statusOut, _, _ := ctx.Git.Status(ctx.Ctx, ctx.RepoRoot)
		if statusOut != "" {
			fmt.Fprintln(ctx.Stdout, "Encrypted-vs-cleartext verification (git status):")
			fmt.Fprint(ctx.Stdout, statusOut)
		}
	}
	if !ctx.NonInteractive && ctx.Confirm != nil {
		ok, err := ctx.Confirm("Continue past encrypt-init verification?", true)
		if err != nil {
			return Result{Status: StatusFailed}, err
		}
		if !ok {
			return Result{Status: StatusFailed},
				fmt.Errorf("privacy: user halted at git status verification")
		}
	}
	return Result{Status: StatusApplied, Note: "privacy=" + ans.Privacy}, nil
}

func canonPrivacy(in string) string {
	v := strings.TrimSpace(strings.ToLower(in))
	switch v {
	case "a", "none", "":
		return "none"
	case "b", "git-crypt", "gitcrypt":
		return "git-crypt"
	case "c", "age":
		return "age"
	}
	return v
}
