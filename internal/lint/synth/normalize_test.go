package synth

import (
	"os"
	"path/filepath"
	"testing"

	"awiki/internal/wiki"
)

func TestNormalizeEvidenceText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "smart quotes", in: "“the human mind”", want: `"the human mind"`},
		{name: "nfc", in: "cafe\u0301 re\u0301sume\u0301", want: "café résumé"},
		{name: "nbsp and whitespace", in: "recorded\u00a0once\nand\t reused", want: "recorded once and reused"},
		{name: "zero width", in: "memex\u200bis", want: "memexis"},
		{name: "hyphen variants", in: "indexing — association – memory", want: "indexing - association - memory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeEvidenceText(tt.in); got != tt.want {
				t.Fatalf("NormalizeEvidenceText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRewriteQuoteWikilinksUsesDisplayOrTitle(t *testing.T) {
	contentDir := t.TempDir()
	target := writeSynthPage(t, contentDir, "s1.md", `---
title: "As We May Think"
type: source
---
source body
`)
	idx := wiki.BuildIndex([]wiki.Page{target})

	got := RewriteQuoteWikilinks("read [[s1]] and [[s1|the essay]]", &idx)
	want := "read As We May Think and the essay"
	if got != want {
		t.Fatalf("RewriteQuoteWikilinks() = %q, want %q", got, want)
	}
}

func TestEvidenceQuoteMatchesSource(t *testing.T) {
	contentDir := t.TempDir()
	source := writeSynthPage(t, contentDir, "s-smart-quotes.md", `---
title: "Smart Quotes Source"
type: source
---

Bush wrote that “the human mind operates by association,” and warned that
record-keeping had outpaced our ability to use it.
`)
	idx := wiki.BuildIndex([]wiki.Page{source})

	if !EvidenceQuoteMatchesSource("the human mind operates by association", source, &idx) {
		t.Fatal("EvidenceQuoteMatchesSource() = false, want true")
	}
}

func TestEvidenceQuoteMatchesAcrossParagraphWhitespace(t *testing.T) {
	contentDir := t.TempDir()
	source := writeSynthPage(t, contentDir, "s-multipara.md", `---
title: "Multi"
type: source
---

Selection by association rather than indexing.

The second paragraph extends the trail concept.
`)
	idx := wiki.BuildIndex([]wiki.Page{source})

	if !EvidenceQuoteMatchesSource("rather than indexing. The second paragraph extends", source, &idx) {
		t.Fatal("EvidenceQuoteMatchesSource() = false, want true")
	}
}

func TestEvidenceQuoteWikilinkTitleRewrite(t *testing.T) {
	contentDir := t.TempDir()
	target := writeSynthPage(t, contentDir, "s1.md", `---
title: "As We May Think"
type: source
---
source body
`)
	source := writePageAt(t, contentDir, "sources/with-link.md", `---
title: "With Link"
type: source
---

The source references [[s1]] directly.
`)
	idx := wiki.BuildIndex([]wiki.Page{target, source})

	if !EvidenceQuoteMatchesSource("references As We May Think directly", source, &idx) {
		t.Fatal("EvidenceQuoteMatchesSource() = false, want true")
	}
}

func writePageAt(t *testing.T, contentDir string, rel string, body string) wiki.Page {
	t.Helper()
	path := filepath.Join(contentDir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	page, err := wiki.ParsePage(path, contentDir)
	if err != nil {
		t.Fatal(err)
	}
	return page
}
