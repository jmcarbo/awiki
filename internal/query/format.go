package query

import "strings"

// FormatTable renders a Result as a GitHub-Flavored Markdown table, matching
// the output of query-format.py byte-for-byte.
//
// When result has no rows, returns "(no rows)\n".
// Otherwise produces:
//
//	| col1 | col2 |
//	|---|---|
//	| val1 | val2 |
func FormatTable(result Result) string {
	if len(result.Rows) == 0 {
		return "(no rows)\n"
	}
	cols := result.Columns

	cell := func(v string) string {
		return strings.ReplaceAll(v, "|", `\|`)
	}

	var sb strings.Builder

	// Header row
	sb.WriteString("| ")
	sb.WriteString(strings.Join(cols, " | "))
	sb.WriteString(" |")
	sb.WriteByte('\n')

	// Separator row: |---|---| (no spaces, matching python)
	sb.WriteByte('|')
	for range cols {
		sb.WriteString("---|")
	}
	sb.WriteByte('\n')

	// Data rows
	for _, row := range result.Rows {
		sb.WriteString("| ")
		cells := make([]string, len(cols))
		for i, c := range row {
			if i < len(cols) {
				cells[i] = cell(c)
			}
		}
		sb.WriteString(strings.Join(cells, " | "))
		sb.WriteString(" |")
		sb.WriteByte('\n')
	}

	return sb.String()
}
