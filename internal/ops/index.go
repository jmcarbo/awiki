package ops

import (
	"context"
	"fmt"
	"io"

	"awiki/internal/adapters"
)

// ReindexCLI runs `awiki reindex`. Mirrors scripts/qmd-index.sh contract:
//   - emits "QMD-INDEX|ok" on stdout on success.
//   - emits "QMD-INDEX|skip|reason=<reason>" on stderr (exit 0) when qmd
//     is missing or content/ is absent. Skipping is not a failure.
//
// Implementation: delegate to the adapters.Qmd.Reindex shim, which
// shells the bash script under the script-resolver. This keeps the
// Go binary's contract identical to the bash form (collection rebuild,
// upstream constraint-bug workaround, etc.) without re-deriving the
// SQLite handling in Go.
func ReindexCLI(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: awiki reindex")
		return 2
	}
	return Reindex(adapters.ExecQmd{}, ".", stdout, stderr)
}

// Reindex executes the reindex against repoRoot using the supplied
// Qmd adapter. Exposed for testing with a fake adapter.
func Reindex(qmd adapters.Qmd, repoRoot string, stdout, stderr io.Writer) int {
	out, code, _ := qmd.Reindex(context.Background(), repoRoot)
	// The bash script prints `QMD-INDEX|ok` to stdout, `QMD-INDEX|skip|...`
	// to stderr. CombinedOutput merges them; route on the prefix.
	for _, line := range splitLines(out) {
		if line == "" {
			continue
		}
		switch {
		case startsWith(line, "QMD-INDEX|skip"):
			fmt.Fprintln(stderr, line)
		default:
			fmt.Fprintln(stdout, line)
		}
	}
	return code
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
