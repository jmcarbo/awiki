package synth

import (
	"regexp"
	"strings"

	"awiki/internal/region"
)

// ClearRegion replaces the body of the BEGIN GENERATED region in
// `text` with a single blank line, and rewrites the BEGIN line to
// declare the given plugin and scope_hash. Returns the new text.
// If markers are missing/duplicate/inverted, returns the input
// unchanged plus an error.
func ClearRegion(text, plugin, scopeHash string) (string, error) {
	gen, diags := region.ParseGenerated(text)
	if len(diags) > 0 {
		return text, &MarkerError{Diagnostics: diags}
	}
	beginLine := "<!-- BEGIN GENERATED plugin=" + plugin +
		" scope_hash=" + scopeHash + " -->"
	prefix := text[:gen.BeginOffset]
	// gen.Body starts after the BEGIN line's trailing \n; gen.EndOffset
	// is the start of the END marker.
	endStart := gen.EndOffset
	suffix := text[endStart:]
	// New region: BEGIN line + \n + blank line + \n? Bash output:
	//   <BEGIN line>\n
	//   \n
	//   <END marker>
	body := beginLine + "\n\n"
	return prefix + body + suffix, nil
}

// CheckMarkers verifies BEGIN GENERATED marker count and order.
// Returns nil on valid markers; *MarkerError otherwise. Matches
// scripts/synth.sh:synth_check_markers — exactly one BEGIN and one
// END, BEGIN before END.
func CheckMarkers(text string) error {
	begins := strings.Count(text, "<!-- BEGIN GENERATED ")
	ends := strings.Count(text, "<!-- END GENERATED -->")
	if begins != 1 || ends != 1 {
		return &MarkerError{Message: "marker integrity failure: BEGIN=" + itoa(begins) +
			" END=" + itoa(ends)}
	}
	bIdx := strings.Index(text, "<!-- BEGIN GENERATED ")
	eIdx := strings.Index(text, "<!-- END GENERATED -->")
	if bIdx > eIdx {
		return &MarkerError{Message: "BEGIN marker after END marker"}
	}
	return nil
}

var scopeHashRE = regexp.MustCompile(`scope_hash=[0-9a-f]+`)

// SetScopeHashInMarker replaces the scope_hash=… token in the BEGIN
// GENERATED line. No-op if the line has no scope_hash.
func SetScopeHashInMarker(text, hash string) string {
	gen, diags := region.ParseGenerated(text)
	if len(diags) > 0 {
		return text
	}
	begin := text[gen.BeginOffset : gen.BeginOffset+len(gen.BeginLine)]
	updated := scopeHashRE.ReplaceAllString(begin, "scope_hash="+hash)
	return text[:gen.BeginOffset] + updated + text[gen.BeginOffset+len(gen.BeginLine):]
}

// MarkerError is returned by ClearRegion / CheckMarkers when the
// BEGIN/END markers are missing, duplicated, or out of order.
type MarkerError struct {
	Message     string
	Diagnostics []region.Diagnostic
}

func (e *MarkerError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if len(e.Diagnostics) > 0 {
		return e.Diagnostics[0].Message
	}
	return "marker error"
}

func itoa(n int) string {
	// avoids strconv import (already pulled in elsewhere; harmless if duplicated)
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
