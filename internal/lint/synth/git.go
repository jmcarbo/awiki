package synth

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"awiki/internal/wiki"
)

func LintS6(page wiki.Page, repoRoot string) []Diagnostic {
	lastGenerated := page.Frontmatter["last_generated"]
	if page.LastUpdated == "" || lastGenerated == "" {
		return nil
	}
	lastUpdatedDate := firstDate(page.LastUpdated)
	lastGeneratedDate := firstDate(lastGenerated)
	if lastUpdatedDate == "" || lastGeneratedDate == "" || lastUpdatedDate <= lastGeneratedDate {
		return nil
	}
	region, diagnostics := ParseGeneratedRegion(page.Body)
	if len(diagnostics) > 0 {
		return nil
	}
	regionStart, regionEnd := generatedRegionFileLines(page, region)
	if regionStart == 0 || regionEnd == 0 || regionEnd < regionStart {
		return nil
	}
	diff, ok := gitDiff(repoRoot, page.Path)
	if !ok {
		return nil
	}
	if !diffIntersects(diff, regionStart, regionEnd) {
		return nil
	}
	return []Diagnostic{{
		Level:   "WARN",
		File:    page.Path,
		Code:    "S6",
		Message: "hand-edit inside generated region (last_updated=" + page.LastUpdated + " > last_generated=" + lastGenerated + ")",
	}}
}

func firstDate(value string) string {
	if len(value) < 10 {
		return ""
	}
	return value[:10]
}

func generatedRegionBodyLines(body string, region GeneratedRegion) (int, int) {
	beginLine := 1 + strings.Count(body[:region.BeginOffset], "\n")
	endLine := 1 + strings.Count(body[:region.EndOffset], "\n")
	return beginLine + 1, endLine - 1
}

func generatedRegionFileLines(page wiki.Page, region GeneratedRegion) (int, int) {
	bodyStartLine := len(page.FrontRawLines) + 3
	if len(page.FrontRawLines) == 0 {
		bodyStartLine = 1
	}
	if data, err := os.ReadFile(page.Path); err == nil {
		_, offset := splitMarkdownFrontmatter(string(data))
		bodyStartLine = 1 + strings.Count(string(data[:offset]), "\n")
	}
	bodyStart, bodyEnd := generatedRegionBodyLines(page.Body, region)
	return bodyStartLine + bodyStart - 1, bodyStartLine + bodyEnd - 1
}

func gitDiff(repoRoot string, path string) (string, bool) {
	rel, err := filepath.Rel(repoRoot, path)
	if err != nil {
		rel = path
	}
	if err := exec.Command("git", "-C", repoRoot, "ls-files", "--error-unmatch", rel).Run(); err != nil {
		return "", false
	}
	cmd := exec.Command("git", "-C", repoRoot, "diff", "-U0", "--", rel)
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

var diffHunkPattern = regexp.MustCompile(`(?m)^@@ -[0-9,]+ \+([0-9]+)(?:,([0-9]+))? @@`)

func diffIntersects(diff string, regionStart int, regionEnd int) bool {
	for _, match := range diffHunkPattern.FindAllStringSubmatch(diff, -1) {
		start, _ := strconv.Atoi(match[1])
		length := 1
		if match[2] != "" {
			length, _ = strconv.Atoi(match[2])
		}
		if length == 0 {
			length = 1
		}
		end := start + length - 1
		if start <= regionEnd && end >= regionStart {
			return true
		}
	}
	return false
}
