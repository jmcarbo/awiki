package initverb

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// stepWikiNameImpl runs Step 2: prompt for a kebab-case identifier +
// one-line purpose. Validates the name with a strict regex; the
// purpose is capped at 120 chars (BOOTSTRAP.md spec).
type stepWikiName struct{}

func (stepWikiName) ID() string          { return "wiki-name" }
func (stepWikiName) Description() string { return "wiki name + purpose" }
func (stepWikiName) Kind() Kind          { return KindInteractive }

// kebabRe accepts lowercase ASCII letters/digits separated by single
// hyphens. Empty + leading/trailing hyphens are rejected.
var kebabRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

const maxPurposeLen = 120

func (stepWikiName) Execute(ctx StepContext, ans *Answers) (Result, error) {
	if ans.WikiName == "" {
		if ctx.NonInteractive {
			ans.WikiName = sanitizeKebab(filepath.Base(ctx.RepoRoot))
		} else {
			if ctx.Prompt == nil {
				return Result{Status: StatusFailed}, fmt.Errorf("wiki-name: Prompt is nil")
			}
			val, err := ctx.Prompt("What's the wiki called? (kebab-case)", "")
			if err != nil {
				return Result{Status: StatusFailed}, err
			}
			ans.WikiName = strings.TrimSpace(val)
		}
	}
	if !kebabRe.MatchString(ans.WikiName) {
		if ctx.NonInteractive {
			ans.WikiName = sanitizeKebab(ans.WikiName)
		}
		if !kebabRe.MatchString(ans.WikiName) {
			return Result{Status: StatusFailed},
				fmt.Errorf("wiki-name: %q is not kebab-case (use lowercase letters, digits, single hyphens)", ans.WikiName)
		}
	}

	if ans.Purpose == "" && !ctx.NonInteractive {
		if ctx.Prompt != nil {
			val, err := ctx.Prompt("One-line purpose? (≤120 chars)", "")
			if err != nil {
				return Result{Status: StatusFailed}, err
			}
			ans.Purpose = strings.TrimSpace(val)
		}
	}
	if len(ans.Purpose) > maxPurposeLen {
		return Result{Status: StatusFailed},
			fmt.Errorf("wiki-name: purpose exceeds %d chars (got %d)", maxPurposeLen, len(ans.Purpose))
	}
	note := fmt.Sprintf("wiki_name=%s", ans.WikiName)
	if ans.Purpose != "" {
		note += fmt.Sprintf(" purpose=%q", truncate(ans.Purpose, 60))
	}
	return Result{Status: StatusApplied, Note: note}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// sanitizeKebab maps an arbitrary string into a kebabRe-valid form.
// Lowercases ASCII letters, replaces runs of non-[a-z0-9] with a
// single hyphen, trims leading/trailing hyphens. Returns "wiki" if
// the result would be empty.
func sanitizeKebab(s string) string {
	var b strings.Builder
	prevDash := true
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "wiki"
	}
	return out
}
