package adapters

import "context"

// Hugo runs the Hugo build/check pipeline.
type Hugo interface {
	Check(ctx context.Context, repoRoot string) (output string, code int, err error)
}

// Qmd is the qmd CLI adapter (search/index/install).
type Qmd interface {
	Search(ctx context.Context, repoRoot, query string) (output string, code int, err error)
	Reindex(ctx context.Context, repoRoot string) (output string, code int, err error)
}

// DuckDB runs an SQL query and returns rows as TSV/JSON depending on
// the caller-supplied format string. Used by the query domain.
type DuckDB interface {
	Run(ctx context.Context, sql, format string) (output string, code int, err error)
}

// VLConvert renders a Vega-Lite spec to SVG. Used by the chart domain.
type VLConvert interface {
	RenderSVG(ctx context.Context, specPath, outPath string) (code int, err error)
}

// VendorVega vendors the Vega JS libraries. Used by the chart domain.
type VendorVega interface {
	Ensure(ctx context.Context, repoRoot string) error
}

// PDFToText extracts text from a PDF. Used by the ingest-pdf flow.
type PDFToText interface {
	Extract(ctx context.Context, pdfPath string) (text string, code int, err error)
}

// Whisper transcribes audio. Used by the ingest-audio flow.
type Whisper interface {
	Transcribe(ctx context.Context, audioPath string) (text string, code int, err error)
}

// Git wraps the git CLI for operations not covered by go-git.
type Git interface {
	Clone(ctx context.Context, url, dest string) (code int, err error)
	RevParse(ctx context.Context, dir, ref string) (sha string, code int, err error)
}

// GPG wraps the gpg/git-crypt/age CLIs for the encrypt-init lifecycle.
type GPG interface {
	Available(ctx context.Context) (bool, error)
}

// FSNotify is the watch-loop adapter; the implementation may use the
// fsnotify package or fall back to polling.
type FSNotify interface {
	Watch(ctx context.Context, root string, events chan<- string) error
}

// --- no-op stubs used solely for compile-time interface assertions ---

type hugoStub struct{}

func (hugoStub) Check(context.Context, string) (string, int, error) { return "", 0, nil }

type qmdStub struct{}

func (qmdStub) Search(context.Context, string, string) (string, int, error) { return "", 0, nil }
func (qmdStub) Reindex(context.Context, string) (string, int, error)        { return "", 0, nil }

type duckdbStub struct{}

func (duckdbStub) Run(context.Context, string, string) (string, int, error) { return "", 0, nil }

type vlconvertStub struct{}

func (vlconvertStub) RenderSVG(context.Context, string, string) (int, error) { return 0, nil }

type pdftotextStub struct{}

func (pdftotextStub) Extract(context.Context, string) (string, int, error) { return "", 0, nil }

type whisperStub struct{}

func (whisperStub) Transcribe(context.Context, string) (string, int, error) { return "", 0, nil }

type xlsxExtractStub struct{}

func (xlsxExtractStub) Slugify(context.Context, string) (string, int, error) { return "", 0, nil }
func (xlsxExtractStub) Extract(context.Context, XLSXExtractOptions) (string, int, error) {
	return "", 0, nil
}
func (xlsxExtractStub) CheckDep(context.Context) (bool, error) { return false, nil }

type gitStub struct{}

func (gitStub) Clone(context.Context, string, string) (int, error)            { return 0, nil }
func (gitStub) RevParse(context.Context, string, string) (string, int, error) { return "", 0, nil }

type gpgStub struct{}

func (gpgStub) Available(context.Context) (bool, error) { return false, nil }

type fsnotifyStub struct{}

func (fsnotifyStub) Watch(context.Context, string, chan<- string) error { return nil }

type vendorVegaStub struct{}

func (vendorVegaStub) Ensure(context.Context, string) error { return nil }

type ingestLintStub struct{}

func (ingestLintStub) Run(context.Context, string, []string) (int, error) { return 0, nil }

type agentStub struct{}

func (agentStub) Run(context.Context, string, string, string) (int, error) { return 0, nil }

type logAppendStub struct{}

func (logAppendStub) Append(context.Context, string, string, string) error { return nil }
