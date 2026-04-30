package ingest

import (
	"path/filepath"

	"awiki/internal/adapters"
)

// ExitError is a sentinel error carrying an OS exit code. Ingest verbs
// return it when they want the CLI dispatcher to exit with a specific
// code (e.g. exit 2 for extractor failure, exit 3 for bookkeeping
// failure). Mirrors synth.ExitError.
type ExitError struct {
	Code int
	Msg  string
}

func (e *ExitError) Error() string { return e.Msg }
func (e *ExitError) ExitCode() int { return e.Code }

// Runner is the per-invocation context shared by every ingest verb.
// Adapter fields are wired by the CLI dispatcher (production exec
// adapters) or by tests (fakes). RepoRoot is resolved by the dispatcher
// from AWIKI_REPO_ROOT or the current working directory.
type Runner struct {
	RepoRoot   string
	ContentDir string
	Config     map[string]string

	// External tool adapters. Populated in subsequent slices that wire
	// the corresponding verbs; the infra slice keeps them as bare
	// fields so types.go and runner.go compile against the adapter
	// package without forcing every verb to land at once.
	PDFToText adapters.PDFToText
	Whisper   adapters.Whisper
	XLSX2CSV  adapters.XLSX2CSV
	FSNotify  adapters.FSNotify
	GitExt    adapters.GitExt
	Qmd       adapters.Qmd

	Today string // injected for tests; defaults to time.Now date
}

// InboxDir returns the canonical inbox directory under RepoRoot.
func (r *Runner) InboxDir() string {
	return filepath.Join(r.RepoRoot, "raw", "inbox")
}

// BatchDir returns the watchdog batch directory under RepoRoot.
func (r *Runner) BatchDir() string {
	return filepath.Join(r.InboxDir(), "batch")
}

// ArchiveDir returns the archive sub-directory of the inbox where
// successful ingest moves consumed sources.
func (r *Runner) ArchiveDir() string {
	return filepath.Join(r.InboxDir(), "_archive")
}

// GitStateDir returns `.awiki/git-state/` under RepoRoot. Used by
// `ingest-git` and `ingest-git-list`.
func (r *Runner) GitStateDir() string {
	return filepath.Join(r.RepoRoot, ".awiki", "git-state")
}
