package synth

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// ParseScopeFromFrontmatter reads a raw frontmatter text block (between
// the --- markers, NOT including them) and extracts the scope: block.
// It mirrors scripts/synth.sh:synth_read_scope with a flat line-by-line
// approach: enter scope: on "^scope:" then collect "^  <key>:" lines.
func ParseScopeFromFrontmatter(text string) Scope {
	var s Scope
	lines := strings.Split(text, "\n")
	inScope := false
	for _, line := range lines {
		// Detect "scope:" header (no value on same line).
		if line == "scope:" || strings.HasPrefix(line, "scope: ") {
			// Only enter scope block if it's the bare "scope:" key.
			if line == "scope:" {
				inScope = true
				continue
			}
		}
		if inScope {
			// Leave scope block when a non-indented, non-empty line appears.
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
				inScope = false
				continue
			}
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			colon := strings.IndexByte(trimmed, ':')
			if colon < 0 {
				continue
			}
			key := trimmed[:colon]
			val := strings.TrimSpace(trimmed[colon+1:])
			switch key {
			case "tag":
				s.Tag = val
			case "slugs":
				// strip surrounding [ ] if present
				val = strings.TrimPrefix(val, "[")
				val = strings.TrimSuffix(val, "]")
				s.Slugs = splitCommaList(val)
			case "query":
				// strip surrounding quotes if present
				val = strings.Trim(val, `"'`)
				s.Query = val
			case "exclude_tags":
				val = strings.TrimPrefix(val, "[")
				val = strings.TrimSuffix(val, "]")
				s.ExcludeTags = splitCommaList(val)
			case "min_last_updated":
				s.MinLastUpdated = val
			case "types":
				val = strings.TrimPrefix(val, "[")
				val = strings.TrimSuffix(val, "]")
				s.Types = splitCommaList(val)
			case "allow_private":
				s.AllowPrivate = (val == "true" || val == "1")
			}
		}
	}

	// Infer Kind from which fields are set.
	if s.Tag != "" {
		s.Kind = "tag"
	} else if len(s.Slugs) > 0 {
		s.Kind = "slugs"
	} else if s.Query != "" {
		s.Kind = "query"
	}

	return s
}

// splitCommaList splits a comma-separated string, trimming whitespace and
// removing quotes and empty entries.
func splitCommaList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.Trim(strings.TrimSpace(p), `"'`)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// ScopeHash matches scripts/synth.sh:synth_scope_hash and
// scripts/lint-synth-hash.py: sha256 of sorted slugs joined with "\n"
// plus a trailing "\n", first 6 hex chars.
//
// Verified: sha256("a\nb\nc\n")[:6] == "880553"
// (matches: printf 'a\nb\nc\n' | python3 scripts/lint-synth-hash.py)
func ScopeHash(slugs []string) string {
	sorted := append([]string(nil), slugs...)
	sort.Strings(sorted)
	var sb strings.Builder
	for _, s := range sorted {
		sb.WriteString(s)
		sb.WriteByte('\n')
	}
	h := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(h[:])[:6]
}
