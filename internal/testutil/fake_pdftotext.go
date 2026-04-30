package testutil

import (
	"context"
	"os"

	"awiki/internal/adapters"
)

// FakePDFToText is a test double for adapters.PDFToText. It returns
// canned text from an on-disk fixture path, ignoring the actual PDF
// contents. Tests inject it into ingest.Runner.PDFToText to drive
// the ingest-pdf verb without requiring a real pdftotext binary.
//
// One of TextPath or Text must be set. TextPath wins when both are
// non-empty; the file is read on every Extract call so fixtures can
// share the on-disk test tree. Code/Err let tests simulate adapter
// failures.
type FakePDFToText struct {
	TextPath string
	Text     string
	Code     int
	Err      error
}

// Extract returns the canned text. The pdfPath argument is ignored —
// the fake exists precisely so tests need not provide a real PDF.
func (f *FakePDFToText) Extract(_ context.Context, _ string) (string, int, error) {
	if f.Err != nil || f.Code != 0 {
		return f.Text, f.Code, f.Err
	}
	if f.TextPath != "" {
		data, err := os.ReadFile(f.TextPath)
		if err != nil {
			return "", 1, err
		}
		return string(data), 0, nil
	}
	return f.Text, 0, nil
}

// Compile-time interface check.
var _ adapters.PDFToText = (*FakePDFToText)(nil)
