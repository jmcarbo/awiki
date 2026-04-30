package adapters

import "testing"

func TestInterfaceShapes(t *testing.T) {
	var _ Hugo = (*hugoStub)(nil)
	var _ Qmd = (*qmdStub)(nil)
	var _ DuckDB = (*duckdbStub)(nil)
	var _ VLConvert = (*vlconvertStub)(nil)
	var _ VendorVega = (*vendorVegaStub)(nil)
	var _ PDFToText = (*pdftotextStub)(nil)
	var _ Whisper = (*whisperStub)(nil)
	var _ Git = (*gitStub)(nil)
	var _ GPG = (*gpgStub)(nil)
	var _ FSNotify = (*fsnotifyStub)(nil)
	var _ XLSXExtract = (*xlsxExtractStub)(nil)

	// Ingest infra slice exec adapter stubs.
	var _ PDFToText = (*ExecPDFToText)(nil)
	var _ Whisper = (*ExecWhisper)(nil)
	var _ XLSXExtract = (*ExecXLSXExtract)(nil)
	var _ FSNotify = (*ExecFSNotify)(nil)
	var _ GitExt = (*ExecGitExt)(nil)

	// Ingest slice 7 (bookkeep orchestrator) adapter stubs.
	var _ IngestLint = (*ingestLintStub)(nil)
	var _ IngestLint = (*ExecIngestLint)(nil)
	var _ Agent = (*agentStub)(nil)
	var _ Agent = (*ExecAgent)(nil)
	var _ LogAppend = (*logAppendStub)(nil)
	var _ LogAppend = (*ExecLogAppend)(nil)

	t.Log("compile-time interface check ok")
}
