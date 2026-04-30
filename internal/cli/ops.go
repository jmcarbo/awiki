// Package cli ops dispatcher for the operational verbs (log, reindex,
// rename, delete, update-catalog, scan, recur, agenda, review,
// check-deps). Each verb is a thin driver around an internal/ops/ helper.
package cli

import (
	"io"

	"awiki/internal/ops"
)

// runLog dispatches `awiki log <action> [message...]`. Mirrors
// scripts/log-append.sh.
func runLog(args []string, stdout, stderr io.Writer) int {
	return ops.LogCLI(args, stdout, stderr)
}
