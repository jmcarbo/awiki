package region

import (
	"regexp"
	"strings"
)

// ManagedReplace returns text with the named managed region's body
// replaced by body. If the region is absent, ManagedReplace appends a
// new region to the end. The marker style is
// `<!-- BEGIN <kind>:<id> -->` / `<!-- END <kind>:<id> -->`.
//
// body must end with a newline; ManagedReplace adds one if missing.
func ManagedReplace(text, kind, id, body string) (string, error) {
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	begin := "<!-- BEGIN " + kind + ":" + id + " -->"
	end := "<!-- END " + kind + ":" + id + " -->"
	newBlock := begin + "\n" + body + end
	pat, err := managedPattern(kind, id)
	if err != nil {
		return "", err
	}
	if pat.MatchString(text) {
		return pat.ReplaceAllLiteralString(text, newBlock), nil
	}
	sep := ""
	if !strings.HasSuffix(text, "\n") {
		sep = "\n"
	}
	return text + sep + "\n" + newBlock + "\n", nil
}

// ManagedExtract returns the body inside the named region. ok=false
// when the region is absent.
func ManagedExtract(text, kind, id string) (string, bool) {
	pat, err := managedExtractPattern(kind, id)
	if err != nil {
		return "", false
	}
	m := pat.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	return m[1], true
}

func managedPattern(kind, id string) (*regexp.Regexp, error) {
	return regexp.Compile(`(?s)<!-- BEGIN ` + regexp.QuoteMeta(kind) +
		`:` + regexp.QuoteMeta(id) + ` -->\n.*?\n<!-- END ` +
		regexp.QuoteMeta(kind) + `:` + regexp.QuoteMeta(id) + ` -->`)
}

func managedExtractPattern(kind, id string) (*regexp.Regexp, error) {
	return regexp.Compile(`(?s)<!-- BEGIN ` + regexp.QuoteMeta(kind) +
		`:` + regexp.QuoteMeta(id) + ` -->\n(.*?)\n<!-- END ` +
		regexp.QuoteMeta(kind) + `:` + regexp.QuoteMeta(id) + ` -->`)
}
