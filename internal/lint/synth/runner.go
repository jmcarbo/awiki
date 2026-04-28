package synth

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"awiki/internal/wiki"
)

type FixRecord struct {
	File    string
	Message string
}

func LintPage(page wiki.Page, repoRoot string, contentDir string, idx *wiki.Index) []Diagnostic {
	var diagnostics []Diagnostic
	diagnostics = append(diagnostics, LintStructural(page, repoRoot, contentDir, idx)...)
	diagnostics = append(diagnostics, LintS3(page, idx)...)
	diagnostics = append(diagnostics, LintS4(page, idx)...)
	diagnostics = append(diagnostics, LintS5(page, idx)...)
	diagnostics = append(diagnostics, LintS6(page, repoRoot)...)
	diagnostics = append(diagnostics, LintS9(page, repoRoot)...)
	return diagnostics
}

func FixPage(path string, contentDir string) ([]FixRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(data)
	region, diagnostics := ParseGeneratedRegion(bodyOnly(text))
	if len(diagnostics) > 0 {
		return nil, nil
	}
	bodyStart := bodyOffset(text)
	absoluteStart := bodyStart + region.BeginOffset
	absoluteEnd := bodyStart + region.EndOffset
	beginLineEnd := strings.IndexByte(text[absoluteStart:], '\n')
	if beginLineEnd < 0 {
		return nil, nil
	}
	regionStart := absoluteStart + beginLineEnd + 1
	fixedRegion := normalizeGeneratedRegion(text[regionStart:absoluteEnd])
	if fixedRegion == text[regionStart:absoluteEnd] {
		return nil, nil
	}
	next := text[:regionStart] + fixedRegion + text[absoluteEnd:]
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		return nil, err
	}
	return []FixRecord{{File: path, Message: "normalized generated region"}}, nil
}

func normalizeGeneratedRegion(region string) string {
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
	)
	return removeZeroWidth(replacer.Replace(region))
}

func bodyOnly(markdown string) string {
	body, _ := splitMarkdownFrontmatter(markdown)
	return body
}

func bodyOffset(markdown string) int {
	_, offset := splitMarkdownFrontmatter(markdown)
	return offset
}

func splitMarkdownFrontmatter(markdown string) (string, int) {
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	if !strings.HasPrefix(markdown, "---\n") {
		return markdown, 0
	}
	end := strings.Index(markdown[4:], "\n---\n")
	if end < 0 {
		return markdown, 0
	}
	offset := 4 + end + len("\n---\n")
	return markdown[offset:], offset
}

func SynthesisPages(contentDir string, onlyFile string) ([]string, error) {
	if onlyFile != "" {
		return []string{onlyFile}, nil
	}
	root := filepath.Join(contentDir, "synthesis")
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".md" {
			paths = append(paths, path)
		}
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	sort.Strings(paths)
	return paths, err
}
