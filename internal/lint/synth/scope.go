package synth

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"awiki/internal/wiki"
)

func ResolveScopeSlugs(page wiki.Page, idx *wiki.Index) []string {
	scope := parseScope(page.FrontRawLines)
	switch scope.kind {
	case "slugs":
		return sortedUnique(scope.slugs)
	case "tag":
		if idx == nil {
			return nil
		}
		var slugs []string
		for _, candidate := range idx.Pages {
			if hasPageTag(candidate, scope.value) {
				slugs = append(slugs, candidate.Slug)
			}
		}
		return sortedUnique(slugs)
	default:
		return nil
	}
}

func LintS4(page wiki.Page, idx *wiki.Index) []Diagnostic {
	scope := parseScope(page.FrontRawLines)
	if scope.kind == "query" {
		return nil
	}
	inScope := make(map[string]bool)
	for _, slug := range ResolveScopeSlugs(page, idx) {
		inScope[slug] = true
	}
	if len(inScope) == 0 {
		return nil
	}
	region, regionDiagnostics := ParseGeneratedRegion(page.Body)
	if len(regionDiagnostics) > 0 {
		return nil
	}
	seen := make(map[string]bool)
	var diagnostics []Diagnostic
	for _, link := range wiki.ExtractLinks(region.Body) {
		if !slugPattern.MatchString(link.Target) || seen[link.Target] {
			continue
		}
		seen[link.Target] = true
		if inScope[link.Target] {
			continue
		}
		diagnostics = append(diagnostics, Diagnostic{
			Level:   "ERROR",
			File:    page.Path,
			Code:    "S4",
			Message: "citation [[" + link.Target + "]] is out of scope",
		})
	}
	return diagnostics
}

func LintS5(page wiki.Page, idx *wiki.Index) []Diagnostic {
	scope := parseScope(page.FrontRawLines)
	if scope.kind == "query" {
		return nil
	}
	region, regionDiagnostics := ParseGeneratedRegion(page.Body)
	if len(regionDiagnostics) > 0 {
		return nil
	}
	declared := declaredScopeHash(region.BeginLine)
	if declared == "" {
		return nil
	}
	current := ScopeHash(ResolveScopeSlugs(page, idx))
	if current == "" || current == declared {
		return nil
	}
	return []Diagnostic{{
		Level:   "WARN",
		File:    page.Path,
		Code:    "S5",
		Message: "scope drift (declared=" + declared + " current=" + current + "); consider regen",
	}}
}

func ScopeHash(slugs []string) string {
	if len(slugs) == 0 {
		return ""
	}
	input := strings.Join(sortedUnique(slugs), "\n") + "\n"
	sum := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", sum)[:6]
}

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
var scopeHashPattern = regexp.MustCompile(`scope_hash=([a-f0-9]{6})`)

func declaredScopeHash(beginLine string) string {
	match := scopeHashPattern.FindStringSubmatch(beginLine)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func hasPageTag(page wiki.Page, tag string) bool {
	for _, candidate := range page.Tags {
		if candidate == tag {
			return true
		}
	}
	return false
}

func sortedUnique(values []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
