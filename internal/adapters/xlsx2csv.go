package adapters

import "context"

// XLSX2CSV shells `xlsx2csv` to expand an `.xlsx` workbook into per-sheet
// CSV files. flags carries verb-specific pass-through arguments; the
// real implementation lands in the ingest-xlsx slice.
type XLSX2CSV interface {
	Convert(ctx context.Context, srcPath, destDir string, flags []string) (output string, code int, err error)
}

// ExecXLSX2CSV is the production adapter. The stub method returns
// ErrNotImplemented.
type ExecXLSX2CSV struct{}

func (ExecXLSX2CSV) Convert(_ context.Context, _, _ string, _ []string) (string, int, error) {
	return "", 0, ErrNotImplemented
}
