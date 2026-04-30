package testutil

import (
	"context"

	"awiki/internal/adapters"
)

// FakeLogAppend is a test double for adapters.LogAppend. It records
// each Append invocation but performs no I/O.
type FakeLogAppend struct {
	Calls   int
	Actions []string
	Msgs    []string
}

func (f *FakeLogAppend) Append(_ context.Context, _ string, action, msg string) error {
	f.Calls++
	f.Actions = append(f.Actions, action)
	f.Msgs = append(f.Msgs, msg)
	return nil
}

// Compile-time interface check.
var _ adapters.LogAppend = (*FakeLogAppend)(nil)
