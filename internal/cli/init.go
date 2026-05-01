package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"awiki/internal/adapters"
	initverb "awiki/internal/init"
)

// runInit dispatches `awiki init [flags]`. Mirrors the spec in
// BOOTSTRAP.md: orchestrates the 14-step bootstrap flow with
// idempotent state under .awiki/init-state.json.
func runInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	agent := fs.String("agent", "",
		"agent CLI (claude/codex/opencode/gemini/...) for delegated steps")
	nonInteractive := fs.Bool("non-interactive", false,
		"skip every prompt; use sane defaults")
	configPath := fs.String("config", "",
		"YAML config file with answers (CI/scripted setup)")
	cont := fs.Bool("continue", false,
		"resume from .awiki/init-state.json")
	reset := fs.Bool("reset", false,
		"wipe .awiki/init-state.json and start over")
	skipSteps := stringList{}
	fs.Var(&skipSteps, "skip-step",
		"skip the named step (repeatable)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "init: unexpected positional args: %v\n", fs.Args())
		return 2
	}

	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}

	deps := initverb.StepContext{
		Stdin:    os.Stdin,
		Stdout:   stdout,
		Stderr:   stderr,
		Agent:    adapters.ExecInitAgent{},
		AgentCLI: *agent,
		Git:      adapters.ExecInitGit{},
		Bash:     adapters.ExecInitBash{},
		NPM:      adapters.ExecInitNPM{},
		Confirm:  initverb.NewConfirmFn(os.Stdin, stderr),
		Prompt:   initverb.NewPromptFn(os.Stdin, stderr),
	}

	rc, err := initverb.Run(initverb.Options{
		RepoRoot:       repoRoot,
		Agent:          *agent,
		NonInteractive: *nonInteractive,
		SkipSteps:      []string(skipSteps),
		ConfigPath:     *configPath,
		Continue:       *cont,
		Reset:          *reset,
	}, deps)
	if err != nil {
		fmt.Fprintf(stderr, "init: %v\n", err)
		if rc == 0 {
			rc = 1
		}
	}
	return rc
}

// stringList implements flag.Value for repeatable string flags
// (--skip-step a --skip-step b).
type stringList []string

func (s *stringList) String() string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("%v", []string(*s))
}

func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}
