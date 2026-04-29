package synth

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"awiki/internal/fsutil"
)

// Refine appends "- <note>" to the page's `## Feedback` section,
// creating the section before the GENERATED region if absent.
// Idempotent: if "- <note>" already exists anywhere in the page, the
// call is a no-op (stderr emits SYNTH-REFINE|skipped (duplicate)|...).
// Updates the frontmatter `last_updated:` field (only if already present).
func (r *Runner) Refine(slug, note string, stderr io.Writer) error {
	synthDir := r.synthDir()
	path := filepath.Join(synthDir, slug+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	body := string(data)
	bullet := "- " + note
	// Idempotence check: mirror bash `grep -F -q -- "- $note"`.
	// The bash grep matches if "- <note>" appears anywhere on a line.
	if strings.Contains(body, bullet+"\n") || strings.HasSuffix(body, bullet) {
		fmt.Fprintf(stderr, "SYNTH-REFINE|skipped (duplicate)|%s\n", slug)
		return nil
	}

	updated := insertFeedbackBullet(body, note)
	updated = setFrontmatterScalar(updated, "last_updated", r.today())
	if err := fsutil.AtomicWrite(path, []byte(updated)); err != nil {
		return err
	}
	fmt.Fprintf(stderr, "SYNTH-REFINE|appended|%s|%s\n", slug, note)
	return nil
}

func (r *Runner) today() string {
	if r != nil && r.Today != "" {
		return r.Today
	}
	return time.Now().Format("2006-01-02")
}

// insertFeedbackBullet returns body with "- <note>\n\n" inserted into
// or before the ## Feedback section. Mirrors bash cmd_refine awk logic.
func insertFeedbackBullet(body, note string) string {
	lines := strings.SplitAfter(body, "\n")
	bullet := "- " + note + "\n"
	blank := "\n"

	// Check if ## Feedback heading exists.
	feedbackIdx := -1
	for i, l := range lines {
		if strings.TrimRight(l, "\n\r") == "## Feedback" {
			feedbackIdx = i
			break
		}
	}

	if feedbackIdx == -1 {
		// No Feedback section: insert "## Feedback\n\n- <note>\n\n" before
		// the first <!-- BEGIN GENERATED ... --> line.
		// Mirrors bash awk: prints ## Feedback, "", "- note", "" then the marker.
		for i, l := range lines {
			if strings.HasPrefix(l, "<!-- BEGIN GENERATED") {
				prefix := strings.Join(lines[:i], "")
				suffix := strings.Join(lines[i:], "")
				return prefix + "## Feedback\n\n" + bullet + blank + suffix
			}
		}
		// No marker: append at EOF.
		if !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		return body + "\n## Feedback\n\n" + bullet
	}

	// ## Feedback exists: find next H2 OR BEGIN marker after feedbackIdx.
	// Mirrors bash awk: emits bullet + blank before that boundary line.
	for i := feedbackIdx + 1; i < len(lines); i++ {
		t := strings.TrimRight(lines[i], "\n\r")
		if strings.HasPrefix(t, "<!-- BEGIN GENERATED") {
			prefix := strings.Join(lines[:i], "")
			suffix := strings.Join(lines[i:], "")
			return prefix + bullet + blank + suffix
		}
		if strings.HasPrefix(t, "## ") && t != "## Feedback" {
			prefix := strings.Join(lines[:i], "")
			suffix := strings.Join(lines[i:], "")
			return prefix + bullet + blank + suffix
		}
	}

	// No subsequent boundary: append bullet at EOF.
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return body + bullet
}

// setFrontmatterScalar replaces a scalar key in the YAML frontmatter.
// Mirrors bash synth_fm_set_scalar: replaces the first matching "key: ..."
// line inside the frontmatter (between the first and second "---" lines).
// If the key does not exist, body is returned unchanged (matching bash behavior).
// If frontmatter markers are absent, body is returned unchanged.
func setFrontmatterScalar(body, key, value string) string {
	lines := strings.SplitAfter(body, "\n")
	dashCount := 0
	newLine := key + ": " + value + "\n"
	replaced := false
	for i, l := range lines {
		trimmed := strings.TrimRight(l, "\n\r")
		if trimmed == "---" {
			dashCount++
			if dashCount == 2 {
				// Past the frontmatter; stop scanning.
				break
			}
			continue
		}
		if dashCount == 1 && !replaced && strings.HasPrefix(l, key+":") {
			lines[i] = newLine
			replaced = true
		}
	}
	if !replaced {
		return body
	}
	return strings.Join(lines, "")
}
