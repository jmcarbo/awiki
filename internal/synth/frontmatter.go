package synth

import (
	"strings"
)

// FmSetScalar replaces the `<key>: ...` line in the YAML frontmatter
// at the start of body. If the key is absent, the input is returned
// unchanged. Matches scripts/synth.sh:synth_fm_set_scalar (which only
// rewrites; never inserts).
func FmSetScalar(body, key, value string) string {
	prefix := "---\n"
	if !strings.HasPrefix(body, prefix) {
		return body
	}
	rest := body[len(prefix):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return body
	}
	fm := rest[:end]
	tail := rest[end:]
	lines := strings.Split(fm, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, key+":") {
			lines[i] = key + ": " + value
			return prefix + strings.Join(lines, "\n") + tail
		}
	}
	return body
}

// FmSetSources rewrites the `sources:` line in frontmatter to
// inline-list form: ["[[s1]]", "[[s2]]"]. Slugs already sorted.
// Matches scripts/synth.sh:synth_fm_set_sources.
func FmSetSources(body string, slugs []string) string {
	parts := make([]string, 0, len(slugs))
	for _, s := range slugs {
		if s == "" {
			continue
		}
		parts = append(parts, `"[[`+s+`]]"`)
	}
	value := "[" + strings.Join(parts, ", ") + "]"
	return FmSetScalar(body, "sources", value)
}
