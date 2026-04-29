// Package region parses awiki's two managed-region marker styles.
package region

import "strings"

const (
	beginMarker = "<!-- BEGIN GENERATED"
	endMarker   = "<!-- END GENERATED -->"
)

type Generated struct {
	BeginOffset int
	EndOffset   int
	Body        string
	BeginLine   string
}

type Diagnostic struct {
	Code    string
	Message string
}

func ParseGenerated(text string) (Generated, []Diagnostic) {
	beginOffsets := markerOffsets(text, beginMarker)
	endOffsets := markerOffsets(text, endMarker)
	var diagnostics []Diagnostic
	if len(beginOffsets) != 1 {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    "S1",
			Message: "expected exactly one BEGIN GENERATED marker",
		})
	}
	if len(endOffsets) != 1 {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    "S1",
			Message: "expected exactly one END GENERATED marker",
		})
	}
	if len(beginOffsets) != 1 || len(endOffsets) != 1 {
		return Generated{BeginOffset: -1, EndOffset: -1}, diagnostics
	}

	beginOffset := beginOffsets[0]
	endOffset := endOffsets[0]
	if beginOffset > endOffset {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    "S1",
			Message: "BEGIN GENERATED marker must appear before END GENERATED marker",
		})
		return Generated{BeginOffset: beginOffset, EndOffset: endOffset}, diagnostics
	}

	beginLineEnd := strings.IndexByte(text[beginOffset:], '\n')
	if beginLineEnd < 0 {
		beginLineEnd = len(text) - beginOffset
	}
	beginLineEnd += beginOffset
	bodyStart := beginLineEnd
	if bodyStart < len(text) && text[bodyStart] == '\n' {
		bodyStart++
	}
	return Generated{
		BeginOffset: beginOffset,
		EndOffset:   endOffset,
		Body:        text[bodyStart:endOffset],
		BeginLine:   text[beginOffset:beginLineEnd],
	}, diagnostics
}

func markerOffsets(text string, marker string) []int {
	var offsets []int
	searchFrom := 0
	for {
		next := strings.Index(text[searchFrom:], marker)
		if next < 0 {
			return offsets
		}
		offset := searchFrom + next
		offsets = append(offsets, offset)
		searchFrom = offset + len(marker)
	}
}
