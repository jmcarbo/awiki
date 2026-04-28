package synth

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"awiki/internal/wiki"
)

func LintS9(page wiki.Page, repoRoot string) []Diagnostic {
	cap := pluginEvidenceWordCap(repoRoot, page.Frontmatter["plugin"])
	total := evidenceWordCount(page)
	if total <= cap {
		return nil
	}
	return []Diagnostic{{
		Level:   "ERROR",
		File:    page.Path,
		Code:    "S9",
		Message: "aggregate evidence words=" + intString(total) + " exceeds plugin cap max_evidence_total_words=" + intString(cap),
	}}
}

func evidenceWordCount(page wiki.Page) int {
	region, diagnostics := ParseGeneratedRegion(page.Body)
	if len(diagnostics) > 0 {
		return 0
	}
	total := 0
	for _, match := range evidenceLinePattern.FindAllStringSubmatch(region.Body, -1) {
		total += len(strings.Fields(match[1]))
	}
	return total
}

func pluginEvidenceWordCap(repoRoot string, plugin string) int {
	path := filepath.Join(repoRoot, "synthesis-plugins", plugin+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return 500
	}
	match := maxEvidencePattern.FindStringSubmatch(string(data))
	if len(match) < 2 {
		return 500
	}
	value, err := strconv.Atoi(match[1])
	if err != nil {
		return 500
	}
	return value
}

var maxEvidencePattern = regexp.MustCompile(`(?m)^max_evidence_total_words:\s*([0-9]+)\s*$`)
