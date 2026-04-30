package template

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// bootstrapStepRe mirrors STEP_RE in
// scripts/_template_helpers/bootstrap_hash.py.
var bootstrapStepRe = regexp.MustCompile(`<!--\s*bootstrap-step:\s*([a-z0-9-]+)\s*-->`)

// bootstrapHeadingRe mirrors HEADING_RE: a line starting with "### "
// (multiline mode).
var bootstrapHeadingRe = regexp.MustCompile(`(?m)^###\s+`)

// bootstrapWhitespaceRe collapses runs of whitespace, matching
// `re.sub(r"\s+", " ", s)` in Python which treats `\s` as
// [ \t\n\r\f\v]. Go's default `\s` is missing \v so we spell out the
// class explicitly to keep oracle parity.
var bootstrapWhitespaceRe = regexp.MustCompile(`[\s\v]+`)

// ParseBootstrapBodies returns {id: body} where body spans from the
// step marker to the next "### " heading or EOF. Mirrors the Python
// `parse` function.
func ParseBootstrapBodies(md string) map[string]string {
	out := map[string]string{}
	matches := bootstrapStepRe.FindAllStringSubmatchIndex(md, -1)
	for _, m := range matches {
		// m = [matchStart, matchEnd, group1Start, group1End]
		start := m[1]
		id := md[m[2]:m[3]]
		nextHeading := bootstrapHeadingRe.FindStringIndex(md[start:])
		var end int
		if nextHeading != nil {
			end = start + nextHeading[0]
		} else {
			end = len(md)
		}
		out[id] = md[start:end]
	}
	return out
}

// ListBootstrapStepIDs returns step IDs in declaration order from md.
// Mirrors `bootstrap_hash.py list`.
func ListBootstrapStepIDs(md string) []string {
	var ids []string
	for _, m := range bootstrapStepRe.FindAllStringSubmatch(md, -1) {
		ids = append(ids, m[1])
	}
	return ids
}

// HashBootstrapStep returns the sha256 hash (prefixed "sha256:") of
// the whitespace-normalized body for id, or ErrStepNotFound if id is
// not present. Mirrors `bootstrap_hash.py hash`.
func HashBootstrapStep(md, id string) (string, error) {
	bodies := ParseBootstrapBodies(md)
	body, ok := bodies[id]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrStepNotFound, id)
	}
	norm := normalizeWhitespace(body)
	sum := sha256.Sum256([]byte(norm))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// BootstrapStepBody returns the raw body for id (no normalization).
func BootstrapStepBody(md, id string) (string, error) {
	bodies := ParseBootstrapBodies(md)
	body, ok := bodies[id]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrStepNotFound, id)
	}
	return body, nil
}

// HashBootstrapStepFile is a convenience wrapper: read the file at
// path then hash step id.
func HashBootstrapStepFile(path, id string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return HashBootstrapStep(string(data), id)
}

// BootstrapStepBodyFile is the file-aware counterpart of BootstrapStepBody.
func BootstrapStepBodyFile(path, id string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return BootstrapStepBody(string(data), id)
}

// ListBootstrapStepIDsFile reads md at path and lists step IDs.
func ListBootstrapStepIDsFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ListBootstrapStepIDs(string(data)), nil
}

// ErrStepNotFound is returned by HashBootstrapStep / BootstrapStepBody
// when the requested id has no marker in the document.
var ErrStepNotFound = errStepNotFound{}

type errStepNotFound struct{}

func (errStepNotFound) Error() string { return "step not found" }

func normalizeWhitespace(s string) string {
	collapsed := bootstrapWhitespaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(collapsed)
}
