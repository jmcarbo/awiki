package adapters

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// Reindex re-indexes the awiki content/ tree with qmd. Native Go port
// of the deleted scripts/qmd-index.sh.
//
// Contract:
//   - Emit `QMD-INDEX|ok` on stdout on success.
//   - Emit `QMD-INDEX|skip|reason=<reason>` on stderr and return code 0
//     if qmd is not installed or the content/ directory is missing.
//
// qmd v0.5.0 has a constraint-failed bug on `qmd update` after a file
// changes; workaround is to remove + re-add the collection per
// reindex.
func (ExecQmd) Reindex(ctx context.Context, repoRoot string) (string, int, error) {
	if _, err := exec.LookPath("qmd"); err != nil {
		return "QMD-INDEX|skip|reason=qmd-not-installed\n", 0, nil
	}
	contentDir := filepath.Join(repoRoot, "content")
	if st, err := os.Stat(contentDir); err != nil || !st.IsDir() {
		return "QMD-INDEX|skip|reason=no-content-dir\n", 0, nil
	}
	qmdDir := filepath.Join(repoRoot, ".qmd")
	if err := os.MkdirAll(qmdDir, 0o755); err != nil {
		return "", 1, err
	}
	indexFile := filepath.Join(qmdDir, "index.sqlite")
	const collection = "awiki"
	contentAbs, err := filepath.Abs(contentDir)
	if err != nil {
		return "", 1, err
	}

	// Remove existing collection if present (workaround).
	listCmd := exec.CommandContext(ctx, "qmd", "--index", indexFile, "collection", "list", "--json")
	listCmd.Dir = repoRoot
	listOut, _ := listCmd.Output()
	if strings.Contains(string(listOut), `"name": "`+collection+`"`) {
		rmCmd := exec.CommandContext(ctx, "qmd", "--index", indexFile, "collection", "remove", collection)
		rmCmd.Dir = repoRoot
		_ = rmCmd.Run()
	}

	addCmd := exec.CommandContext(ctx, "qmd", "--index", indexFile, "collection", "add", contentAbs, "--name", collection)
	addCmd.Dir = repoRoot
	if out, err := addCmd.CombinedOutput(); err != nil {
		return "", 1, fmt.Errorf("qmd collection add: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	updCmd := exec.CommandContext(ctx, "qmd", "--index", indexFile, "update", "-c", collection)
	updCmd.Dir = repoRoot
	if out, err := updCmd.CombinedOutput(); err != nil {
		return "", 1, fmt.Errorf("qmd update: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return "QMD-INDEX|ok\n", 0, nil
}
