package testutil

import (
	"context"

	"awiki/internal/adapters"
)

// FakeIngestLint is a test double for adapters.IngestLint. It records
// every Run invocation and returns a configurable exit code/error,
// letting tests pin the bookkeep verb's auto-lint branch without
// shelling out to scripts/lint.sh.
type FakeIngestLint struct {
	// Code is the exit code returned by Run. 0 means success;
	// >= 2 forces the bookkeep flow to set LINT_RC=4 (matches
	// scripts/ingest.sh:94).
	Code int
	// Err is the error returned to the caller. Mirrors what os/exec
	// would surface on a non-zero exit.
	Err error

	// Calls counts the number of Run invocations. Tests assert on
	// Calls instead of mocking PATH so the auto-lint branch is
	// independently verifiable.
	Calls int
	// LastRepoRoot / LastEnv capture the args passed to the most
	// recent Run call.
	LastRepoRoot string
	LastEnv      []string
}

// Run records the call and returns the configured Code/Err.
func (f *FakeIngestLint) Run(_ context.Context, repoRoot string, env []string) (int, error) {
	f.Calls++
	f.LastRepoRoot = repoRoot
	f.LastEnv = env
	return f.Code, f.Err
}

// Compile-time interface check.
var _ adapters.IngestLint = (*FakeIngestLint)(nil)
