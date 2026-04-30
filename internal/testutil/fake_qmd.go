package testutil

import (
	"context"

	"awiki/internal/adapters"
)

// FakeQmd is a test double for adapters.Qmd. Tests configure separate
// codes for Search vs Reindex; the bookkeep flow uses Reindex only.
type FakeQmd struct {
	SearchOutput string
	SearchCode   int
	SearchErr    error

	ReindexOutput string
	ReindexCode   int
	ReindexErr    error

	SearchCalls  int
	ReindexCalls int
}

func (f *FakeQmd) Search(_ context.Context, _, _ string) (string, int, error) {
	f.SearchCalls++
	return f.SearchOutput, f.SearchCode, f.SearchErr
}

func (f *FakeQmd) Reindex(_ context.Context, _ string) (string, int, error) {
	f.ReindexCalls++
	return f.ReindexOutput, f.ReindexCode, f.ReindexErr
}

// Compile-time interface check.
var _ adapters.Qmd = (*FakeQmd)(nil)
