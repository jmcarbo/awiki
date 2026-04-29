package synth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// LoadPlugin parses a synth plugin manifest at path. The file must be
// a YAML frontmatter block (---/--- bracketed) plus a body. Returns
// an error if required fields (name, version) are missing or the
// declared name does not match the file's basename.
func LoadPlugin(path string) (Plugin, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Plugin{}, err
	}
	fmText, body, err := splitFrontmatter(string(data))
	if err != nil {
		return Plugin{}, fmt.Errorf("%s: %w", path, err)
	}
	p, err := parsePluginFrontmatter(fmText)
	if err != nil {
		return Plugin{}, fmt.Errorf("%s: %w", path, err)
	}
	p.Body = body
	if p.Name == "" {
		return Plugin{}, fmt.Errorf("%s: missing 'name'", path)
	}
	if p.Version == "" {
		return Plugin{}, fmt.Errorf("%s: missing 'version'", path)
	}
	expected := strings.TrimSuffix(filepath.Base(path), ".md")
	if p.Name != expected {
		return Plugin{}, fmt.Errorf("%s: name %q must match filename %q", path, p.Name, expected)
	}
	if p.MaxEvidenceTotalWords == 0 {
		p.MaxEvidenceTotalWords = 500 // matches synth-plugin-load.sh default
	}
	return p, nil
}

// splitFrontmatter splits "---\n<frontmatter>\n---\n<body>".
func splitFrontmatter(text string) (string, string, error) {
	if !strings.HasPrefix(text, "---\n") {
		return "", "", errors.New("missing leading '---'")
	}
	rest := text[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		// allow file ending with --- and no body
		end = strings.Index(rest, "\n---")
		if end < 0 {
			return "", "", errors.New("missing closing '---'")
		}
	}
	fm := rest[:end]
	bodyStart := end + len("\n---\n")
	body := ""
	if bodyStart <= len(rest) {
		body = rest[bodyStart:]
	}
	return fm, body, nil
}

// parsePluginFrontmatter handles scalar fields plus
// `required_sections` in inline `[a, b]` or block (- item) form.
func parsePluginFrontmatter(text string) (Plugin, error) {
	var p Plugin
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		val := strings.TrimSpace(line[colon+1:])
		switch key {
		case "name":
			p.Name = val
		case "description":
			p.Description = val
		case "output_type":
			p.OutputType = val
		case "output_subtype":
			p.OutputSubtype = val
		case "min_sources":
			p.MinSources, _ = strconv.Atoi(val)
		case "max_sources":
			p.MaxSources, _ = strconv.Atoi(val)
		case "max_evidence_total_words":
			p.MaxEvidenceTotalWords, _ = strconv.Atoi(val)
		case "post_hook":
			p.PostHook = val
		case "render":
			p.Render = val
		case "version":
			p.Version = val
		case "required_sections":
			if strings.HasPrefix(val, "[") {
				p.RequiredSections = parseInlineList(val)
			} else if val == "" {
				// block form
				j := i + 1
				for j < len(lines) {
					l := lines[j]
					t := strings.TrimSpace(l)
					if t == "" || !strings.HasPrefix(t, "- ") {
						break
					}
					p.RequiredSections = append(p.RequiredSections, strings.TrimSpace(strings.TrimPrefix(t, "- ")))
					j++
				}
				i = j - 1
			}
		}
	}
	return p, nil
}

func parseInlineList(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return out
}
