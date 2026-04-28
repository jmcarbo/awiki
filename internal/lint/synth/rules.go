package synth

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"awiki/internal/wiki"
)

const feedbackThreshold = 20

type Diagnostic struct {
	Level   string
	File    string
	Code    string
	Message string
}

func LintStructural(page wiki.Page, repoRoot string, contentDir string, idx *wiki.Index) []Diagnostic {
	if page.Type != "synthesis" || page.Frontmatter["plugin"] == "" {
		return nil
	}

	var diagnostics []Diagnostic
	region, regionDiagnostics := ParseGeneratedRegion(page.Body)
	for _, diagnostic := range regionDiagnostics {
		diagnostics = append(diagnostics, Diagnostic{
			Level:   "ERROR",
			File:    page.Path,
			Code:    diagnostic.Code,
			Message: diagnostic.Message,
		})
	}
	if len(regionDiagnostics) == 0 {
		diagnostics = append(diagnostics, lintS2(page, repoRoot, region)...)
	}
	diagnostics = append(diagnostics, lintS7(page)...)
	diagnostics = append(diagnostics, lintS8(page, idx)...)
	return diagnostics
}

func lintS2(page wiki.Page, repoRoot string, region GeneratedRegion) []Diagnostic {
	required, ok := requiredSections(repoRoot, page.Frontmatter["plugin"])
	if !ok {
		return []Diagnostic{{
			Level:   "ERROR",
			File:    page.Path,
			Code:    "S2",
			Message: "cannot read manifest for plugin " + page.Frontmatter["plugin"],
		}}
	}
	if len(required) == 0 {
		return nil
	}
	regionHeadings := make(map[string]bool)
	for _, line := range strings.Split(region.Body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			regionHeadings[line] = true
		}
	}
	var diagnostics []Diagnostic
	for _, heading := range required {
		if regionHeadings[heading] {
			continue
		}
		diagnostics = append(diagnostics, Diagnostic{
			Level:   "ERROR",
			File:    page.Path,
			Code:    "S2",
			Message: "required section missing: " + heading,
		})
	}
	return diagnostics
}

func requiredSections(repoRoot string, plugin string) ([]string, bool) {
	path := filepath.Join(repoRoot, "synthesis-plugins", plugin+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	inFrontmatter := false
	frontmatterDone := false
	inRequired := false
	var sections []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			frontmatterDone = true
			break
		}
		if !inFrontmatter || frontmatterDone {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "required_sections:") {
			inRequired = true
			continue
		}
		if inRequired && trimmed != "" && !strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			inRequired = false
		}
		if !inRequired || !strings.HasPrefix(trimmed, "-") {
			continue
		}
		section := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
		section = strings.Trim(section, `"'`)
		sections = append(sections, section)
	}
	return sections, true
}

func lintS7(page wiki.Page) []Diagnostic {
	count := feedbackBulletCount(page.Body)
	diagnostics := []Diagnostic{{
		Level:   "INFO",
		File:    page.Path,
		Code:    "S7",
		Message: "feedback_count=" + intString(count),
	}}
	if count > feedbackThreshold {
		diagnostics = append(diagnostics, Diagnostic{
			Level:   "WARN",
			File:    page.Path,
			Code:    "S7",
			Message: "feedback_count=" + intString(count) + " exceeds 20; consider scope refactor or page split",
		})
	}
	return diagnostics
}

func feedbackBulletCount(body string) int {
	count := 0
	inFeedback := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "## Feedback":
			inFeedback = true
			continue
		case inFeedback && strings.HasPrefix(trimmed, "## "):
			inFeedback = false
		case inFeedback && strings.HasPrefix(trimmed, "<!-- BEGIN GENERATED"):
			inFeedback = false
		case inFeedback && strings.HasPrefix(trimmed, "- "):
			count++
		}
	}
	return count
}

func lintS8(page wiki.Page, idx *wiki.Index) []Diagnostic {
	scope := parseScope(page.FrontRawLines)
	if scope.kind == "query" || len(scope.slugs) == 0 {
		return nil
	}
	inScope := make(map[string]bool)
	for _, slug := range scope.slugs {
		inScope[slug] = true
	}
	for _, slug := range feedbackLinks(page.Body) {
		if inScope[slug] {
			continue
		}
		if idx != nil {
			if _, ok := idx.Resolve(slug); !ok {
				continue
			}
		}
		return []Diagnostic{{
			Level:   "WARN",
			File:    page.Path,
			Code:    "S8",
			Message: "feedback references out-of-scope page " + slug + "; widen scope or remove bullet",
		}}
	}
	return nil
}

type pageScope struct {
	kind  string
	slugs []string
}

func parseScope(lines []string) pageScope {
	var scope pageScope
	inScope := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "scope:" {
			inScope = true
			continue
		}
		if !inScope {
			continue
		}
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "query":
			scope.kind = "query"
		case "slugs":
			scope.kind = "slugs"
			scope.slugs = parseInlineSlugList(value)
		case "tag":
			scope.kind = "tag"
		}
	}
	return scope
}

func parseInlineSlugList(value string) []string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	if strings.TrimSpace(value) == "" {
		return nil
	}
	var slugs []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(strings.Trim(part, `"'`))
		if part != "" {
			slugs = append(slugs, part)
		}
	}
	return slugs
}

var feedbackLinkPattern = regexp.MustCompile(`\[\[([a-z0-9][a-z0-9-]*)(?:\|[^\]]*)?\]\]`)

func feedbackLinks(body string) []string {
	seen := make(map[string]bool)
	var links []string
	inFeedback := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "## Feedback":
			inFeedback = true
			continue
		case inFeedback && strings.HasPrefix(trimmed, "## "):
			inFeedback = false
		case inFeedback && strings.HasPrefix(trimmed, "<!-- BEGIN GENERATED"):
			inFeedback = false
		}
		if !inFeedback || !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		for _, match := range feedbackLinkPattern.FindAllStringSubmatch(trimmed, -1) {
			if seen[match[1]] {
				continue
			}
			seen[match[1]] = true
			links = append(links, match[1])
		}
	}
	return links
}

func intString(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}
