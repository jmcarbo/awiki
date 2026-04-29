package query

import (
	"strings"
)

// ExtractFences finds all ```awiki-query [out=<slug>] ... ``` blocks in text
// and returns their FenceSpec descriptors. Each FenceSpec carries the SQL
// body, an optional Out slug parsed from the info-line, and byte offsets
// marking the entire fence block (opening fence line through closing ```).
//
// Ill-formed fences (no closing ```) are silently skipped.
func ExtractFences(text string) []FenceSpec {
	var result []FenceSpec
	pos := 0
	for pos < len(text) {
		// Find the next opening fence line that starts with ```awiki-query
		lineStart := pos
		lineEnd := strings.Index(text[pos:], "\n")
		if lineEnd < 0 {
			break
		}
		lineEnd += pos // absolute index of \n

		line := text[lineStart:lineEnd]
		// Check for ```awiki-query ...
		if !strings.HasPrefix(strings.TrimSpace(line), "```awiki-query") {
			pos = lineEnd + 1
			continue
		}

		// Parse the info-line: ```awiki-query [out=<slug>]
		info := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "```awiki-query"))
		outSlug := ""
		for _, part := range strings.Fields(info) {
			if strings.HasPrefix(part, "out=") {
				outSlug = strings.TrimPrefix(part, "out=")
			}
		}

		// Collect body lines until closing ```
		bodyStart := lineEnd + 1 // byte offset right after the opening \n
		closePos := -1
		searchPos := bodyStart
		for searchPos < len(text) {
			nl := strings.Index(text[searchPos:], "\n")
			if nl < 0 {
				// No newline found; check remaining text.
				remaining := strings.TrimSpace(text[searchPos:])
				if remaining == "```" {
					closePos = searchPos + len(text[searchPos:]) - len(strings.TrimLeft(text[searchPos:], " \t"))
				}
				break
			}
			nlAbs := searchPos + nl
			candidate := strings.TrimSpace(text[searchPos:nlAbs])
			if candidate == "```" {
				closePos = nlAbs + 1 // byte offset just past the closing fence's \n
				break
			}
			searchPos = nlAbs + 1
		}

		if closePos < 0 {
			// Ill-formed: no closing fence; skip
			pos = lineEnd + 1
			continue
		}

		// SQL body is from bodyStart to the line before the closing ```.
		// Find where the closing ``` line starts.
		closingLineStart := strings.LastIndex(text[bodyStart:closePos], "\n```")
		var sqlBody string
		if closingLineStart < 0 {
			// closing ``` is on first body line
			sqlBody = ""
		} else {
			sqlBody = text[bodyStart : bodyStart+closingLineStart]
		}

		result = append(result, FenceSpec{
			SQL:   sqlBody,
			Out:   outSlug,
			Start: lineStart,
			End:   closePos,
		})
		pos = closePos
	}
	return result
}
