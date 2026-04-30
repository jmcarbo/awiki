package template

import "strings"

// EscapePlanField encodes a single field value for the PLAN|... line
// format. Mirrors `escape.escape` in scripts/_template_helpers/escape.py:
// percent first, then pipe, then newline. Tab and carriage return are
// not escaped — the bash oracle never emits them in plan fields.
func EscapePlanField(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "|", "%7C")
	s = strings.ReplaceAll(s, "\n", "%0A")
	return s
}
