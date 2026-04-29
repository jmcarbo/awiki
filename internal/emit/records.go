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

// AgentPrompt formats an AGENT-PROMPT| record. fields are emitted in
// the order given (the bash callers preserve insertion order).
func AgentPrompt(verb string, fields []string) string {
	if len(fields) == 0 {
		return "AGENT-PROMPT|" + verb
	}
	return "AGENT-PROMPT|" + verb + "|" + strings.Join(fields, "|")
}

// Ingest formats an INGEST| record.
func Ingest(format, source, status string) string {
	return fmt.Sprintf("INGEST|%s|%s|%s", format, source, status)
}

// Review formats a REVIEW| record. extra fields are joined with '|'.
func Review(kind, period string, fields []string) string {
	head := fmt.Sprintf("REVIEW|%s|%s", kind, period)
	if len(fields) == 0 {
		return head
	}
	return head + "|" + strings.Join(fields, "|")
}

// Recur formats a RECUR| record produced by action-recur.
func Recur(file, blockID string, fields ...string) string {
	head := fmt.Sprintf("RECUR|%s|%s", file, blockID)
	if len(fields) == 0 {
		return head
	}
	return head + "|" + strings.Join(fields, "|")
}
