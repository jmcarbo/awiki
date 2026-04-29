package query

import (
	"testing"
)

func TestExtractFences_Single(t *testing.T) {
	text := "before\n```awiki-query\nSELECT 1\n```\nafter\n"
	fences := ExtractFences(text)
	if len(fences) != 1 {
		t.Fatalf("want 1 fence, got %d", len(fences))
	}
	if fences[0].SQL != "SELECT 1" {
		t.Errorf("want SQL %q, got %q", "SELECT 1", fences[0].SQL)
	}
	if fences[0].Out != "" {
		t.Errorf("want empty Out, got %q", fences[0].Out)
	}
}

func TestExtractFences_WithOut(t *testing.T) {
	text := "```awiki-query out=my-dataset\nSELECT 2\n```\n"
	fences := ExtractFences(text)
	if len(fences) != 1 {
		t.Fatalf("want 1 fence, got %d", len(fences))
	}
	if fences[0].Out != "my-dataset" {
		t.Errorf("want Out %q, got %q", "my-dataset", fences[0].Out)
	}
	if fences[0].SQL != "SELECT 2" {
		t.Errorf("want SQL %q, got %q", "SELECT 2", fences[0].SQL)
	}
}

func TestExtractFences_Multi(t *testing.T) {
	text := "```awiki-query\nSELECT 1\n```\nsome text\n```awiki-query out=foo\nSELECT 2\n```\n"
	fences := ExtractFences(text)
	if len(fences) != 2 {
		t.Fatalf("want 2 fences, got %d", len(fences))
	}
	if fences[0].SQL != "SELECT 1" {
		t.Errorf("fence 0 SQL: want %q got %q", "SELECT 1", fences[0].SQL)
	}
	if fences[1].SQL != "SELECT 2" {
		t.Errorf("fence 1 SQL: want %q got %q", "SELECT 2", fences[1].SQL)
	}
	if fences[1].Out != "foo" {
		t.Errorf("fence 1 Out: want %q got %q", "foo", fences[1].Out)
	}
}

func TestExtractFences_NoClosing(t *testing.T) {
	// Ill-formed: no closing fence; should be skipped.
	text := "```awiki-query\nSELECT 1\n"
	fences := ExtractFences(text)
	if len(fences) != 0 {
		t.Fatalf("want 0 fences for ill-formed input, got %d", len(fences))
	}
}

func TestExtractFences_ByteOffsets(t *testing.T) {
	text := "```awiki-query\nSELECT 1\n```\n"
	fences := ExtractFences(text)
	if len(fences) != 1 {
		t.Fatalf("want 1 fence, got %d", len(fences))
	}
	f := fences[0]
	if f.Start != 0 {
		t.Errorf("want Start=0, got %d", f.Start)
	}
	if f.End != len(text) {
		t.Errorf("want End=%d, got %d", len(text), f.End)
	}
	// Verify round-trip: text[f.Start:f.End] == original text
	if text[f.Start:f.End] != text {
		t.Errorf("byte slice mismatch")
	}
}
