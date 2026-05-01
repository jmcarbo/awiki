package initverb

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// stepPatchIdentity runs Step 7: write the user-collected answers
// into the four identity files.
//
//   - WIKI.md Identity section: replace the three `<unset>` lines.
//   - hugo.toml: rewrite title (and baseURL when blank — placeholder
//     OK).
//   - content/_index.md: rewrite title + landing paragraph. With
//     --agent set, shells the agent CLI for a curated paragraph;
//     without, leaves a TODO placeholder the user can fill in later.
//   - content/log.md: frontmatter `draft:` already handled by Step 6;
//     we only verify the field exists.
//
// The mutations are line-oriented + idempotent: re-running with the
// same answers is a no-op.
type stepPatchIdentity struct{}

func (stepPatchIdentity) ID() string          { return "patch-identity" }
func (stepPatchIdentity) Description() string { return "patch identity files" }
func (stepPatchIdentity) Kind() Kind          { return KindHybrid }

func (stepPatchIdentity) Execute(ctx StepContext, ans *Answers) (Result, error) {
	if ans.WikiName == "" {
		return Result{Status: StatusFailed}, fmt.Errorf("patch-identity: wiki_name not set; re-run wiki-name step")
	}

	// 1. WIKI.md Identity section.
	if err := patchWikiMD(ctx.RepoRoot, ans); err != nil {
		return Result{Status: StatusFailed}, err
	}

	// 2. hugo.toml title.
	if err := patchHugoTOML(ctx.RepoRoot, ans); err != nil {
		return Result{Status: StatusFailed}, err
	}

	// 3. content/_index.md title + landing paragraph.
	landing := patchIdentityPlaceholder
	if ctx.Agent != nil && ctx.AgentCLI != "" {
		prompt := buildLandingPrompt(ans)
		out, err := ctx.Agent.Run(ctx.Ctx, ctx.AgentCLI, prompt)
		if err != nil {
			fmt.Fprintf(ctx.Stderr, "patch-identity: agent failed (%v); falling back to placeholder\n", err)
		} else if strings.TrimSpace(out) != "" {
			landing = strings.TrimSpace(out)
		}
	}
	if err := patchIndexMD(ctx.RepoRoot, ans, landing); err != nil {
		return Result{Status: StatusFailed}, err
	}

	// 4. content/log.md frontmatter — Step 6 already mutated draft:;
	//    no further work here (the BOOTSTRAP.md prose mentioned it
	//    only as a checklist item).

	note := fmt.Sprintf("patched WIKI.md, hugo.toml, content/_index.md (landing=%s)",
		labelLanding(landing))
	return Result{Status: StatusApplied, Note: note}, nil
}

const patchIdentityPlaceholder = "TODO: write landing paragraph here."

func labelLanding(s string) string {
	if s == patchIdentityPlaceholder {
		return "TODO"
	}
	return "agent"
}

// buildLandingPrompt mirrors the prompt the BOOTSTRAP.md spec
// describes for the agent CLI. Captured here so the step + tests
// agree on the exact wording.
func buildLandingPrompt(ans *Answers) string {
	purpose := ans.Purpose
	if purpose == "" {
		purpose = "(unspecified)"
	}
	return fmt.Sprintf(
		"Write a one-paragraph landing for a wiki named '%s' on domain '%s'. Purpose: '%s'. Plain prose, no markdown headings.",
		ans.WikiName, ans.Domain, purpose)
}

// --- WIKI.md ---

var (
	wikiNameLineRe = regexp.MustCompile(`(?m)^(\s*-\s*\*\*wiki_name:\*\*\s*).*$`)
	wikiDomainLineRe = regexp.MustCompile(`(?m)^(\s*-\s*\*\*domain:\*\*\s*).*$`)
	wikiPurposeLineRe = regexp.MustCompile(`(?m)^(\s*-\s*\*\*purpose:\*\*\s*).*$`)
)

func patchWikiMD(repoRoot string, ans *Answers) error {
	p := filepath.Join(repoRoot, "WIKI.md")
	body, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("read WIKI.md: %w", err)
	}
	out := string(body)
	out = wikiNameLineRe.ReplaceAllString(out, "${1}`"+ans.WikiName+"`")
	out = wikiDomainLineRe.ReplaceAllString(out, "${1}`"+ans.Domain+"`")
	purpose := ans.Purpose
	if purpose == "" {
		purpose = "<unset>"
	}
	out = wikiPurposeLineRe.ReplaceAllString(out, "${1}`"+purpose+"`")
	if out == string(body) {
		return nil
	}
	return os.WriteFile(p, []byte(out), 0o644)
}

// --- hugo.toml ---

var (
	hugoTitleLineRe = regexp.MustCompile(`(?m)^(\s*title\s*=\s*).*$`)
)

func patchHugoTOML(repoRoot string, ans *Answers) error {
	p := filepath.Join(repoRoot, "hugo.toml")
	body, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("read hugo.toml: %w", err)
	}
	out := hugoTitleLineRe.ReplaceAllString(string(body), "${1}'"+ans.WikiName+"'")
	if out == string(body) {
		return nil
	}
	return os.WriteFile(p, []byte(out), 0o644)
}

// --- content/_index.md ---

var (
	indexTitleFMRe = regexp.MustCompile(`(?m)^(\s*title\s*:\s*).*$`)
)

func patchIndexMD(repoRoot string, ans *Answers, landing string) error {
	p := filepath.Join(repoRoot, "content", "_index.md")
	body, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("read content/_index.md: %w", err)
	}
	out := string(body)
	// Replace the first title: line of the frontmatter.
	out = replaceFirst(out, indexTitleFMRe, "${1}\""+ans.WikiName+"\"")
	// Append (or replace) the landing paragraph just under the
	// frontmatter close. We use a sentinel comment so re-runs replace
	// the same block in place.
	const startMarker = "<!-- BOOTSTRAP-LANDING-START -->"
	const endMarker = "<!-- BOOTSTRAP-LANDING-END -->"
	block := startMarker + "\n" + landing + "\n" + endMarker
	if i := strings.Index(out, startMarker); i >= 0 {
		j := strings.Index(out[i:], endMarker)
		if j >= 0 {
			j += i + len(endMarker)
			out = out[:i] + block + out[j:]
		}
	} else {
		// Insert just after the closing frontmatter `---`.
		closing := strings.Index(out, "\n---\n")
		if closing >= 0 {
			insertAt := closing + len("\n---\n")
			out = out[:insertAt] + "\n" + block + "\n" + out[insertAt:]
		} else {
			out = block + "\n" + out
		}
	}
	if out == string(body) {
		return nil
	}
	return os.WriteFile(p, []byte(out), 0o644)
}

// replaceFirst applies re's first match in s with repl, leaving the
// rest of s untouched. regexp.ReplaceAll is global; we want first-only.
func replaceFirst(s string, re *regexp.Regexp, repl string) string {
	loc := re.FindStringIndex(s)
	if loc == nil {
		return s
	}
	matched := s[loc[0]:loc[1]]
	return s[:loc[0]] + re.ReplaceAllString(matched, repl) + s[loc[1]:]
}
