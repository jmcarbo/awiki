package dataset

import (
	"strings"
)

// FmGet returns the scalar value of key in the YAML frontmatter, stripped of
// surrounding quotes and whitespace. ok=false when absent or when the text has
// no frontmatter.
func FmGet(text, key string) (string, bool) {
	lines := splitLines(text)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", false
	}
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			break
		}
		n := strings.IndexByte(line, ':')
		if n < 0 {
			continue
		}
		k := strings.TrimSpace(line[:n])
		if k != key {
			continue
		}
		v := strings.TrimSpace(line[n+1:])
		// Strip surrounding double-quotes (single-quotes are kept as-is,
		// matching the awk in dataset-fm.sh which only strips `"`).
		v = strings.TrimPrefix(v, `"`)
		v = strings.TrimSuffix(v, `"`)
		return v, true
	}
	return "", false
}

// FmSet replaces the value of key in the frontmatter. If key is absent but
// frontmatter exists, inserts `<key>: <value>` immediately before the closing
// `---`. If the text has no frontmatter, returns text unchanged.
func FmSet(text, key, value string) string {
	lines := splitLines(text)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return text
	}

	replaced := false
	for i := 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "---" {
			if !replaced {
				// Insert before the closing ---.
				newLine := key + ": " + value
				lines = append(lines[:i], append([]string{newLine}, lines[i:]...)...)
			}
			break
		}
		n := strings.IndexByte(lines[i], ':')
		if n < 0 {
			continue
		}
		k := strings.TrimSpace(lines[i][:n])
		if k == key {
			lines[i] = key + ": " + value
			replaced = true
		}
	}
	return strings.Join(lines, "\n")
}

// FmRemove deletes the key line from the frontmatter. Returns text unchanged
// when key is absent or when the text has no frontmatter.
func FmRemove(text, key string) string {
	lines := splitLines(text)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return text
	}

	out := make([]string, 0, len(lines))
	inFM := false
	opened := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if !opened {
				inFM = true
				opened = true
				out = append(out, line)
				continue
			}
			inFM = false
			out = append(out, line)
			continue
		}
		if inFM {
			n := strings.IndexByte(line, ':')
			if n >= 0 {
				k := strings.TrimSpace(line[:n])
				if k == key {
					continue // drop this line
				}
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// splitLines splits text on newlines, preserving an empty trailing element
// when text ends with a newline.
func splitLines(text string) []string {
	// Use \n as separator; handle \r\n by normalizing first.
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.Split(text, "\n")
}
