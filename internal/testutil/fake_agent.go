package testutil

import (
	"context"

	"awiki/internal/adapters"
)

// FakeAgent is a test double for adapters.Agent. It records every Run
// invocation and returns a configurable exit code/error so the
// bookkeep flow's --agent branch can be exercised without a real
// agent CLI.
type FakeAgent struct {
	Code int
	Err  error

	Calls      int
	LastCLI    string
	LastFlags  string
	LastPrompt string
}

// Run records the call and returns the configured Code/Err.
func (f *FakeAgent) Run(_ context.Context, cli, flags, prompt string) (int, error) {
	f.Calls++
	f.LastCLI = cli
	f.LastFlags = flags
	f.LastPrompt = prompt
	return f.Code, f.Err
}

// Compile-time interface check.
var _ adapters.Agent = (*FakeAgent)(nil)
