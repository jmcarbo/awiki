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
	t.Log("compile-time interface check ok")
}
