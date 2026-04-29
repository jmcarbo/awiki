package chart

import (
	"path/filepath"
	"regexp"
	"strings"

	"awiki/internal/region"
)

// InjectPreview inserts or refreshes a managed chart-preview region in pageText
// for the given chartID. The SVG path is assumed to be at
// ../../assets/charts/<chartID>.svg relative to the page (the canonical path
// for pages under content/charts/).
//
// Use InjectPreviewAt when the page lives at an arbitrary path.
func InjectPreview(pageText, chartID string) string {
	body := "![" + chartID + "](../../assets/charts/" + chartID + ".svg)"
	result, _ := region.ManagedReplace(pageText, "chart-preview", chartID, body)
	return result
}

// InjectPreviewAt computes the correct relative SVG path from the page's
// directory to the chart sidecar, then injects the managed region.
//
//   - repoRoot is the absolute path to the repository root.
//   - assetsDir is the absolute path to the assets/charts directory.
//   - pagePath is the absolute path to the markdown page.
func InjectPreviewAt(pageText, chartID, repoRoot, assetsDir, pagePath string) string {
	// Compute the page's directory relative to repoRoot to get the
	// "page_dir" used by the Python equivalent.
	pageRel, err := filepath.Rel(repoRoot, pagePath)
	if err != nil {
		return InjectPreview(pageText, chartID)
	}
	pageDir := filepath.Dir(pageRel)

	// Absolute path of the SVG sidecar.
	absSidecar := filepath.Join(assetsDir, chartID+".svg")

	// Relative path from <repoRoot>/<pageDir> to the sidecar.
	rel, err := filepath.Rel(filepath.Join(repoRoot, pageDir), absSidecar)
	if err != nil {
		return InjectPreview(pageText, chartID)
	}
	// Use forward slashes for markdown compatibility on all platforms.
	rel = filepath.ToSlash(rel)

	body := "![" + chartID + "](" + rel + ")"
	result, _ := region.ManagedReplace(pageText, "chart-preview", chartID, body)
	return result
}

// RemovePreview strips the managed chart-preview region for the given chartID
// from pageText. If the region is absent, the text is returned unchanged.
// Any surrounding blank lines introduced by the region are also cleaned up.
var managedRegionRE = func() *regexp.Regexp {
	return regexp.MustCompile(`\n<!-- BEGIN chart-preview:[a-z0-9][a-z0-9-]* -->\n.*?\n<!-- END chart-preview:[a-z0-9][a-z0-9-]* -->\n`)
}()

func RemovePreview(pageText, chartID string) string {
	begin := "<!-- BEGIN chart-preview:" + chartID + " -->"
	end := "<!-- END chart-preview:" + chartID + " -->"

	// Build a pattern that matches the full block including surrounding newlines.
	pat := regexp.MustCompile(
		`(?s)\n` +
			regexp.QuoteMeta(begin) +
			`\n.*?\n` +
			regexp.QuoteMeta(end) +
			`\n?`,
	)
	result := pat.ReplaceAllLiteralString(pageText, "\n")
	// Collapse multiple consecutive blank lines left by removal.
	result = collapseBlankLines(result)
	// Check if the block was at the very beginning (no leading \n).
	if strings.HasPrefix(pageText, begin) {
		pat2 := regexp.MustCompile(
			`(?s)^` +
				regexp.QuoteMeta(begin) +
				`\n.*?\n` +
				regexp.QuoteMeta(end) +
				`\n?`,
		)
		result = pat2.ReplaceAllLiteralString(result, "")
	}
	return result
}

// collapseBlankLines replaces three or more consecutive newlines with two.
func collapseBlankLines(s string) string {
	re := regexp.MustCompile(`\n{3,}`)
	return re.ReplaceAllLiteralString(s, "\n\n")
}
