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
	got := AgentPrompt("Process the source at raw/inbox/foo.md per WIKI.md §4.1")
	want := "AGENT-PROMPT|Process the source at raw/inbox/foo.md per WIKI.md §4.1"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestReviewRecordWithFields(t *testing.T) {
	got := Review("projects-no-next-action", []string{"count=3", "slugs=foo,bar"})
	want := "REVIEW|projects-no-next-action|count=3|slugs=foo,bar"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestReviewRecordEmptyFields(t *testing.T) {
	got := Review("inbox-unprocessed", nil)
	want := "REVIEW|inbox-unprocessed"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
