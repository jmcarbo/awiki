package wiki

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Page struct {
	Path          string
	RelPath       string
	Slug          string
	Title         string
	Type          string
	Date          string
	LastUpdated   string
	Tags          []string
	Aliases       []string
	Sources       []string
	Draft         bool
	Frontmatter   map[string]string
	FrontRawLines []string
	Body          string
	Links         []Link
}

type Link struct {
	Target  string
	Display string
	Raw     string
}

var wikiLinkPattern = regexp.MustCompile(`\[\[([^\]\|\n]+)(?:\|([^\]\n]+))?\]\]`)

func DiscoverPages(contentDir string) ([]Page, error) {
	var paths []string
	if err := filepath.WalkDir(contentDir, func(path string, entry os.DirEntry, err error) error {
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
	}); err != nil {
		return nil, err
	}

	sort.Slice(paths, func(i, j int) bool {
		return relPath(paths[i], contentDir) < relPath(paths[j], contentDir)
	})

	pages := make([]Page, 0, len(paths))
	for _, path := range paths {
		page, err := ParsePage(path, contentDir)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, nil
}

func ParsePage(path, contentDir string) (Page, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Page{}, err
	}

	body, frontRaw := splitFrontmatter(string(data))
	frontmatter, lists := parseFrontmatter(frontRaw)

	page := Page{
		Path:          path,
		RelPath:       relPath(path, contentDir),
		Slug:          strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Title:         frontmatter["title"],
		Type:          frontmatter["type"],
		Date:          frontmatter["date"],
		LastUpdated:   frontmatter["last_updated"],
		Tags:          lists["tags"],
		Aliases:       lists["aliases"],
		Sources:       lists["sources"],
		Draft:         parseBool(frontmatter["draft"]),
		Frontmatter:   frontmatter,
		FrontRawLines: frontRaw,
		Body:          body,
		Links:         ExtractLinks(body),
	}
	return page, nil
}

func ExtractLinks(body string) []Link {
	matches := wikiLinkPattern.FindAllStringSubmatch(body, -1)
	links := make([]Link, 0, len(matches))
	for _, match := range matches {
		display := ""
		if len(match) > 2 {
			display = strings.TrimSpace(match[2])
		}
		links = append(links, Link{
			Target:  strings.TrimSpace(match[1]),
			Display: display,
			Raw:     match[0],
		})
	}
	return links
}

func splitFrontmatter(content string) (string, []string) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.SplitAfter(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content, nil
	}

	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			frontRaw := make([]string, 0, i-1)
			for _, line := range lines[1:i] {
				frontRaw = append(frontRaw, strings.TrimSuffix(line, "\n"))
			}
			return strings.Join(lines[i+1:], ""), frontRaw
		}
	}
	return content, nil
}

func parseFrontmatter(lines []string) (map[string]string, map[string][]string) {
	fields := make(map[string]string)
	lists := make(map[string][]string)

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if value == "" {
			var values []string
			for i+1 < len(lines) {
				next := strings.TrimSpace(lines[i+1])
				if next == "" {
					i++
					continue
				}
				if !strings.HasPrefix(next, "-") {
					break
				}
				item := strings.TrimSpace(strings.TrimPrefix(next, "-"))
				values = append(values, cleanValue(item))
				i++
			}
			if len(values) > 0 {
				lists[key] = values
				fields[key] = strings.Join(values, ",")
			}
			continue
		}

		if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			values := parseInlineList(value)
			lists[key] = values
			fields[key] = strings.Join(values, ",")
			continue
		}

		fields[key] = cleanValue(value)
	}

	return fields, lists
}

func parseInlineList(value string) []string {
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
	if inner == "" {
		return nil
	}

	var values []string
	var current strings.Builder
	var quote rune
	for _, r := range inner {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ',':
			values = append(values, cleanValue(current.String()))
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	values = append(values, cleanValue(current.String()))
	return values
}

func cleanValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		first := value[0]
		last := value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func parseBool(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true")
}

func relPath(path, contentDir string) string {
	rel, err := filepath.Rel(contentDir, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
