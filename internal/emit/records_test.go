package emit

import "testing"

func TestLintRecordWithoutCode(t *testing.T) {
	got := Lint("ERROR", "content/entities/foo.md", "", "broken wikilink: [[bar]]")
	want := "LINT|ERROR|content/entities/foo.md|broken wikilink: [[bar]]"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLintRecordWithCode(t *testing.T) {
	got := Lint("ERROR", "content/datasets/bad.md", "D1", "missing storage")
	want := "LINT|ERROR|content/datasets/bad.md|D1|missing storage"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFixRecord(t *testing.T) {
	got := Fix("content/entities/foo.md", "added last_updated: 2026-04-29")
	want := "FIX|content/entities/foo.md|added last_updated: 2026-04-29"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLintSummary(t *testing.T) {
	got := LintSummary(2, 1, 0)
	want := "LINT-SUMMARY|errors=2|warnings=1|info=0"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestAgentPromptRecord(t *testing.T) {
	got := AgentPrompt("ingest", []string{"path=raw/inbox/foo.md", "kind=entity"})
	want := "AGENT-PROMPT|ingest|path=raw/inbox/foo.md|kind=entity"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestIngestRecord(t *testing.T) {
	got := Ingest("xlsx", "raw/inbox/batch/sales.xlsx", "ok")
	want := "INGEST|xlsx|raw/inbox/batch/sales.xlsx|ok"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestReviewRecord(t *testing.T) {
	got := Review("week", "2026-W17", []string{"open=12", "done=4"})
	want := "REVIEW|week|2026-W17|open=12|done=4"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRecurRecord(t *testing.T) {
	got := Recur("content/inbox.md", "abc123", "due=2026-05-06")
	want := "RECUR|content/inbox.md|abc123|due=2026-05-06"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
