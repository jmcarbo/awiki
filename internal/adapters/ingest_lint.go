package adapters

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

// IngestLint shells the repo's lint script for the auto-lint phase of
// the ingest bookkeep flow (scripts/ingest.sh:84-98). When the
// AWIKI_LINT_CMD env var is set on the supplied env (or in the process
// environment when env is nil), the adapter runs that command via
// `sh -c` (mirroring bash's `eval "$AWIKI_LINT_CMD"` test/override
// hook). Otherwise it shells `bash <repoRoot>/scripts/lint.sh`.
//
// The bash form runs the command in a subshell so an `exit` in the
// override cannot terminate the parent script. The Go adapter mirrors
// that — the spawned process gets its own pgroup and a non-zero exit
// is returned as a numeric code rather than a panic.
type IngestLint interface {
	Run(ctx context.Context, repoRoot string, env []string) (code int, err error)
}

// ExecIngestLint is the production adapter.
type ExecIngestLint struct{}

func (ExecIngestLint) Run(ctx context.Context, repoRoot string, env []string) (int, error) {
	// Resolve AWIKI_LINT_CMD: prefer the caller-supplied env, fall
	// back to the process env. nil env means "inherit".
	override := lookupEnv(env, "AWIKI_LINT_CMD")
	if override == "" {
		override = os.Getenv("AWIKI_LINT_CMD")
	}

	var cmd *exec.Cmd
	if override != "" {
		cmd = exec.CommandContext(ctx, "sh", "-c", override)
	} else {
		cmd = exec.CommandContext(ctx, "bash", resolveScript(repoRoot, "lint.sh"))
	}
	cmd.Dir = repoRoot
	if env != nil {
		cmd.Env = env
	}
	// Inherit stdout/stderr — the bash form lets lint output flow to
	// the user's terminal. Tests can intercept by setting cmd.Stdout
	// in a wrapper, but for v1 the production path stays simple.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), err
	}
	return 127, err
}

// lookupEnv returns the value of key in a slice of "KEY=VALUE" entries.
// Returns "" when the key is absent. Mirrors os.Getenv's miss semantics.
func lookupEnv(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if len(e) > len(prefix) && e[:len(prefix)] == prefix {
			return e[len(prefix):]
		}
	}
	return ""
}
