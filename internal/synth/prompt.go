package synth

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// EmitPrompt writes the rendered plugin prompt bundle to out.
// scopeDesc replaces {{scope_description}}. The {{#pages}}...
// {{/pages}} block is replaced with one row per slug. The
// {{#feedback}}...{{/feedback}} block is dropped if feedback is
// empty; otherwise the {{feedback}} placeholder inside is replaced
// with the feedback string and the {{#feedback}}/{{/feedback}}
// delimiter lines are stripped.
//
// Matches scripts/synth.sh:synth_emit_prompt.
func (r *Runner) EmitPrompt(out io.Writer, plugin Plugin, slugs []string, scopeDesc, feedback string) error {
	body := plugin.Body
	body = strings.ReplaceAll(body, "{{scope_description}}", scopeDesc)
	body = expandPagesBlock(body, r.pagesBlock(slugs))
	body = expandFeedbackBlock(body, feedback)
	_, err := io.WriteString(out, body)
	return err
}

// pagesBlock returns the row-per-slug content used to fill the
// {{#pages}}...{{/pages}} block. Each row:
//
//	- [[<slug>]] (<type>) — <lead-up-to-200-chars>
func (r *Runner) pagesBlock(slugs []string) string {
	var b strings.Builder
	for _, slug := range slugs {
		path := r.slugToPath(slug)
		if path == "" {
			continue
		}
		fm, body, ok := readFrontmatterAndBody(path)
		if !ok {
			continue
		}
		title := strings.Trim(fm["title"], `"`)
		_ = title // bash sets but row format does not include title
		typ := fm["type"]
		lead := firstNonBlankLine(body)
		if len(lead) > 200 {
			lead = lead[:200]
		}
		fmt.Fprintf(&b, "- [[%s]] (%s) — %s\n", slug, typ, lead)
	}
	return strings.TrimRight(b.String(), "\n")
}

func expandPagesBlock(body, rows string) string {
	const open = "{{#pages}}"
	const close = "{{/pages}}"
	startIdx := strings.Index(body, open)
	if startIdx < 0 {
		return body
	}
	endIdx := strings.Index(body[startIdx:], close)
	if endIdx < 0 {
		return body
	}
	endIdx += startIdx + len(close)
	// Bash awk processes per-line: prints everything before {{#pages}},
	// skips the delimiter lines, and emits the rows followed by a newline
	// then continues with the line after {{/pages}}.
	// prefix includes the newline before {{#pages}}; suffix strips the
	// newline immediately after {{/pages}} (which is the line separator).
	prefix := body[:startIdx]
	suffix := stripLeadingNewline(body[endIdx:])
	return prefix + rows + "\n" + suffix
}

func expandFeedbackBlock(body, feedback string) string {
	const open = "{{#feedback}}"
	const close = "{{/feedback}}"
	startIdx := strings.Index(body, open)
	if startIdx < 0 {
		return body
	}
	endIdx := strings.Index(body[startIdx:], close)
	if endIdx < 0 {
		return body
	}
	endIdx += startIdx + len(close)
	if feedback == "" {
		// drop the entire wrapped region including the surrounding newlines.
		// body[:startIdx] includes the trailing newline before {{#feedback}};
		// trimTrailingNewline removes it so the drop is clean.
		prefix := trimTrailingNewline(body[:startIdx])
		suffix := stripLeadingNewline(body[endIdx:])
		return prefix + suffix
	}
	// substitute {{feedback}} inside the inner region; strip the
	// delimiter lines.
	inner := body[startIdx+len(open) : endIdx-len(close)]
	inner = strings.ReplaceAll(inner, "{{feedback}}", feedback)
	inner = strings.TrimPrefix(inner, "\n")
	inner = strings.TrimSuffix(inner, "\n")
	prefix := trimTrailingNewline(body[:startIdx])
	suffix := stripLeadingNewline(body[endIdx:])
	return prefix + "\n" + inner + "\n" + suffix
}

// renderFeedbackBlock builds the fenced text block that
// {{#feedback}}...{{/feedback}} consumes when the regen path has
// non-empty feedback. Matches scripts/synth.sh:synth_render_feedback_block.
func renderFeedbackBlock(pageBytes []byte) string {
	lines := strings.Split(string(pageBytes), "\n")
	inBlock := false
	var bullets []string
	for _, l := range lines {
		t := strings.TrimRight(l, "\r")
		if t == "## Feedback" {
			inBlock = true
			continue
		}
		if inBlock {
			if strings.HasPrefix(t, "## ") {
				inBlock = false
				continue
			}
			if strings.HasPrefix(t, "<!-- BEGIN GENERATED") {
				inBlock = false
				continue
			}
			if strings.HasPrefix(t, "- ") {
				bullets = append(bullets, strings.TrimPrefix(t, "- "))
			}
		}
	}
	if len(bullets) == 0 {
		return ""
	}
	return "```text\n" + strings.Join(bullets, "\n") + "\n```"
}

func trimTrailingNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s[:len(s)-1]
	}
	return s
}

func stripLeadingNewline(s string) string {
	if strings.HasPrefix(s, "\n") {
		return s[1:]
	}
	return s
}

func firstNonBlankLine(body string) string {
	for _, l := range strings.Split(body, "\n") {
		if strings.TrimSpace(l) != "" {
			return l
		}
	}
	return ""
}

func readFrontmatterAndBody(path string) (map[string]string, string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", false
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		return nil, text, false
	}
	rest := text[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return nil, text, false
	}
	fm := map[string]string{}
	for _, l := range strings.Split(rest[:end], "\n") {
		if i := strings.Index(l, ":"); i > 0 {
			fm[strings.TrimSpace(l[:i])] = strings.TrimSpace(l[i+1:])
		}
	}
	body := rest[end+len("\n---\n"):]
	return fm, body, true
}
