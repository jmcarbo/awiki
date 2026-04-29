package query

import (
	"crypto/sha1"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	reDeterminismBanned = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bNOW\s*\(`),
		regexp.MustCompile(`(?i)\bCURRENT_TIMESTAMP\b`),
		regexp.MustCompile(`(?i)\bCURRENT_DATE\b`),
		regexp.MustCompile(`(?i)\bCURRENT_TIME\b`),
		regexp.MustCompile(`(?i)\bRANDOM\s*\(`),
		regexp.MustCompile(`(?i)\bUUID\s*\(`),
		regexp.MustCompile(`(?i)\bGEN_RANDOM_UUID\s*\(`),
	}

	reOrderBy = regexp.MustCompile(`(?i)\bORDER\s+BY\b`)
)

// IsDeterministic checks whether sql is suitable for materialization.
// Returns an error describing the first non-determinism issue found,
// or nil if the SQL is acceptable.
//
// Rules (matches query-determinism.py):
//   - Banned tokens: NOW(), CURRENT_TIMESTAMP, etc.
//   - Top-level statement must contain ORDER BY.
func IsDeterministic(sql string) error {
	stripped := stripStringsAndComments(sql)
	for _, pat := range reDeterminismBanned {
		m := pat.FindString(stripped)
		if m != "" {
			return fmt.Errorf("QUERY|ERROR|non-deterministic token: %s", strings.TrimSpace(m))
		}
	}
	top := topLevel(stripped)
	if !reOrderBy.MatchString(top) {
		return fmt.Errorf("QUERY|ERROR|materialized SQL must have ORDER BY on the outer SELECT")
	}
	return nil
}

// topLevel returns sql with all parenthesised sub-expressions removed
// (matches query-determinism.py _toplevel).
func topLevel(sql string) string {
	var out strings.Builder
	depth := 0
	for _, ch := range sql {
		switch ch {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				out.WriteRune(ch)
			}
		}
	}
	return out.String()
}

// CanonicalHash computes a deterministic hash of a Result independent of
// row order. Rows are sorted lexicographically by their joined cell values;
// the hash is SHA1 of "<col-list>\n<sorted-rows>".
//
// Column order matters; row order does not.
func CanonicalHash(result Result) string {
	// Stringify and sort rows.
	rowStrings := make([]string, len(result.Rows))
	for i, row := range result.Rows {
		rowStrings[i] = strings.Join(row, "\x00")
	}
	sort.Strings(rowStrings)

	colLine := strings.Join(result.Columns, ",")
	rowsBlock := strings.Join(rowStrings, "\n")

	payload := colLine + "\n" + rowsBlock
	h := sha1.Sum([]byte(payload))
	return fmt.Sprintf("%x", h)
}
