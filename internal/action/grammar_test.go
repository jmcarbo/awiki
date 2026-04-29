package action

import "testing"

func TestParseSimpleOpen(t *testing.T) {
	a, ok := ParseLine("- [ ] write spec @home due:2026-05-01 ^abc123")
	if !ok {
		t.Fatal("expected match")
	}
	if a.Status != " " {
		t.Fatalf("status: %q", a.Status)
	}
	if a.Text != "write spec" {
		t.Fatalf("text: %q", a.Text)
	}
	if a.Context != "@home" {
		t.Fatalf("context: %q", a.Context)
	}
	if a.TailKeys["due"] != "2026-05-01" {
		t.Fatalf("due: %q", a.TailKeys["due"])
	}
	if a.ID != "abc123" {
		t.Fatalf("id: %q", a.ID)
	}
}

func TestParseRejectsDoubleCheckbox(t *testing.T) {
	if _, ok := ParseLine("- [ ] [ ] doubled"); ok {
		t.Fatal("expected reject")
	}
}

func TestParseBadDate(t *testing.T) {
	a, ok := ParseLine("- [ ] task due:not-a-date ^abc123")
	if !ok {
		t.Fatal("expected match")
	}
	if a.BadDate == "" {
		t.Fatal("expected BadDate to be set")
	}
}
