package template

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BootstrapStepGit captures the git operations the rerun-bootstrap-step
// driver needs. The bash oracle (scripts/template-update.sh, the
// `--rerun-bootstrap-step` block) drives:
//   - `git for-each-ref refs/heads/awiki-template-update/` (presence test)
//   - `git checkout -b <branch>`
//   - `git checkout <default>` (rollback on user decline)
//   - `git branch -D <branch>` (cleanup on user decline)
//   - `git add -A` + `git diff --cached --quiet` + `git commit -m`
//
// Production wires `os/exec`; tests inject fakes.
type BootstrapStepGit interface {
	// HasUpdateBranch reports whether any `refs/heads/awiki-template-update/`
	// branch currently exists.
	HasUpdateBranch() (bool, error)
	// Checkout shells `git checkout -q <ref>` (no -b).
	Checkout(ref string) error
	// CheckoutNewBranch shells `git checkout -q -b <branch>`.
	CheckoutNewBranch(branch string) error
	// DeleteBranch shells `git branch -D <branch>`.
	DeleteBranch(branch string) error
	// AddAll shells `git add -A`.
	AddAll() error
	// DiffCachedQuiet returns true when the index has no staged changes
	// (mirrors `git diff --cached --quiet`).
	DiffCachedQuiet() (clean bool, err error)
	// Commit shells `git commit -q -m <msg>`.
	Commit(msg string) error
}

// BootstrapStepBash invokes `bash <script>` with a stripped env. The
// bash oracle uses `env -i` to wipe the parent env, then explicitly
// re-exports PATH/HOME/LANG/LC_ALL plus AWIKI_REPO_ROOT. Production
// adapters mirror that exactly.
type BootstrapStepBash interface {
	// RunBashScript runs `bash <script>` with env strictly equal to env
	// (slice of "K=V" pairs). Returns the exit code; a non-nil error
	// signals the binary is missing.
	RunBashScript(script string, env []string) (code int, err error)
}

// BootstrapStepConfirm prompts the user "Run? [y/N]" and reports the
// answer. Production reads stdin; tests inject a fake.
type BootstrapStepConfirm interface {
	Confirm(prompt string) (yes bool, err error)
}

// BootstrapStepInput bundles every input RerunBootstrapStep needs.
type BootstrapStepInput struct {
	RepoRoot       string
	StepID         string
	NonInteractive bool
	// DefaultBranch is the branch name to return to when the user
	// declines. Bash reads `template-config.sh get .awiki/config
	// default_branch main`. Callers resolve this before calling.
	DefaultBranch string
	// Now is overrideable for tests so the rerun branch name is
	// deterministic. Production uses time.Now().Unix().
	Now func() int64
	// PathEnv / HomeEnv / LangEnv / LCAllEnv mirror the env-stripped
	// values bash re-exports. Empty strings fall back to package
	// defaults (PATH=/usr/bin:/bin; LANG=LC_ALL=C.UTF-8).
	PathEnv  string
	HomeEnv  string
	LangEnv  string
	LCAllEnv string
}

// RerunBootstrapStepResult captures the post-run state for callers /
// test assertions.
type RerunBootstrapStepResult struct {
	// Declined is true when the user said "no" at the confirmation
	// prompt (interactive mode only).
	Declined bool
	// Branch is the rerun branch that was created. Empty when Declined
	// (the branch has been deleted).
	Branch string
	// StepBody is the raw body that was surfaced (and, unless declined,
	// executed).
	StepBody string
	// NewHash is the new content_hash recorded in template.json.
	NewHash string
}

// RerunBootstrapStep mirrors the `--rerun-bootstrap-step <id>` block of
// scripts/template-update.sh. It assumes the caller has already resolved
// the repo root + default branch + interactivity flag.
//
// Returns ErrUpdateInProgress if a state file or update branch is
// present, or pending prompts exist.
func RerunBootstrapStep(g BootstrapStepGit, b BootstrapStepBash, c BootstrapStepConfirm, in BootstrapStepInput) (*RerunBootstrapStepResult, error) {
	if g == nil {
		return nil, errors.New("bootstrap-step: git adapter is nil")
	}
	if b == nil {
		return nil, errors.New("bootstrap-step: bash adapter is nil")
	}
	repoRoot := in.RepoRoot
	statePath := filepath.Join(repoRoot, ".awiki", "template-cache", "_fetch", ".update-state.json")
	if _, err := os.Stat(statePath); err == nil {
		return nil, ErrUpdateInProgress
	}
	if has, err := pendingPromptsPresent(repoRoot); err != nil {
		return nil, err
	} else if has {
		return nil, ErrPendingPrompts
	}
	hasBranch, err := g.HasUpdateBranch()
	if err != nil {
		return nil, err
	}
	if hasBranch {
		return nil, ErrUpdateBranchPresent
	}

	bootstrapMD := filepath.Join(repoRoot, "BOOTSTRAP.md")
	if _, err := os.Stat(bootstrapMD); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("BOOTSTRAP.md not found at repo root")
		}
		return nil, err
	}

	body, err := BootstrapStepBodyFile(bootstrapMD, in.StepID)
	if err != nil {
		return nil, err
	}

	now := in.Now
	if now == nil {
		now = func() int64 { return nowUnix() }
	}
	branch := fmt.Sprintf("awiki-template-update/rerun-%s-%d", in.StepID, now())

	if err := g.CheckoutNewBranch(branch); err != nil {
		return nil, err
	}

	res := &RerunBootstrapStepResult{Branch: branch, StepBody: body}

	if !in.NonInteractive {
		if c == nil {
			return nil, errors.New("bootstrap-step: confirm adapter is nil")
		}
		yes, err := c.Confirm(fmt.Sprintf("Step '%s':\n%sRun? [y/N] ", in.StepID, body))
		if err != nil {
			return nil, err
		}
		if !yes {
			res.Declined = true
			defaultBranch := in.DefaultBranch
			if defaultBranch == "" {
				defaultBranch = "main"
			}
			// Best-effort cleanup; mirror bash's `|| true`.
			_ = g.Checkout(defaultBranch)
			_ = g.DeleteBranch(branch)
			res.Branch = ""
			return res, nil
		}
	}

	bodyFile, err := writeTempBody(body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(bodyFile) }()

	env := buildBootstrapStepEnv(repoRoot, in)
	if _, err := b.RunBashScript(bodyFile, env); err != nil {
		// Bash errors propagate up; the bash oracle ignores the exit
		// code (`|| true`) but a missing-binary error is fatal.
		// Distinguish by err == nil vs non-nil; here err != nil means
		// exec failure.
		return nil, err
	}

	// Compute the new content_hash from the (current) BOOTSTRAP.md.
	newHash, err := HashBootstrapStepFile(bootstrapMD, in.StepID)
	if err != nil {
		return nil, err
	}
	res.NewHash = newHash

	pj := filepath.Join(repoRoot, ".awiki", "template.json")
	if err := upsertBootstrapStepDone(pj, in.StepID, newHash); err != nil {
		return nil, err
	}

	if err := g.AddAll(); err != nil {
		return nil, err
	}
	clean, err := g.DiffCachedQuiet()
	if err != nil {
		return nil, err
	}
	if !clean {
		if err := g.Commit(fmt.Sprintf("chore(template): re-run bootstrap step %s", in.StepID)); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// upsertBootstrapStepDone updates the matching bootstrap_steps_done
// entry in template.json (or appends one) to status="applied" with the
// new content_hash. Mirrors the inline Python heredoc in the bash
// oracle.
func upsertBootstrapStepDone(pj, stepID, newHash string) error {
	data, err := os.ReadFile(pj)
	if err != nil {
		return err
	}
	var d map[string]any
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}
	stepsRaw, _ := d["bootstrap_steps_done"].([]any)
	found := false
	for i, sRaw := range stepsRaw {
		s, ok := sRaw.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := s["id"].(string); id == stepID {
			s["content_hash"] = newHash
			s["status"] = "applied"
			stepsRaw[i] = s
			found = true
			break
		}
	}
	if !found {
		stepsRaw = append(stepsRaw, map[string]any{
			"id":           stepID,
			"status":       "applied",
			"content_hash": newHash,
		})
	}
	d["bootstrap_steps_done"] = stepsRaw
	out, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(pj, out, 0o644)
}

func writeTempBody(body string) (string, error) {
	f, err := os.CreateTemp("", "awiki-bootstrap-body-*.sh")
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(body); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func buildBootstrapStepEnv(repoRoot string, in BootstrapStepInput) []string {
	pathEnv := in.PathEnv
	if pathEnv == "" {
		pathEnv = "/usr/bin:/bin"
	}
	homeEnv := in.HomeEnv
	langEnv := in.LangEnv
	if langEnv == "" {
		langEnv = "C.UTF-8"
	}
	lcEnv := in.LCAllEnv
	if lcEnv == "" {
		lcEnv = "C.UTF-8"
	}
	env := []string{
		"PATH=" + pathEnv,
		"HOME=" + homeEnv,
		"LANG=" + langEnv,
		"LC_ALL=" + lcEnv,
		"AWIKI_REPO_ROOT=" + repoRoot,
	}
	return env
}

func pendingPromptsPresent(repoRoot string) (bool, error) {
	dir := filepath.Join(repoRoot, ".awiki", "pending-prompts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			return true, nil
		}
	}
	return false, nil
}

// nowUnix is overrideable in tests.
var nowUnix = func() int64 { return Now().Unix() }

// Sentinel errors surfaced by RerunBootstrapStep so the CLI can map to
// the bash oracle's exit codes (1) and stderr lines.
var (
	ErrUpdateInProgress    = errors.New("update in progress — run --abort first")
	ErrPendingPrompts      = errors.New("pending-prompts present — resolve first")
	ErrUpdateBranchPresent = errors.New("existing update branch present — finish or --abort first")
)

// IsBootstrapStepHalt returns true when err is one of the halt sentinels
// (used by callers to map to "halt: ..." stderr + exit 1).
func IsBootstrapStepHalt(err error) bool {
	for _, e := range []error{ErrUpdateInProgress, ErrPendingPrompts, ErrUpdateBranchPresent} {
		if errors.Is(err, e) {
			return true
		}
	}
	return strings.Contains(fmt.Sprintf("%v", err), "BOOTSTRAP.md not found")
}
