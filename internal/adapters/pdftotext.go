package adapters

import (
	"context"
	"errors"
)

// ErrNotImplemented is returned by adapter stubs that have not yet been
// wired to their real exec backend. Subsequent ingest slices replace
// the stub method bodies with real `os/exec` calls.
var ErrNotImplemented = errors.New("adapter: not implemented")

// ExecPDFToText shells `pdftotext` to extract plain text from a PDF.
// The stub method returns ErrNotImplemented; the real implementation
// lands in the ingest-pdf slice.
type ExecPDFToText struct{}

func (ExecPDFToText) Extract(_ context.Context, _ string) (string, int, error) {
	return "", 0, ErrNotImplemented
}
