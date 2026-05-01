package initverb

import (
	"fmt"
	"os"
	"path/filepath"

	"awiki/internal/adapters"
	"awiki/internal/ops"
)

// stepInstallQmd runs Step 8: install qmd. Treats failure as
// non-fatal — when qmd is unavailable the agent falls back to grep
// fallback per WIKI.md. The success path also kicks `awiki reindex`.
type stepInstallQmd struct{}

func (stepInstallQmd) ID() string          { return "install-qmd" }
func (stepInstallQmd) Description() string { return "install qmd" }
func (stepInstallQmd) Kind() Kind          { return KindMechanical }

func (stepInstallQmd) Execute(ctx StepContext, _ *Answers) (Result, error) {
	rc := ops.InstallQmd(ops.InstallQmdOptions{RepoRoot: ctx.RepoRoot}, ctx.Stdout, ctx.Stderr)
	if rc != 0 {
		// Non-fatal: ensure qmd-status reads `missing` and continue.
		statusFile := filepath.Join(ctx.RepoRoot, ".awiki", "qmd-status")
		_ = os.MkdirAll(filepath.Dir(statusFile), 0o755)
		_ = os.WriteFile(statusFile, []byte("missing\n"), 0o644)
		fmt.Fprintln(ctx.Stderr, "install-qmd: qmd unavailable; agent will use grep fallback")
		return Result{Status: StatusApplied, Note: "qmd-status=missing"}, nil
	}
	// On success, reindex. Failure here is also non-fatal.
	rcIdx := ops.Reindex(adapters.ExecQmd{}, ctx.RepoRoot, ctx.Stdout, ctx.Stderr)
	if rcIdx != 0 {
		fmt.Fprintln(ctx.Stderr, "install-qmd: reindex returned non-zero (continuing)")
	}
	return Result{Status: StatusApplied, Note: "qmd-status=ok"}, nil
}

// QmdStatus reads .awiki/qmd-status and returns "ok"/"missing"/"" if
// the file is absent. Used by Step 9a to gate the wire-qmd-mcp prompt.
func QmdStatus(repoRoot string) string {
	path := filepath.Join(repoRoot, ".awiki", "qmd-status")
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, c := range []byte(body) {
		if c == '\n' {
			break
		}
	}
	if len(body) == 0 {
		return ""
	}
	// Strip trailing newline.
	out := string(body)
	for len(out) > 0 && (out[len(out)-1] == '\n' || out[len(out)-1] == '\r') {
		out = out[:len(out)-1]
	}
	return out
}
