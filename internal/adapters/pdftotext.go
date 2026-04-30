package adapters

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

// ErrNotImplemented is returned by adapter stubs that have not yet been
// wired to their real exec backend. Subsequent ingest slices replace
// the stub method bodies with real `os/exec` calls.
var ErrNotImplemented = errors.New("adapter: not implemented")

// ExecPDFToText shells `pdftotext` to extract plain text from a PDF.
// Mirrors the bash invocation in scripts/ingest-pdf.sh:17 —
//
//	pdftotext -layout "$PDF" -
//
// (the trailing `-` writes the text body to stdout). The bash script
// also probes for `marker` as a fallback. We do not reproduce the
// fallback here: parity with the v1 ingest-pdf path is sufficient, and
// callers that want marker can either install pdftotext or set
// AWIKI_INGEST_PDF_LEGACY=1 to fall back to the bash script (which
// keeps the marker branch).
type ExecPDFToText struct{}

// Extract runs `pdftotext -layout <pdfPath> -` and returns the
// captured stdout as the extracted text. stderr is preserved in the
// returned error when the binary exits non-zero so callers can surface
// the underlying message (matches the conventions of
// adapters.ExecDuckDB / adapters.ExecVLConvert).
func (ExecPDFToText) Extract(ctx context.Context, pdfPath string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", pdfPath, "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		msg := stderr.String()
		if msg == "" {
			msg = stdout.String()
		}
		return stdout.String(), exitErr.ExitCode(), errors.New(msg)
	}
	return "", 127, err
}
