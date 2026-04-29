package synth

import (
	"context"
	"errors"
	"testing"

	"awiki/internal/adapters"
)

// fakeGit is a test double for adapters.SynthGit.
type fakeGit struct {
	tracked bool
	lsErr   error
	head    string
	headOk  bool
	headErr error
}

func (f *fakeGit) LsFiles(_ context.Context, _, _ string) (bool, error) {
	return f.tracked, f.lsErr
}

func (f *fakeGit) ShowHead(_ context.Context, _, _ string) (string, bool, error) {
	return f.head, f.headOk, f.headErr
}

var _ adapters.SynthGit = (*fakeGit)(nil)

const testPage = `---
title: test
---

Lead.

<!-- BEGIN GENERATED plugin=briefing scope_hash=abc123 -->
Generated body.
<!-- END GENERATED -->
`

func newTestRunner(g adapters.SynthGit) *Runner {
	return &Runner{
		RepoRoot: "/repo",
		Git:      g,
	}
}

func TestHandEditCheckUntracked(t *testing.T) {
	r := newTestRunner(&fakeGit{tracked: false})
	edited, untracked, err := r.HandEditCheck(context.Background(), "/repo/page.md", testPage)
	if err != nil {
		t.Fatal(err)
	}
	if edited || !untracked {
		t.Errorf("got edited=%v untracked=%v, want false/true", edited, untracked)
	}
}

func TestHandEditCheckNoHead(t *testing.T) {
	r := newTestRunner(&fakeGit{tracked: true, headOk: false})
	edited, untracked, err := r.HandEditCheck(context.Background(), "/repo/page.md", testPage)
	if err != nil {
		t.Fatal(err)
	}
	if edited || !untracked {
		t.Errorf("got edited=%v untracked=%v, want false/true", edited, untracked)
	}
}

func TestHandEditCheckEqualRegions(t *testing.T) {
	r := newTestRunner(&fakeGit{tracked: true, head: testPage, headOk: true})
	edited, untracked, err := r.HandEditCheck(context.Background(), "/repo/page.md", testPage)
	if err != nil {
		t.Fatal(err)
	}
	if edited || untracked {
		t.Errorf("got edited=%v untracked=%v, want false/false", edited, untracked)
	}
}

func TestHandEditCheckDivergentRegions(t *testing.T) {
	modifiedPage := `---
title: test
---

Lead.

<!-- BEGIN GENERATED plugin=briefing scope_hash=abc123 -->
Hand-edited content — different!
<!-- END GENERATED -->
`
	r := newTestRunner(&fakeGit{tracked: true, head: testPage, headOk: true})
	edited, untracked, err := r.HandEditCheck(context.Background(), "/repo/page.md", modifiedPage)
	if err != nil {
		t.Fatal(err)
	}
	if !edited || untracked {
		t.Errorf("got edited=%v untracked=%v, want true/false", edited, untracked)
	}
}

func TestHandEditCheckMissingMarkers(t *testing.T) {
	noMarkers := "---\ntitle: test\n---\n\nNo markers here.\n"
	r := newTestRunner(&fakeGit{tracked: true, head: noMarkers, headOk: true})
	edited, untracked, err := r.HandEditCheck(context.Background(), "/repo/page.md", noMarkers)
	if err != nil {
		t.Fatal(err)
	}
	// Both have no markers → extractRegion returns "" for both → equal → not edited.
	if edited || untracked {
		t.Errorf("got edited=%v untracked=%v, want false/false", edited, untracked)
	}
}

func TestHandEditCheckLsFilesError(t *testing.T) {
	r := newTestRunner(&fakeGit{lsErr: errors.New("git error")})
	_, _, err := r.HandEditCheck(context.Background(), "/repo/page.md", testPage)
	if err == nil {
		t.Fatal("expected error from LsFiles")
	}
}

func TestExtractRegion(t *testing.T) {
	got := extractRegion(testPage)
	want := "<!-- BEGIN GENERATED plugin=briefing scope_hash=abc123 -->\nGenerated body.\n<!-- END GENERATED -->\n"
	if got != want {
		t.Errorf("extractRegion:\ngot  %q\nwant %q", got, want)
	}
}

func TestExtractRegionNoMarkers(t *testing.T) {
	got := extractRegion("no markers")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}
