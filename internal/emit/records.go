// Package emit owns the user-visible record formats awiki commands
// write to stdout. Domain code never assembles record strings by hand;
// it calls the typed formatters here.
package emit

import (
	"fmt"
	"strings"
)

// Lint formats a LINT| record. code may be empty.
func Lint(level, file, code, message string) string {
	if code == "" {
		return fmt.Sprintf("LINT|%s|%s|%s", level, file, message)
	}
	return fmt.Sprintf("LINT|%s|%s|%s|%s", level, file, code, message)
}

// Fix formats a FIX| record.
func Fix(file, message string) string {
	return fmt.Sprintf("FIX|%s|%s", file, message)
}

// LintSummary formats the trailing summary line for `awiki lint`.
func LintSummary(errors, warnings, infos int) string {
	return fmt.Sprintf("LINT-SUMMARY|errors=%d|warnings=%d|info=%d",
		errors, warnings, infos)
}

// AgentPrompt formats an AGENT-PROMPT| record. The bash original emits
// a single free-form prompt string after the prefix
// (scripts/ingest.sh:115).
func AgentPrompt(prompt string) string {
	return "AGENT-PROMPT|" + prompt
}

// Review formats a REVIEW| record. The bash original is
// `REVIEW|<key>|<k=v>[|<k=v>...]` (scripts/review-status.sh). fields
// is the list of `<k=v>` tokens after the key.
func Review(key string, fields []string) string {
	if len(fields) == 0 {
		return "REVIEW|" + key
	}
	return "REVIEW|" + key + "|" + strings.Join(fields, "|")
}
