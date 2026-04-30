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

// runReindex dispatches `awiki reindex`. Mirrors scripts/qmd-index.sh.
func runReindex(args []string, stdout, stderr io.Writer) int {
	return ops.ReindexCLI(args, stdout, stderr)
}

// runCheckDeps dispatches `awiki check-deps`. Mirrors
// scripts/check-deps.sh.
func runCheckDeps(args []string, stdout, stderr io.Writer) int {
	return ops.CheckDepsCLI(args, stdout, stderr)
}

// runRename dispatches `awiki rename <old> <new>`. Mirrors
// scripts/rename.sh.
func runRename(args []string, stdout, stderr io.Writer) int {
	return ops.RenameCLI(args, stdout, stderr)
}

// runDelete dispatches `awiki delete <slug>`. Mirrors
// scripts/delete-page.sh.
func runDelete(args []string, stdout, stderr io.Writer) int {
	return ops.DeleteCLI(args, stdout, stderr)
}

// runUpdateCatalog dispatches `awiki update-catalog`. Mirrors
// scripts/update-catalog.sh.
func runUpdateCatalog(args []string, stdout, stderr io.Writer) int {
	return ops.UpdateCatalogCLI(args, stdout, stderr)
}

// runScan dispatches `awiki scan`. Mirrors scripts/action-scan.sh.
func runScan(args []string, stdout, stderr io.Writer) int {
	return ops.ScanCLI(args, stdout, stderr)
}

// runRecur dispatches `awiki recur` and `awiki recur-dry`. Mirrors
// scripts/action-recur.sh.
func runRecur(dry bool, args []string, stdout, stderr io.Writer) int {
	return ops.RecurCLI(dry, args, stdout, stderr)
}
