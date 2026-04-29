package synth

import (
	"strings"
	"testing"
)

const regionFixture = `---
title: test
---

Some content.

<!-- BEGIN GENERATED plugin=briefing scope_hash=abc123 -->
Old generated content.
<!-- END GENERATED -->

After.
`

func TestClearRegionHappyPath(t *testing.T) {
	got, err := ClearRegion(regionFixture, "briefing", "def456")
	if err != nil {
		t.Fatalf("ClearRegion: %v", err)
	}
	if !strings.Contains(got, "<!-- BEGIN GENERATED plugin=briefing scope_hash=def456 -->") {
		t.Errorf("BEGIN line not rewritten: %q", got)
	}
	if strings.Contains(got, "Old generated content.") {
		t.Errorf("old body not cleared: %q", got)
	}
	if !strings.Contains(got, "<!-- END GENERATED -->") {
		t.Errorf("END marker missing: %q", got)
	}
	// The region body should be a single blank line (i.e., "\n\n" between BEGIN and END).
	beginIdx := strings.Index(got, "<!-- BEGIN GENERATED")
	beginEnd := strings.Index(got[beginIdx:], "\n") + beginIdx + 1
	endIdx := strings.Index(got, "<!-- END GENERATED -->")
	body := got[beginEnd:endIdx]
	if body != "\n" {
		t.Errorf("body after ClearRegion should be single blank line, got %q", body)
	}
}

func TestClearRegionDuplicateBegin(t *testing.T) {
	text := `<!-- BEGIN GENERATED plugin=a scope_hash=111 -->
x
<!-- BEGIN GENERATED plugin=a scope_hash=222 -->
y
<!-- END GENERATED -->
`
	_, err := ClearRegion(text, "a", "333")
	if err == nil {
		t.Fatal("expected error for duplicate BEGIN")
	}
}

func TestClearRegionMissingEnd(t *testing.T) {
	text := "<!-- BEGIN GENERATED plugin=a scope_hash=111 -->\nno end\n"
	_, err := ClearRegion(text, "a", "222")
	if err == nil {
		t.Fatal("expected error for missing END")
	}
}

func TestCheckMarkersValid(t *testing.T) {
	if err := CheckMarkers(regionFixture); err != nil {
		t.Fatalf("CheckMarkers on valid text: %v", err)
	}
}

func TestCheckMarkersNoBegin(t *testing.T) {
	text := "no markers here\n<!-- END GENERATED -->\n"
	if err := CheckMarkers(text); err == nil {
		t.Fatal("expected error")
	}
}

func TestCheckMarkersNoEnd(t *testing.T) {
	text := "<!-- BEGIN GENERATED plugin=a scope_hash=111 -->\nno end\n"
	if err := CheckMarkers(text); err == nil {
		t.Fatal("expected error")
	}
}

func TestCheckMarkersBeginAfterEnd(t *testing.T) {
	text := "<!-- END GENERATED -->\n<!-- BEGIN GENERATED plugin=a scope_hash=111 -->\n"
	if err := CheckMarkers(text); err == nil {
		t.Fatal("expected error for inverted markers")
	}
}

func TestSetScopeHashInMarker(t *testing.T) {
	got := SetScopeHashInMarker(regionFixture, "newhas")
	if !strings.Contains(got, "scope_hash=newhas") {
		t.Errorf("scope_hash not updated: %q", got)
	}
	if strings.Contains(got, "scope_hash=abc123") {
		t.Errorf("old scope_hash still present: %q", got)
	}
}

func TestItoaBasic(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{-3, "-3"},
	}
	for _, c := range cases {
		if got := itoa(c.n); got != c.want {
			t.Errorf("itoa(%d) = %q want %q", c.n, got, c.want)
		}
	}
}
