package synth

import "testing"

func TestParseScopeFromFrontmatterTag(t *testing.T) {
	fm := "scope:\n  tag: decision\n"
	got := ParseScopeFromFrontmatter(fm)
	if got.Kind != "tag" || got.Tag != "decision" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseScopeSlugs(t *testing.T) {
	fm := "scope:\n  slugs: [alpha, beta]\n"
	got := ParseScopeFromFrontmatter(fm)
	if got.Kind != "slugs" {
		t.Fatalf("kind %q", got.Kind)
	}
	if len(got.Slugs) != 2 || got.Slugs[0] != "alpha" || got.Slugs[1] != "beta" {
		t.Fatalf("slugs %v", got.Slugs)
	}
}

func TestParseScopeQuery(t *testing.T) {
	fm := "scope:\n  query: \"decision making\"\n"
	got := ParseScopeFromFrontmatter(fm)
	if got.Kind != "query" {
		t.Fatalf("kind %q", got.Kind)
	}
	if got.Query != "decision making" {
		t.Fatalf("query %q", got.Query)
	}
}

func TestParseScopeExcludeTags(t *testing.T) {
	fm := "scope:\n  tag: project\n  exclude_tags: [archive, draft]\n"
	got := ParseScopeFromFrontmatter(fm)
	if got.Kind != "tag" || got.Tag != "project" {
		t.Fatalf("tag %+v", got)
	}
	if len(got.ExcludeTags) != 2 || got.ExcludeTags[0] != "archive" || got.ExcludeTags[1] != "draft" {
		t.Fatalf("exclude_tags %v", got.ExcludeTags)
	}
}

func TestScopeHashMatchesPython(t *testing.T) {
	// Verified: printf 'a\nb\nc\n' | python3 scripts/lint-synth-hash.py
	// outputs "880553". sha256("a\nb\nc\n")[:6]
	got := ScopeHash([]string{"a", "b", "c"})
	want := "880553"
	if got != want {
		t.Fatalf("hash %q want %q", got, want)
	}
}

func TestScopeHashEmpty(t *testing.T) {
	// sha256("") = e3b0c4...
	got := ScopeHash(nil)
	want := "e3b0c4"
	if got != want {
		t.Fatalf("hash %q want %q", got, want)
	}
}

func TestScopeHashSortsInput(t *testing.T) {
	// Hash should be same regardless of input order.
	h1 := ScopeHash([]string{"c", "a", "b"})
	h2 := ScopeHash([]string{"a", "b", "c"})
	if h1 != h2 {
		t.Fatalf("hashes differ: %q vs %q", h1, h2)
	}
}
