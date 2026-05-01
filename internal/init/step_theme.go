package initverb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// stepTheme runs Step 5: pick the Hugo theme. Default `hugo-book` is
// already a submodule in the template, so accepting the default is a
// no-op. A non-default answer triggers the submodule deinit/rm/add
// chain plus a hugo.toml `theme = ` rewrite.
//
// We require the theme URL (any non-default name needs a clone source)
// — the prompt asks for `<name>=<url>` style. In --non-interactive
// mode we always keep `hugo-book`.
type stepTheme struct{}

func (stepTheme) ID() string          { return "theme" }
func (stepTheme) Description() string { return "Hugo theme" }
func (stepTheme) Kind() Kind          { return KindHybrid }

const defaultTheme = "hugo-book"

func (stepTheme) Execute(ctx StepContext, ans *Answers) (Result, error) {
	if ans.Theme == "" {
		if ctx.NonInteractive {
			ans.Theme = defaultTheme
		} else {
			if ctx.Prompt == nil {
				return Result{Status: StatusFailed}, fmt.Errorf("theme: Prompt is nil")
			}
			val, err := ctx.Prompt("Hugo theme name (or '<name>=<url>' for a non-default theme)", defaultTheme)
			if err != nil {
				return Result{Status: StatusFailed}, err
			}
			ans.Theme = strings.TrimSpace(val)
		}
	}
	name, url := splitThemeSpec(ans.Theme)
	ans.Theme = name
	if name == defaultTheme {
		// Default — already a submodule + already wired in hugo.toml.
		return Result{Status: StatusApplied, Note: "theme=" + name + " (default kept)"}, nil
	}
	if url == "" {
		return Result{Status: StatusFailed},
			fmt.Errorf("theme: non-default theme %q requires a URL (use '<name>=<url>')", name)
	}
	if ctx.Git == nil {
		return Result{Status: StatusFailed}, fmt.Errorf("theme: Git adapter is nil")
	}
	deinitPath := filepath.Join("themes", defaultTheme)
	if _, err := ctx.Git.SubmoduleDeinit(ctx.Ctx, ctx.RepoRoot, deinitPath); err != nil {
		return Result{Status: StatusFailed}, fmt.Errorf("submodule deinit: %w", err)
	}
	if _, err := ctx.Git.SubmoduleRm(ctx.Ctx, ctx.RepoRoot, deinitPath); err != nil {
		return Result{Status: StatusFailed}, fmt.Errorf("submodule rm: %w", err)
	}
	// Remove the cached .git/modules entry the bash oracle wipes.
	_ = os.RemoveAll(filepath.Join(ctx.RepoRoot, ".git", "modules", "themes", defaultTheme))
	addPath := filepath.Join("themes", name)
	if _, err := ctx.Git.SubmoduleAdd(ctx.Ctx, ctx.RepoRoot, url, addPath); err != nil {
		return Result{Status: StatusFailed}, fmt.Errorf("submodule add: %w", err)
	}
	// Rewrite hugo.toml.
	hp := filepath.Join(ctx.RepoRoot, "hugo.toml")
	body, err := os.ReadFile(hp)
	if err != nil {
		return Result{Status: StatusFailed}, fmt.Errorf("read hugo.toml: %w", err)
	}
	updated := rewriteThemeLine(string(body), name)
	if err := os.WriteFile(hp, []byte(updated), 0o644); err != nil {
		return Result{Status: StatusFailed}, fmt.Errorf("write hugo.toml: %w", err)
	}
	return Result{Status: StatusApplied, Note: "theme=" + name}, nil
}

// splitThemeSpec accepts "name" or "name=url" and returns (name, url).
func splitThemeSpec(spec string) (string, string) {
	if i := strings.Index(spec, "="); i >= 0 {
		return strings.TrimSpace(spec[:i]), strings.TrimSpace(spec[i+1:])
	}
	return strings.TrimSpace(spec), ""
}

// rewriteThemeLine swaps `theme = "..."` to the new name, preserving
// indentation + line endings. If no theme= line exists, the original
// body is returned unchanged.
func rewriteThemeLine(body, name string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "theme") && strings.Contains(trim, "=") {
			lines[i] = fmt.Sprintf("theme = '%s'", name)
		}
	}
	return strings.Join(lines, "\n")
}
