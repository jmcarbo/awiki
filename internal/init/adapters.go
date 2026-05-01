package initverb

import "context"

// AgentRunner shells an external agent CLI (claude, codex, opencode,
// gemini, ...) with a curated prompt and returns the captured stdout.
// Used by Step 7 (patch-identity) for the landing-paragraph generator.
//
// Implementations live in internal/adapters/init_agent.go for
// production and in init_test.go for fakes.
type AgentRunner interface {
	// Run invokes <agentCLI> -p "<prompt>" and returns trimmed stdout.
	// A non-zero exit + captured stderr surface as an error.
	Run(ctx context.Context, agentCLI, prompt string) (string, error)
}

// GitInit captures the narrow set of git operations the init verb
// drives. Each method shells the equivalent `git ...` command from
// repoRoot. Production wires `os/exec`; tests inject fakes.
type GitInit interface {
	// SubmoduleUpdate shells `git submodule update --init --recursive`
	// (Step 0).
	SubmoduleUpdate(ctx context.Context, repoRoot string) (output string, code int, err error)
	// Status shells `git -C <repo> status --short`. Used for Step 3
	// verification + Step 11 stage-commit preview.
	Status(ctx context.Context, repoRoot string) (output string, code int, err error)
	// AddAll shells `git -C <repo> add -A` (Step 11).
	AddAll(ctx context.Context, repoRoot string) (code int, err error)
	// Commit shells `git -C <repo> commit -m <msg>` (Step 11).
	Commit(ctx context.Context, repoRoot, msg string) (output string, code int, err error)
	// SubmoduleDeinit / SubmoduleRm / SubmoduleAdd back the Step 5
	// theme-switch chain. Each runs `git -C <repo> submodule <verb>`.
	SubmoduleDeinit(ctx context.Context, repoRoot, path string) (code int, err error)
	SubmoduleRm(ctx context.Context, repoRoot, path string) (code int, err error)
	SubmoduleAdd(ctx context.Context, repoRoot, url, path string) (code int, err error)
	// RevParseHEAD returns the current HEAD SHA (used by Step 12).
	RevParseHEAD(ctx context.Context, repoRoot string) (sha string, err error)
}

// BashRunner shells a bash script with the supplied argv. Used by
// Step 3 (encrypt-init.sh). Production wires `bash <script> [args...]`
// inheriting stdio; tests inject fakes.
type BashRunner interface {
	Run(ctx context.Context, repoRoot, script string, args ...string) (output string, code int, err error)
}

// NPMRunner shells `npm install --silent` in dir. Used by Step 9b.
type NPMRunner interface {
	Install(ctx context.Context, dir string) (output string, code int, err error)
}
