// Package action parses awiki task-action lines following the grammar in scripts/lib/action-grammar.sh.
package action

import (
	"regexp"
	"strings"
	"time"
)

var actionLinePattern = regexp.MustCompile(`^\s*[-*]\s+\[([^\]])\]\s+(.+)$`)
var contextPattern = regexp.MustCompile(`^@[a-z0-9][a-z0-9-]*$`)
var blockIDPattern = regexp.MustCompile(`^\^([^\s]+)$`)
var validIDPattern = regexp.MustCompile(`^[a-z0-9]{3,16}(~[0-9]+)?$`)
var validPlainIDPattern = regexp.MustCompile(`^[a-z0-9]{3,16}$`)
var validChainIDPattern = regexp.MustCompile(`^[a-z0-9]{3,16}~[0-9]+$`)
var tokenKeyPattern = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_-]*):(.+)$`)

var allowedTailKeys = map[string]bool{
	"due": true, "defer": true, "wait": true, "since": true,
	"every": true, "done": true, "priority": true, "est": true,
}

// Action represents a parsed awiki task-action line.
type Action struct {
	ID        string
	Status    string
	Text      string
	File      string
	AbsPath   string
	Line      int
	Context   string
	Due       string
	Defer     string
	Wait      string
	Since     string
	Every     string
	Done      string
	Priority  string
	Estimate  string
	Project   string
	Raw       string
	TailKeys  map[string]string
	HasBadID  bool
	BadID     string
	BadDate   string
	BadKey    string
	BadStatus string
}

// ParseLine parses a single action line (e.g. "- [ ] do thing @ctx due:2026-01-01 ^abc123").
// Returns the parsed Action and true on success; zero value and false if the line does not match.
func ParseLine(line string) (Action, bool) {
	match := actionLinePattern.FindStringSubmatch(line)
	if len(match) == 0 {
		return Action{}, false
	}
	action := Action{
		Status:   match[1],
		Raw:      line,
		TailKeys: make(map[string]string),
	}
	if !strings.Contains(" /?>x-", action.Status) {
		action.BadStatus = action.Status
		return action, true
	}
	body := match[2]
	if regexp.MustCompile(`^\[.\]\s+`).MatchString(body) {
		return Action{}, false
	}
	var text []string
	for _, tok := range strings.Fields(body) {
		switch {
		case action.Context == "" && contextPattern.MatchString(tok):
			action.Context = tok
		case blockIDPattern.MatchString(tok):
			id := strings.TrimPrefix(tok, "^")
			action.ID = id
			if !validIDPattern.MatchString(id) {
				action.HasBadID = true
				action.BadID = id
			}
		case tokenKeyPattern.MatchString(tok):
			parts := tokenKeyPattern.FindStringSubmatch(tok)
			key, value := parts[1], parts[2]
			if !allowedTailKeys[key] {
				action.BadKey = key
				continue
			}
			action.TailKeys[key] = value
			switch key {
			case "due":
				action.Due = value
			case "defer":
				action.Defer = value
			case "wait":
				action.Wait = value
			case "since":
				action.Since = value
			case "every":
				action.Every = value
			case "done":
				action.Done = value
			case "priority":
				action.Priority = value
			case "est":
				action.Estimate = value
			}
			if isDateKey(key) && !validDate(value) {
				action.BadDate = key + ":" + value
			}
		default:
			if action.Context == "" && len(action.TailKeys) == 0 && action.ID == "" {
				text = append(text, tok)
			}
		}
	}
	action.Text = strings.Join(text, " ")
	return action, true
}

func isDateKey(key string) bool {
	return key == "due" || key == "defer" || key == "since" || key == "done"
}

func validDate(value string) bool {
	if len(value) != 10 {
		return false
	}
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

// ActionLinePattern returns the compiled regexp that matches action lines.
// Exported for use by consumers that need to check a line without fully parsing it.
func ActionLinePattern() *regexp.Regexp {
	return actionLinePattern
}

// ContextPattern returns the compiled regexp that matches context tokens (@ctx).
func ContextPattern() *regexp.Regexp {
	return contextPattern
}

// BlockIDPattern returns the compiled regexp that matches block-ID tokens (^id).
func BlockIDPattern() *regexp.Regexp {
	return blockIDPattern
}

// TokenKeyPattern returns the compiled regexp for key:value tail tokens.
func TokenKeyPattern() *regexp.Regexp {
	return tokenKeyPattern
}
