package region

import "testing"

func TestParseGeneratedHappyPath(t *testing.T) {
	text := "intro\n<!-- BEGIN GENERATED v1 -->\nbody\n<!-- END GENERATED -->\nfooter\n"
	got, diags := ParseGenerated(text)
	if len(diags) != 0 {
		t.Fatalf("diagnostics: %+v", diags)
	}
	if got.Body != "body\n" {
		t.Fatalf("body: %q", got.Body)
	}
	if got.BeginLine != "<!-- BEGIN GENERATED v1 -->" {
		t.Fatalf("begin line: %q", got.BeginLine)
	}
}

func TestParseGeneratedMissingBegin(t *testing.T) {
	_, diags := ParseGenerated("no markers here\n")
	if len(diags) == 0 {
		t.Fatalf("expected diagnostics for missing markers")
	}
}

func TestParseGeneratedDuplicateBegin(t *testing.T) {
	text := "<!-- BEGIN GENERATED a -->\n<!-- BEGIN GENERATED b -->\n<!-- END GENERATED -->\n"
	_, diags := ParseGenerated(text)
	if len(diags) == 0 {
		t.Fatalf("expected diagnostics for duplicate BEGIN")
	}
}
