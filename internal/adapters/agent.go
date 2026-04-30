package adapters

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// Agent shells the configured agent CLI with the AGENT-PROMPT line.
// Mirrors scripts/ingest.sh:121-137. The bash form uses a per-CLI
// argv shape:
//
//	claude              -> "$AGENT_CLI" $AWIKI_AGENT_FLAGS --print "$PROMPT"
//	codex|opencode|gemini -> "$AGENT_CLI" $AWIKI_AGENT_FLAGS -p "$PROMPT"
//	*                    -> "$AGENT_CLI" $AWIKI_AGENT_FLAGS "$PROMPT"
//
// flags is the value of AWIKI_AGENT_FLAGS — word-split on whitespace
// (mirroring bash's unquoted ${AWIKI_AGENT_FLAGS:-} expansion).
type Agent interface {
	Run(ctx context.Context, cli, flags, prompt string) (code int, err error)
}

// ExecAgent is the production adapter.
type ExecAgent struct{}

func (ExecAgent) Run(ctx context.Context, cli, flags, prompt string) (int, error) {
	args := splitFlags(flags)
	switch cli {
	case "claude":
		args = append(args, "--print", prompt)
	case "codex", "opencode", "gemini":
		args = append(args, "-p", prompt)
	default:
		args = append(args, prompt)
	}
	cmd := exec.CommandContext(ctx, cli, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
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

// splitFlags performs the same word-splitting bash applies to an
// unquoted variable expansion: split on runs of whitespace, drop empty
// fields. Mirrors `${AWIKI_AGENT_FLAGS:-}` behavior at
// scripts/ingest.sh:130-132.
func splitFlags(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Fields(s)
}
