package adapters

import (
	"context"
	"errors"
	"os/exec"
)

// ExecQmd is the production Qmd adapter. It shells to the `qmd`
// binary on PATH. Tests inject a fake.
type ExecQmd struct{}

func (ExecQmd) Search(ctx context.Context, repoRoot, query string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "qmd", "search", "--", query)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode(), err
	}
	return string(out), 127, err
}

// Reindex shells `bash scripts/qmd-index.sh` so the awiki reindex
// contract (QMD-INDEX|ok / QMD-INDEX|skip records, content/ existence
// check, qmd v0.5.0 collection-rebuild workaround) runs the same way
// the bash bookkeep flow invokes it (scripts/ingest.sh:103).
//
// Script lookup mirrors the bash form's `bash "$SCRIPT_DIR/qmd-index.sh"`,
// where SCRIPT_DIR is the directory containing ingest.sh — which may
// differ from the data repo (`AWIKI_REPO_ROOT`) when the binary is
// invoked via the shim from a separate working directory (e.g. bats
// tests cd into a temp $WORK before exec'ing the Go binary). Resolution
// order:
//
//  1. `<repoRoot>/scripts/qmd-index.sh` (data repo carries the script)
//  2. `<binDir>/../scripts/qmd-index.sh` (binary's source repo)
//  3. fall back to a relative path so a packager that wired things
//     through `cwd` still works.
//
// The script's CWD is the data repo (matches bash's `cd "$REPO_ROOT"`).
func (ExecQmd) Reindex(ctx context.Context, repoRoot string) (string, int, error) {
	script := resolveScript(repoRoot, "qmd-index.sh")
	cmd := exec.CommandContext(ctx, "bash", script)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode(), err
	}
	return string(out), 127, err
}
