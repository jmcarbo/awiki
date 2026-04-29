package dataset

import (
	"errors"
	"strings"
)

// ExtractDataFence returns (content, format, true) when a fenced block exists
// under the "## Data" heading. The format is the fence info-string (e.g.
// "csv"). An empty content with ok=true is a valid empty fence.
func ExtractDataFence(text string) (content []byte, format Format, ok bool) {
	lines := splitLines(text)
	inData := false
	for i, line := range lines {
		if strings.TrimSpace(line) == "## Data" {
			inData = true
			continue
		}
		if !inData {
			continue
		}
		// Look for the opening fence.
		if strings.HasPrefix(line, "```") {
			info := strings.TrimSpace(strings.TrimPrefix(line, "```"))
			var bodyLines []string
			for _, dataLine := range lines[i+1:] {
				if strings.TrimSpace(dataLine) == "```" {
					body := strings.Join(bodyLines, "\n")
					if len(bodyLines) > 0 {
						body += "\n"
					}
					return []byte(body), Format(info), true
				}
				bodyLines = append(bodyLines, dataLine)
			}
			// Unclosed fence: still return what we have.
			body := strings.Join(bodyLines, "\n")
			return []byte(body), Format(info), true
		}
	}
	return nil, "", false
}

// ReplaceDataFence rewrites the fence body and info-string under "## Data".
// If "## Data" exists but contains no fence, a new fenced block is inserted.
// If "## Data" is missing, the text is returned unchanged with an error.
func ReplaceDataFence(text string, format Format, content []byte) (string, error) {
	lines := splitLines(text)

	dataIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "## Data" {
			dataIdx = i
			break
		}
	}
	if dataIdx < 0 {
		return text, errors.New("## Data section not found")
	}

	// Find existing fence after ## Data heading.
	fenceOpen := -1
	fenceClose := -1
	for i := dataIdx + 1; i < len(lines); i++ {
		line := lines[i]
		// Stop searching if we hit another heading.
		if strings.HasPrefix(line, "#") && fenceOpen < 0 {
			break
		}
		if strings.HasPrefix(line, "```") {
			if fenceOpen < 0 {
				fenceOpen = i
			} else if strings.TrimSpace(line) == "```" {
				fenceClose = i
				break
			}
		}
	}

	// Build the replacement fence lines.
	contentStr := string(content)
	if len(contentStr) > 0 && !strings.HasSuffix(contentStr, "\n") {
		contentStr += "\n"
	}
	fenceLines := []string{"```" + string(format)}
	fenceLines = append(fenceLines, strings.Split(strings.TrimSuffix(contentStr, "\n"), "\n")...)
	fenceLines = append(fenceLines, "```")

	var out []string
	if fenceOpen < 0 {
		// No existing fence: insert after the ## Data heading.
		out = append(out, lines[:dataIdx+1]...)
		out = append(out, "")
		out = append(out, fenceLines...)
		out = append(out, lines[dataIdx+1:]...)
	} else if fenceClose < 0 {
		// Unclosed fence: replace from fenceOpen to end.
		out = append(out, lines[:fenceOpen]...)
		out = append(out, fenceLines...)
	} else {
		// Replace the block from fenceOpen to fenceClose (inclusive).
		out = append(out, lines[:fenceOpen]...)
		out = append(out, fenceLines...)
		out = append(out, lines[fenceClose+1:]...)
	}

	return strings.Join(out, "\n"), nil
}

// RemoveDataFence strips the fenced block under "## Data", leaving the
// heading but making the section empty. Returns text unchanged if there is no
// fence under "## Data".
func RemoveDataFence(text string) string {
	lines := splitLines(text)

	dataIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "## Data" {
			dataIdx = i
			break
		}
	}
	if dataIdx < 0 {
		return text
	}

	fenceOpen := -1
	fenceClose := -1
	for i := dataIdx + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "#") && fenceOpen < 0 {
			break
		}
		if strings.HasPrefix(line, "```") {
			if fenceOpen < 0 {
				fenceOpen = i
			} else if strings.TrimSpace(line) == "```" {
				fenceClose = i
				break
			}
		}
	}

	if fenceOpen < 0 {
		return text
	}

	var out []string
	if fenceClose < 0 {
		// Unclosed fence: remove from fenceOpen to end.
		out = append(out, lines[:fenceOpen]...)
	} else {
		out = append(out, lines[:fenceOpen]...)
		out = append(out, lines[fenceClose+1:]...)
	}
	return strings.Join(out, "\n")
}
