package synth

import "testing"

func TestParseGeneratedRegionValid(t *testing.T) {
	text := "# Page\n\n<!-- BEGIN GENERATED scope_hash=abc -->\n## TL;DR\nbody\n<!-- END GENERATED -->\n"

	region, diagnostics := ParseGeneratedRegion(text)

	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	if got, want := region.Body, "## TL;DR\nbody\n"; got != want {
		t.Fatalf("Body = %q, want %q", got, want)
	}
	if region.BeginOffset < 0 || region.EndOffset <= region.BeginOffset {
		t.Fatalf("unexpected offsets: %#v", region)
	}
}

func TestParseGeneratedRegionDoubleBegin(t *testing.T) {
	text := "<!-- BEGIN GENERATED -->\nfirst\n<!-- BEGIN GENERATED -->\nsecond\n<!-- END GENERATED -->\n"

	_, diagnostics := ParseGeneratedRegion(text)

	assertRegionDiagnostic(t, diagnostics, "S1", "expected exactly one BEGIN GENERATED marker")
}

func TestParseGeneratedRegionMissingEnd(t *testing.T) {
	text := "<!-- BEGIN GENERATED -->\nbody\n"

	_, diagnostics := ParseGeneratedRegion(text)

	assertRegionDiagnostic(t, diagnostics, "S1", "expected exactly one END GENERATED marker")
}

func TestParseGeneratedRegionBeginAfterEnd(t *testing.T) {
	text := "<!-- END GENERATED -->\nbody\n<!-- BEGIN GENERATED -->\n"

	_, diagnostics := ParseGeneratedRegion(text)

	assertRegionDiagnostic(t, diagnostics, "S1", "BEGIN GENERATED marker must appear before END GENERATED marker")
}

func assertRegionDiagnostic(t *testing.T, diagnostics []RegionDiagnostic, code string, message string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code && diagnostic.Message == message {
			return
		}
	}
	t.Fatalf("missing diagnostic %s %q in %#v", code, message, diagnostics)
}
