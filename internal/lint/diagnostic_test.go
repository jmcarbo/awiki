package lint

import (
	"strings"
	"testing"
)

func TestDiagnosticRecordFormat(t *testing.T) {
	d := Diagnostic{Level: Error, File: "content/entities/foo.md", Message: "broken wikilink: [[bar]]"}
	if got, want := d.Record(), "LINT|ERROR|content/entities/foo.md|broken wikilink: [[bar]]"; got != want {
		t.Fatalf("Record() = %q, want %q", got, want)
	}
}

func TestSummaryAndExitCode(t *testing.T) {
	var c Collector
	c.Add(Diagnostic{Level: Error, File: "a.md", Message: "bad"})
	c.Add(Diagnostic{Level: Warn, File: "b.md", Message: "warn"})
	c.Add(Diagnostic{Level: Info, File: "c.md", Message: "info"})
	if got, want := c.Summary(), "LINT-SUMMARY|errors=1|warnings=1|info=1"; got != want {
		t.Fatalf("Summary() = %q, want %q", got, want)
	}
	if got := c.ExitCode(); got != 2 {
		t.Fatalf("ExitCode() = %d, want 2", got)
	}
}

func TestWarningOnlyExitCode(t *testing.T) {
	var c Collector
	c.Add(Diagnostic{Level: Warn, File: "b.md", Message: "warn"})
	if got := c.ExitCode(); got != 1 {
		t.Fatalf("ExitCode() = %d, want 1", got)
	}
}

func TestFixRecordFormat(t *testing.T) {
	r := FixRecord{File: "content/entities/foo.md", Message: "added last_updated: 2026-04-28"}
	if got := r.Record(); !strings.HasPrefix(got, "FIX|content/entities/foo.md|added last_updated:") {
		t.Fatalf("Record() = %q", got)
	}
}
