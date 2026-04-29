package task

import (
	"regexp"

	"awiki/internal/action"
)

// Action is a type alias for action.Action so existing rule code compiles unchanged.
type Action = action.Action

// ParseActionLine parses a single task-action line.
// It delegates to action.ParseLine; existing callers need no changes.
func ParseActionLine(line string) (Action, bool) {
	return action.ParseLine(line)
}

// Package-level pattern variables used by scan.go and fix.go.
// These are thin references to the canonical vars in the action package.
var (
	actionLinePattern = action.ActionLinePattern()
	contextPattern    = action.ContextPattern()
	blockIDPattern    = action.BlockIDPattern()
	tokenKeyPattern   = action.TokenKeyPattern()
)

func normalizeDateToken(value string) string {
	parts := regexp.MustCompile(`^([0-9]{4})/([0-9]{1,2})/([0-9]{1,2})$`).FindStringSubmatch(value)
	if len(parts) == 0 {
		return value
	}
	year := parts[1]
	month := parts[2]
	day := parts[3]
	if len(month) == 1 {
		month = "0" + month
	}
	if len(day) == 1 {
		day = "0" + day
	}
	return year + "-" + month + "-" + day
}
