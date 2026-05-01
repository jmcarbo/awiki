package initverb

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Kind tags each step so callers (and tests) can filter on the
// classification BOOTSTRAP.md uses.
type Kind string

const (
	KindMechanical  Kind = "mechanical"
	KindInteractive Kind = "interactive"
	KindAgent       Kind = "agent"
	KindHybrid      Kind = "hybrid"
)

// Result is what each Step.Execute returns. Status is one of the
// Status* constants from state.go. Note is a free-form one-liner
// surfaced to the user + persisted to init-state.json.
type Result struct {
	Status string
	Note   string
}

// Step is one of the 14 BOOTSTRAP.md steps. Implementations are
// idempotent: re-running an already-applied step must be a no-op
// (or a benign re-record).
type Step interface {
	ID() string
	Description() string
	Kind() Kind
	Execute(ctx StepContext, ans *Answers) (Result, error)
}

// StepContext carries the dependencies a step needs. Mostly adapter
// interfaces so the domain code stays exec-free; production wires
// real adapters via internal/adapters, tests inject fakes.
type StepContext struct {
	Ctx            context.Context
	Stdin          io.Reader
	Stdout         io.Writer
	Stderr         io.Writer
	RepoRoot       string
	Now            time.Time
	Agent          AgentRunner
	Git            GitInit
	Bash           BashRunner
	NPM            NPMRunner
	Confirm        ConfirmFn
	Prompt         PromptFn
	NonInteractive bool
	Skip           map[string]bool
}

// Options bundles every CLI flag the verb supports.
type Options struct {
	RepoRoot       string
	Agent          string
	NonInteractive bool
	SkipSteps      []string
	ConfigPath     string
	Continue       bool
	Reset          bool
	// Now overrides the timestamp source for tests.
	Now time.Time
}

// Run is the top-level entrypoint. It loads/initializes state, runs
// each step in BOOTSTRAP.md order, and persists state after each step
// so a Ctrl-C / process kill leaves the run resumable.
func Run(opts Options, deps StepContext) (int, error) {
	if opts.RepoRoot == "" {
		opts.RepoRoot, _ = os.Getwd()
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	deps.RepoRoot = opts.RepoRoot
	deps.Now = opts.Now
	deps.NonInteractive = opts.NonInteractive

	skipSet := map[string]bool{}
	for _, s := range opts.SkipSteps {
		skipSet[s] = true
	}
	deps.Skip = skipSet

	if opts.Reset {
		if err := ResetState(opts.RepoRoot); err != nil {
			return 1, err
		}
		fmt.Fprintln(deps.Stdout, "INIT|reset|.awiki/init-state.json removed")
	}

	state, err := LoadState(opts.RepoRoot)
	if err != nil {
		return 1, err
	}
	if state == nil {
		state = NewState(opts.Now, opts.Agent)
	}
	if state.Agent == "" && opts.Agent != "" {
		state.Agent = opts.Agent
	}

	// Layer config-file answers (if any) on top of any pre-existing
	// answers from a prior partial run. Explicit values in the config
	// always win; they fill stub defaults set by --non-interactive.
	if opts.ConfigPath != "" {
		cfgAns, cfgAgent, err := LoadConfig(opts.ConfigPath)
		if err != nil {
			return 1, err
		}
		mergeAnswers(&state.Answers, cfgAns)
		if cfgAgent != "" && opts.Agent == "" {
			state.Agent = cfgAgent
		}
	}
	// In non-interactive mode the verb is a noop without stub answers
	// for the wiki name (Step 2 prompts when blank). Use the repo dir
	// name as the deterministic fallback so the run completes without
	// stdin.
	if opts.NonInteractive && state.Answers.WikiName == "" {
		state.Answers.WikiName = filepath.Base(opts.RepoRoot)
	}
	if opts.NonInteractive && state.Answers.Theme == "" {
		state.Answers.Theme = "hugo-book"
	}
	if opts.NonInteractive && state.Answers.Privacy == "" {
		state.Answers.Privacy = "none"
	}
	if opts.NonInteractive && state.Answers.Domain == "" {
		state.Answers.Domain = "other"
	}

	if err := SaveState(opts.RepoRoot, state); err != nil {
		return 1, err
	}

	deps.Ctx = context.Background()

	steps := DefaultSteps()
	rc := 0
	for _, step := range steps {
		id := step.ID()
		if skipSet[id] {
			state.UpsertStep(id, StatusSkipped, "skipped via --skip-step", opts.Now)
			_ = SaveState(opts.RepoRoot, state)
			fmt.Fprintf(deps.Stdout, "INIT|skip|%s\n", id)
			continue
		}
		if opts.Continue && state.IsApplied(id) {
			fmt.Fprintf(deps.Stdout, "INIT|resume|%s already applied\n", id)
			continue
		}
		fmt.Fprintf(deps.Stdout, "INIT|step|%s|%s\n", id, step.Description())
		res, err := step.Execute(deps, &state.Answers)
		if err != nil {
			state.UpsertStep(id, StatusFailed, err.Error(), opts.Now)
			_ = SaveState(opts.RepoRoot, state)
			fmt.Fprintf(deps.Stderr, "INIT|fail|%s|%v\n", id, err)
			return 1, err
		}
		status := res.Status
		if status == "" {
			status = StatusApplied
		}
		state.UpsertStep(id, status, res.Note, opts.Now)
		if err := SaveState(opts.RepoRoot, state); err != nil {
			return 1, err
		}
		fmt.Fprintf(deps.Stdout, "INIT|%s|%s|%s\n", status, id, res.Note)
	}
	fmt.Fprintln(deps.Stdout, "INIT|done")
	return rc, nil
}

// mergeAnswers overwrites zero fields in dst with non-zero fields
// from src. Empty strings + false bools are treated as "unset" for
// the purposes of merge.
func mergeAnswers(dst *Answers, src Answers) {
	if src.Domain != "" {
		dst.Domain = src.Domain
	}
	if src.WikiName != "" {
		dst.WikiName = src.WikiName
	}
	if src.Purpose != "" {
		dst.Purpose = src.Purpose
	}
	if src.Privacy != "" {
		dst.Privacy = src.Privacy
	}
	if src.TrackProcessed {
		dst.TrackProcessed = true
	}
	if src.Theme != "" {
		dst.Theme = src.Theme
	}
	if src.PublishLog {
		dst.PublishLog = true
	}
	if src.WireQmdMCP {
		dst.WireQmdMCP = true
	}
	if src.WireAwikiMCP {
		dst.WireAwikiMCP = true
	}
	if src.StageCommit {
		dst.StageCommit = true
	}
}
