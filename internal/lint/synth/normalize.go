package synth

import (
	"regexp"
	"strings"

	"awiki/internal/wiki"
	"golang.org/x/text/unicode/norm"
)

var whitespacePattern = regexp.MustCompile(`\s+`)

func NormalizeEvidenceText(text string) string {
	text = norm.NFC.String(text)
	text = removeZeroWidth(text)
	replacer := strings.NewReplacer(
		"‐", "-",
		"‑", "-",
		"‒", "-",
		"–", "-",
		"—", "-",
		"“", `"`,
		"”", `"`,
		"‘", "'",
		"’", "'",
		"\u00a0", " ",
	)
	text = replacer.Replace(text)
	text = whitespacePattern.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func removeZeroWidth(text string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\u200b', '\u200c', '\u200d', '\ufeff', '\u200e', '\u200f', '\u202a', '\u202b', '\u202c', '\u202d', '\u202e':
			return -1
		default:
			return r
		}
	}, text)
}

func RewriteQuoteWikilinks(text string, idx *wiki.Index) string {
	return wikiLinkReplacePattern.ReplaceAllStringFunc(text, func(raw string) string {
		links := wiki.ExtractLinks(raw)
		if len(links) == 0 {
			return raw
		}
		link := links[0]
		if link.Display != "" {
			return link.Display
		}
		if idx != nil {
			if page, ok := idx.Resolve(link.Target); ok && page.Title != "" {
				return page.Title
			}
		}
		return link.Target
	})
}

var wikiLinkReplacePattern = regexp.MustCompile(`\[\[[^\]\n]+\]\]`)

func EvidenceQuoteMatchesSource(quote string, source wiki.Page, idx *wiki.Index) bool {
	rewrittenQuote := RewriteQuoteWikilinks(quote, idx)
	rewrittenSource := RewriteQuoteWikilinks(source.Body, idx)
	normalizedQuote := NormalizeEvidenceText(rewrittenQuote)
	normalizedSource := NormalizeEvidenceText(rewrittenSource)
	return normalizedQuote != "" && strings.Contains(normalizedSource, normalizedQuote)
}

var evidenceLinePattern = regexp.MustCompile(`(?m)^>\s+"(.+)"\s+—\s+\[\[([a-z0-9][a-z0-9-]*)\]\]\s*$`)

func LintS3(page wiki.Page, idx *wiki.Index) []Diagnostic {
	region, regionDiagnostics := ParseGeneratedRegion(page.Body)
	if len(regionDiagnostics) > 0 {
		return nil
	}
	var diagnostics []Diagnostic
	for _, match := range evidenceLinePattern.FindAllStringSubmatch(region.Body, -1) {
		quote := match[1]
		slug := match[2]
		source, ok := resolvePage(idx, slug)
		if !ok {
			diagnostics = append(diagnostics, Diagnostic{
				Level:   "ERROR",
				File:    page.Path,
				Code:    "S3",
				Message: "evidence cites unresolvable slug: " + slug,
			})
			continue
		}
		if EvidenceQuoteMatchesSource(quote, source, idx) {
			continue
		}
		message := "evidence quote not found in cited source: " + slug
		if suggestion := FuzzySuggestion(source.Body, quote); suggestion != "" {
			message += "; suggestion: " + NormalizeEvidenceText(suggestion)
		}
		diagnostics = append(diagnostics, Diagnostic{
			Level:   "ERROR",
			File:    page.Path,
			Code:    "S3",
			Message: message,
		})
	}
	return diagnostics
}

func resolvePage(idx *wiki.Index, slug string) (wiki.Page, bool) {
	if idx == nil {
		return wiki.Page{}, false
	}
	return idx.Resolve(slug)
}
