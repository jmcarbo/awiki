package git

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// RepoConfig is one entry under `repos:` in `.awiki/git-sources.yml`.
// Mirrors the JSON shape printed by `awiki_git_config_get`
// (scripts/lib/git-config.sh:71-83).
type RepoConfig struct {
	Name            string   `json:"name"`
	URL             string   `json:"url"`
	Paths           []string `json:"paths"`
	Exclude         []string `json:"exclude"`
	Private         bool     `json:"private"`
	PrivateExplicit bool     `json:"private_explicit"`
	ProtectEdits    bool     `json:"protect_edits"`
	Summarize       bool     `json:"summarize"`
	DefaultBranch   string   `json:"default_branch"`
}

var nameRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// ValidateName mirrors `awiki_git_config_validate_name`
// (scripts/lib/git-config.sh:9-15) — the name must match
// `^[a-z][a-z0-9-]*$`.
func ValidateName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid repo name (need ^[a-z][a-z0-9-]*$): %s", name)
	}
	return nil
}

// ValidatePaths mirrors `awiki_git_config_validate_paths`
// (scripts/lib/git-config.sh:18-37): every entry must be a non-empty
// string with no `..` segment and no leading `/`.
func ValidatePaths(paths []string) error {
	for _, p := range paths {
		segs := strings.Split(p, "/")
		for _, s := range segs {
			if s == ".." {
				return fmt.Errorf("'..' not allowed in path: %s", p)
			}
		}
		if strings.HasPrefix(p, "/") {
			return fmt.Errorf("leading '/' not allowed in path: %s", p)
		}
	}
	return nil
}

// LoadRepoConfig reads yamlPath and returns the named repo entry.
// Mirrors `awiki_git_config_get` (scripts/lib/git-config.sh:39-86):
//
//   - exit 12 on missing config or unknown repo (we surface as error).
//   - default `paths` falls back to README.md, docs/, rfcs/, adr/.
//   - `private: true` is auto-set on SSH URLs unless the field is
//     explicitly present in the YAML.
//   - `private_explicit` records whether the YAML pinned the field.
func LoadRepoConfig(yamlPath, repoName string) (RepoConfig, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return RepoConfig{}, fmt.Errorf("config not found: %s", yamlPath)
		}
		return RepoConfig{}, fmt.Errorf("config read %s: %w", yamlPath, err)
	}
	doc, err := parseSourcesYAML(string(data))
	if err != nil {
		return RepoConfig{}, fmt.Errorf("yaml parse: %w", err)
	}
	if doc.schema != 1 {
		return RepoConfig{}, errors.New("missing or wrong schema (need schema: 1)")
	}
	for _, raw := range doc.repos {
		name, _ := raw["name"].(string)
		if name != repoName {
			continue
		}
		if err := ValidateName(name); err != nil {
			return RepoConfig{}, fmt.Errorf("invalid name: %s", name)
		}
		url, _ := raw["url"].(string)
		paths := stringSlice(raw["paths"])
		if len(paths) == 0 {
			paths = []string{"README.md", "docs/", "rfcs/", "adr/"}
		}
		exclude := stringSlice(raw["exclude"])
		_, privateExplicit := raw["private"]
		private := boolField(raw["private"])
		// SSH url auto-flips private — only when explicit private flag
		// is not set. Mirrors git-config.sh:69-71.
		if strings.HasPrefix(url, "git@") && !privateExplicit {
			private = true
		}
		return RepoConfig{
			Name:            name,
			URL:             url,
			Paths:           paths,
			Exclude:         exclude,
			Private:         private,
			PrivateExplicit: privateExplicit,
			ProtectEdits:    boolField(raw["protect_edits"]),
			Summarize:       boolField(raw["summarize"]),
			DefaultBranch:   stringField(raw["default_branch"]),
		}, nil
	}
	return RepoConfig{}, fmt.Errorf("no repo named: %s", repoName)
}

func stringField(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func boolField(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func stringSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// ----- minimal YAML parser for `.awiki/git-sources.yml` --------------
//
// We do not pull in a YAML dependency for v1 because the on-disk shape
// is fixed and shallow:
//
//	schema: <int>
//	repos:
//	  - name: <str>
//	    url:  <str>
//	    paths: [<str>, ...]      # flow sequence on a single line
//	    exclude: [<str>, ...]
//	    private: true|false
//	    ...
//
// The parser supports exactly that subset: top-level scalars, a `repos`
// list of inline-key maps, and `[a, b, c]` flow sequences (with optional
// quotes). Block scalar values, multi-line sequences, anchors, etc. are
// out of scope — the v1 spec for `.awiki/git-sources.yml` is identical
// to what scripts/lib/git-config.sh feeds to PyYAML, and the bash
// fixtures we ship today (tests/fixtures/git-sources-good.yml,
// tests/fixtures/git-sources-bad.yml) live entirely within the subset.

type sourcesDoc struct {
	schema int
	repos  []map[string]interface{}
}

func parseSourcesYAML(text string) (sourcesDoc, error) {
	var doc sourcesDoc
	lines := strings.Split(text, "\n")
	i := 0
	for i < len(lines) {
		raw := lines[i]
		// Strip comments. YAML-style: '#' starts a comment when
		// preceded by whitespace or beginning-of-line.
		stripped := stripComment(raw)
		if strings.TrimSpace(stripped) == "" {
			i++
			continue
		}
		indent := leadingSpaces(stripped)
		if indent != 0 {
			i++
			continue
		}
		key, val, ok := splitKey(stripped)
		if !ok {
			return doc, fmt.Errorf("expected key: at line %d: %q", i+1, raw)
		}
		switch key {
		case "schema":
			n, err := strconv.Atoi(strings.TrimSpace(val))
			if err != nil {
				return doc, fmt.Errorf("schema must be int (line %d): %w", i+1, err)
			}
			doc.schema = n
			i++
		case "repos":
			i++
			repos, next, err := parseReposBlock(lines, i)
			if err != nil {
				return doc, err
			}
			doc.repos = repos
			i = next
		default:
			i++
		}
	}
	return doc, nil
}

// parseReposBlock consumes contiguous list items at the same indent
// starting at start. Returns (entries, indexOfFirstUnconsumedLine).
func parseReposBlock(lines []string, start int) ([]map[string]interface{}, int, error) {
	out := []map[string]interface{}{}
	i := start
	for i < len(lines) {
		raw := lines[i]
		stripped := stripComment(raw)
		if strings.TrimSpace(stripped) == "" {
			i++
			continue
		}
		// Check if this is a list item (starts with `-` after some indent).
		dashIdx := strings.Index(stripped, "-")
		if dashIdx < 0 || strings.TrimSpace(stripped[:dashIdx]) != "" {
			// Not a list item at any indent — back to top-level.
			return out, i, nil
		}
		listIndent := dashIdx
		entry := map[string]interface{}{}
		// First key on the same line as the dash:
		//    - name: foo
		afterDash := strings.TrimLeft(stripped[dashIdx+1:], " \t")
		if afterDash != "" {
			k, v, ok := splitKey(afterDash)
			if !ok {
				return nil, 0, fmt.Errorf("expected key: after `-` on line %d: %q", i+1, raw)
			}
			entry[k] = parseScalar(v)
		}
		i++
		// Consume subsequent lines that are indented strictly more than
		// the dash (i.e. continuation of this list entry).
		for i < len(lines) {
			raw2 := lines[i]
			stripped2 := stripComment(raw2)
			if strings.TrimSpace(stripped2) == "" {
				i++
				continue
			}
			ind2 := leadingSpaces(stripped2)
			if ind2 <= listIndent {
				break
			}
			k, v, ok := splitKey(stripped2)
			if !ok {
				return nil, 0, fmt.Errorf("expected key: at line %d: %q", i+1, raw2)
			}
			entry[k] = parseScalar(v)
			i++
		}
		out = append(out, entry)
	}
	return out, i, nil
}

// stripComment removes the trailing `# ...` comment from a YAML line,
// preserving any `#` that lives inside quotes. v1 is permissive — the
// fixtures we ship use comments only at end-of-line.
func stripComment(line string) string {
	inSingle := false
	inDouble := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch c {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				if i == 0 || line[i-1] == ' ' || line[i-1] == '\t' {
					return line[:i]
				}
			}
		}
	}
	return line
}

func leadingSpaces(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' {
			return i
		}
	}
	return len(s)
}

// splitKey splits "key: value" into (key, value). Returns ok=false when
// no colon-separator is present at the top level of the line.
func splitKey(line string) (string, string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	idx := strings.Index(trimmed, ":")
	if idx < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(trimmed[:idx])
	val := ""
	if idx+1 < len(trimmed) {
		val = strings.TrimLeft(trimmed[idx+1:], " \t")
	}
	return key, val, true
}

// parseScalar turns a raw YAML scalar (or `[a, b, c]` flow sequence)
// into a Go value. Recognises bool, int, quoted strings, and
// single-line `[...]` arrays. Unknown shapes are returned as the raw
// trimmed string.
func parseScalar(v string) interface{} {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
		inner := strings.TrimSpace(v[1 : len(v)-1])
		if inner == "" {
			return []interface{}{}
		}
		parts := splitFlowList(inner)
		out := make([]interface{}, 0, len(parts))
		for _, p := range parts {
			out = append(out, parseScalar(p))
		}
		return out
	}
	switch v {
	case "true", "True", "TRUE":
		return true
	case "false", "False", "FALSE":
		return false
	case "null", "Null", "NULL", "~":
		return nil
	}
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') ||
			(v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}

// splitFlowList splits the body of a `[ ... ]` flow sequence on commas,
// honoring quoted strings.
func splitFlowList(s string) []string {
	out := []string{}
	depth := 0
	inSingle := false
	inDouble := false
	last := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '[':
			if !inSingle && !inDouble {
				depth++
			}
		case ']':
			if !inSingle && !inDouble {
				depth--
			}
		case ',':
			if !inSingle && !inDouble && depth == 0 {
				out = append(out, strings.TrimSpace(s[last:i]))
				last = i + 1
			}
		}
	}
	out = append(out, strings.TrimSpace(s[last:]))
	return out
}
