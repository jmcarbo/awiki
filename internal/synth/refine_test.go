package synth

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeSynthPage writes a synthesis page under dir/content/synthesis/<slug>.md
// and returns the runner configured for that temp dir.
func makeSynthPageForRefine(t *testing.T, slug, content string) (string, *Runner) {
	t.Helper()
	dir := t.TempDir()
	contentDir := filepath.Join(dir, "content")
	synthDir := filepath.Join(contentDir, "synthesis")
	if err := os.MkdirAll(synthDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(synthDir, slug+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write page: %v", err)
	}
	r := &Runner{
		RepoRoot:   dir,
		ContentDir: contentDir,
		Today:      "2026-04-29",
	}
	return path, r
}

// pageBody reads and returns the current content of the synthesis page.
func pageBody(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read page: %v", err)
	}
	return string(data)
}

// TestRefineAppendToExistingFeedback covers case 1: page has ## Feedback,
// then a later section. Bullet appears inside Feedback, downstream intact.
func TestRefineAppendToExistingFeedback(t *testing.T) {
	const slug = "my-synth"
	const initial = `---
title: My Synth
last_updated: 2026-01-01
---

## Feedback

- existing note

## Summary

Some text.

<!-- BEGIN GENERATED v1 plugin=test scope_hash=abc123 -->
generated content
<!-- END GENERATED -->
`
	path, r := makeSynthPageForRefine(t, slug, initial)
	var stderr bytes.Buffer
	if err := r.Refine(slug, "new feedback note", &stderr); err != nil {
		t.Fatalf("Refine error: %v", err)
	}

	got := pageBody(t, path)

	// Bullet should appear before ## Summary.
	feedbackPos := strings.Index(got, "## Feedback")
	bulletPos := strings.Index(got, "- new feedback note")
	summaryPos := strings.Index(got, "## Summary")
	if feedbackPos < 0 {
		t.Error("## Feedback missing")
	}
	if bulletPos < 0 {
		t.Error("bullet missing")
	}
	if summaryPos < 0 {
		t.Error("## Summary missing")
	}
	if bulletPos < feedbackPos {
		t.Error("bullet appeared before ## Feedback")
	}
	if bulletPos > summaryPos {
		t.Error("bullet appeared after ## Summary, expected inside Feedback section")
	}

	// Blank line should follow the bullet before ## Summary.
	afterBullet := got[bulletPos+len("- new feedback note"):]
	if !strings.HasPrefix(afterBullet, "\n\n") {
		t.Errorf("expected blank line after bullet, got: %q", afterBullet[:min(20, len(afterBullet))])
	}

	// Downstream content intact.
	if !strings.Contains(got, "generated content") {
		t.Error("generated content missing")
	}

	// last_updated updated.
	if !strings.Contains(got, "last_updated: 2026-04-29") {
		t.Errorf("last_updated not updated; got:\n%s", got)
	}

	// Stderr.
	if !strings.Contains(stderr.String(), "SYNTH-REFINE|appended|my-synth|new feedback note") {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
}

// TestRefineInsertSectionBeforeBeginMarker covers case 2: no Feedback section.
func TestRefineInsertSectionBeforeBeginMarker(t *testing.T) {
	const slug = "no-feedback"
	const initial = `---
title: No Feedback
last_updated: 2026-01-01
---

## Summary

Some text.

<!-- BEGIN GENERATED v1 plugin=test scope_hash=abc123 -->
generated content
<!-- END GENERATED -->
`
	path, r := makeSynthPageForRefine(t, slug, initial)
	var stderr bytes.Buffer
	if err := r.Refine(slug, "first note", &stderr); err != nil {
		t.Fatalf("Refine error: %v", err)
	}

	got := pageBody(t, path)

	// ## Feedback section should now exist.
	fbPos := strings.Index(got, "## Feedback")
	if fbPos < 0 {
		t.Fatal("## Feedback not inserted")
	}

	// The section should be immediately before <!-- BEGIN GENERATED.
	markerPos := strings.Index(got, "<!-- BEGIN GENERATED")
	if markerPos < 0 {
		t.Fatal("BEGIN GENERATED marker missing")
	}

	// The inserted block should be: ## Feedback\n\n- first note\n\n<!-- BEGIN...
	sectionBlock := "## Feedback\n\n- first note\n\n"
	blockPos := strings.Index(got, sectionBlock)
	if blockPos < 0 {
		t.Errorf("expected %q in output; got:\n%s", sectionBlock, got)
	}
	// Marker must immediately follow the section block.
	if blockPos+len(sectionBlock) != markerPos {
		t.Errorf("BEGIN GENERATED not immediately after Feedback block; blockEnd=%d markerPos=%d",
			blockPos+len(sectionBlock), markerPos)
	}

	// Content before the new section unchanged (## Summary still there).
	if !strings.Contains(got, "## Summary") {
		t.Error("## Summary missing")
	}
	summaryPos := strings.Index(got, "## Summary")
	if summaryPos > fbPos {
		t.Error("## Summary appears after ## Feedback, expected before")
	}

	// last_updated updated.
	if !strings.Contains(got, "last_updated: 2026-04-29") {
		t.Errorf("last_updated not updated; got:\n%s", got)
	}
}

// TestRefineIdempotentOnDuplicate covers case 3: duplicate note skipped.
func TestRefineIdempotentOnDuplicate(t *testing.T) {
	const slug = "idem-synth"
	const initial = `---
title: Idem
last_updated: 2026-01-01
---

<!-- BEGIN GENERATED v1 plugin=test scope_hash=abc -->
content
<!-- END GENERATED -->
`
	path, r := makeSynthPageForRefine(t, slug, initial)
	var stderr1 bytes.Buffer
	if err := r.Refine(slug, "same note", &stderr1); err != nil {
		t.Fatalf("first Refine error: %v", err)
	}

	body1 := pageBody(t, path)
	bulletCount1 := strings.Count(body1, "- same note")
	if bulletCount1 != 1 {
		t.Errorf("expected 1 bullet after first call, got %d", bulletCount1)
	}

	var stderr2 bytes.Buffer
	if err := r.Refine(slug, "same note", &stderr2); err != nil {
		t.Fatalf("second Refine error: %v", err)
	}

	body2 := pageBody(t, path)
	bulletCount2 := strings.Count(body2, "- same note")
	if bulletCount2 != 1 {
		t.Errorf("expected 1 bullet after second (duplicate) call, got %d", bulletCount2)
	}

	// Stderr on second call must contain skipped (duplicate).
	if !strings.Contains(stderr2.String(), "skipped (duplicate)") {
		t.Errorf("second call stderr missing 'skipped (duplicate)': %q", stderr2.String())
	}
}

// TestRefineSetsLastUpdated covers case 4.
func TestRefineSetsLastUpdated(t *testing.T) {
	const slug = "date-synth"
	const initial = `---
title: Date Test
last_updated: 2020-01-01
---

<!-- BEGIN GENERATED v1 plugin=test scope_hash=abc -->
content
<!-- END GENERATED -->
`
	path, r := makeSynthPageForRefine(t, slug, initial)
	r.Today = "2026-04-29"
	var stderr bytes.Buffer
	if err := r.Refine(slug, "some note", &stderr); err != nil {
		t.Fatalf("Refine error: %v", err)
	}

	got := pageBody(t, path)
	if !strings.Contains(got, "last_updated: 2026-04-29") {
		t.Errorf("last_updated not set to Today; got:\n%s", got)
	}
	// Old value gone.
	if strings.Contains(got, "last_updated: 2020-01-01") {
		t.Error("old last_updated still present")
	}
}

// TestRefineMultiWordNote covers case 5: note with spaces and punctuation.
func TestRefineMultiWordNote(t *testing.T) {
	const slug = "multi-word"
	const initial = `---
title: Multi Word
last_updated: 2026-01-01
---

<!-- BEGIN GENERATED v1 plugin=test scope_hash=abc -->
content
<!-- END GENERATED -->
`
	path, r := makeSynthPageForRefine(t, slug, initial)
	note := "Fix the data pipeline: missing null checks & edge cases!"
	var stderr bytes.Buffer
	if err := r.Refine(slug, note, &stderr); err != nil {
		t.Fatalf("Refine error: %v", err)
	}

	got := pageBody(t, path)
	expected := "- " + note
	if !strings.Contains(got, expected) {
		t.Errorf("note not found verbatim in page; expected %q\ngot:\n%s", expected, got)
	}
}

// TestRefineMissingPageErrors covers case 6.
func TestRefineMissingPageErrors(t *testing.T) {
	dir := t.TempDir()
	contentDir := filepath.Join(dir, "content")
	synthDir := filepath.Join(contentDir, "synthesis")
	if err := os.MkdirAll(synthDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	r := &Runner{
		RepoRoot:   dir,
		ContentDir: contentDir,
		Today:      "2026-04-29",
	}
	var stderr bytes.Buffer
	err := r.Refine("nonexistent-slug", "some note", &stderr)
	if err == nil {
		t.Fatal("expected error for missing page, got nil")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected IsNotExist error, got: %v", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
