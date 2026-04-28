package task

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func Run(opts Options) ([]Diagnostic, []FixRecord, error) {
	if opts.Fix {
		fixes, err := ApplyFixes(opts)
		if err != nil {
			return nil, fixes, err
		}
		scan, err := ScanContent(opts.ContentDir)
		if err != nil {
			return nil, fixes, err
		}
		return Lint(scan, opts), fixes, nil
	}
	scan, err := ScanContent(opts.ContentDir)
	if err != nil {
		return nil, nil, err
	}
	return Lint(scan, opts), nil, nil
}

func Lint(scan Scan, opts Options) []Diagnostic {
	var diagnostics []Diagnostic
	diagnostics = append(diagnostics, lintT1T3T4T14(scan)...)
	diagnostics = append(diagnostics, lintT2(scan)...)
	diagnostics = append(diagnostics, lintT5(scan)...)
	diagnostics = append(diagnostics, lintT6(scan)...)
	diagnostics = append(diagnostics, lintT7(scan)...)
	diagnostics = append(diagnostics, lintT8(scan)...)
	diagnostics = append(diagnostics, lintT9T10T11(scan, today(opts))...)
	diagnostics = append(diagnostics, lintT12(scan)...)
	diagnostics = append(diagnostics, lintT13(scan)...)
	diagnostics = append(diagnostics, lintT15(scan)...)
	return diagnostics
}

func lintT1T3T4T14(scan Scan) []Diagnostic {
	var out []Diagnostic
	for _, action := range scan.Actions {
		if action.BadStatus != "" {
			out = append(out, diag(Error, action.File, "T1", fmt.Sprintf("bad status marker on line %d (marker=[%s])", action.Line, action.BadStatus)))
		}
		if action.BadKey != "" {
			out = append(out, diag(Error, action.File, "T3", fmt.Sprintf("unrecognized tail key %s on line %d", action.BadKey, action.Line)))
		}
		if action.BadDate != "" {
			out = append(out, diag(Error, action.File, "T4", fmt.Sprintf("invalid %s on line %d", action.BadDate, action.Line)))
		}
		if action.HasBadID {
			msg := fmt.Sprintf("bad id shape ^%s on line %d", action.BadID, action.Line)
			if strings.Contains(action.BadID, "~") {
				msg += " (chain shape requires base~digits)"
			}
			out = append(out, diag(Error, action.File, "T14", msg))
		}
	}
	return out
}

func lintT2(scan Scan) []Diagnostic {
	seen := map[string][]Action{}
	for _, action := range scan.Actions {
		if action.ID == "" || action.HasBadID {
			continue
		}
		seen[action.ID] = append(seen[action.ID], action)
	}
	var out []Diagnostic
	for id, actions := range seen {
		if len(actions) <= 1 {
			continue
		}
		var where []string
		for _, action := range actions {
			where = append(where, fmt.Sprintf("%s:%d", action.File, action.Line))
		}
		out = append(out, diag(Error, "(multi)", "T2", fmt.Sprintf("duplicate id ^%s at %s", id, strings.Join(where, ", "))))
	}
	sortDiagnostics(out)
	return out
}

func lintT5(scan Scan) []Diagnostic {
	var out []Diagnostic
	for _, action := range scan.Actions {
		if action.Status == "?" && action.Wait == "" {
			out = append(out, diag(Error, action.File, "T5", fmt.Sprintf("waiting line missing wait:[[person]] (line %d)", action.Line)))
		}
	}
	return out
}

func lintT6(scan Scan) []Diagnostic {
	contextAliases := contextAliasSet(scan)
	var out []Diagnostic
	for _, action := range scan.Actions {
		if action.Context == "" {
			continue
		}
		if !contextAliases[action.Context] {
			out = append(out, diag(Error, action.File, "T6", fmt.Sprintf("unknown context %s (line %d)", action.Context, action.Line)))
		}
	}
	return out
}

func lintT7(scan Scan) []Diagnostic {
	var out []Diagnostic
	for _, edit := range scan.AgendaEdits {
		out = append(out, diag(Error, edit.File, "T7", fmt.Sprintf("hand-edit detected inside agenda:%s region (line %d)", edit.Region, edit.Line)))
	}
	return out
}

func lintT8(scan Scan) []Diagnostic {
	hasOpen := map[string]bool{}
	for _, action := range scan.Actions {
		if action.Status == " " || action.Status == "/" {
			hasOpen[action.File] = true
		}
	}
	var out []Diagnostic
	for _, page := range scan.Pages {
		if page.Type != "project" || page.Status != "active" || hasOpen[page.RelPath] {
			continue
		}
		base := filepath.Base(page.RelPath)
		if base == "_loose.md" || base == "_someday.md" || base == "_index.md" {
			continue
		}
		out = append(out, diag(Warn, page.RelPath, "T8", "type:project, status:active page has zero open [ ]/[/] actions"))
	}
	return out
}

func lintT9T10T11(scan Scan, today time.Time) []Diagnostic {
	var out []Diagnostic
	pageByRel := map[string]PageInfo{}
	for _, page := range scan.Pages {
		pageByRel[page.RelPath] = page
	}
	for _, action := range scan.Actions {
		if action.Status == "?" && action.Since != "" {
			if age, ok := daysBefore(action.Since, today); ok && age > 14 {
				out = append(out, diag(Warn, action.File, "T9", fmt.Sprintf("waiting-stale (since:%s, %dd ago) ^%s", action.Since, age, action.ID)))
			}
		}
		if (action.Status == " " || action.Status == "/") && action.Due != "" {
			if age, ok := daysBefore(action.Due, today); ok && age > 0 {
				out = append(out, diag(Warn, action.File, "T10", fmt.Sprintf("overdue (due:%s, %dd ago) ^%s", action.Due, age, action.ID)))
			}
		}
		if action.Status == ">" {
			page := pageByRel[action.File]
			if age, ok := daysBefore(page.LastUpdated, today); ok && age > 90 {
				out = append(out, diag(Warn, action.File, "T11", fmt.Sprintf("stale-someday (page last_updated:%s, %dd ago) ^%s", page.LastUpdated, age, action.ID)))
			}
		}
	}
	return out
}

func lintT12(scan Scan) []Diagnostic {
	counts := map[string]int{}
	file := map[string]string{}
	for _, action := range scan.Actions {
		if action.ID == "" || action.HasBadID {
			continue
		}
		base := action.ID
		if idx := strings.Index(base, "~"); idx >= 0 {
			base = base[:idx]
		}
		counts[base]++
		file[base] = action.File
	}
	var out []Diagnostic
	for base, count := range counts {
		switch {
		case count >= 200:
			out = append(out, diag(Error, file[base], "T12", fmt.Sprintf("recur-chain ^%s length=%d (>=200)", base, count)))
		case count >= 150:
			out = append(out, diag(Warn, file[base], "T12", fmt.Sprintf("recur-chain ^%s length=%d (>=150)", base, count)))
		}
	}
	sortDiagnostics(out)
	return out
}

func lintT13(scan Scan) []Diagnostic {
	aliasToSlug := map[string]string{}
	contextPages := map[string]PageInfo{}
	for _, page := range scan.Pages {
		if page.Type != "context" || page.Slug == "_index" {
			continue
		}
		contextPages[page.Slug] = page
		aliasToSlug["@"+page.Slug] = page.Slug
		for _, alias := range page.Aliases {
			aliasToSlug[alias] = page.Slug
		}
	}
	referenced := map[string]bool{}
	for _, action := range scan.Actions {
		if action.Context == "" {
			continue
		}
		if slug := aliasToSlug[action.Context]; slug != "" {
			referenced[slug] = true
		} else {
			referenced[strings.TrimPrefix(action.Context, "@")] = true
		}
	}
	var out []Diagnostic
	for slug, page := range contextPages {
		if !referenced[slug] {
			out = append(out, diag(Info, page.RelPath, "T13", fmt.Sprintf("context-unused (no actions reference @%s)", slug)))
		}
	}
	sortDiagnostics(out)
	return out
}

func lintT15(scan Scan) []Diagnostic {
	var out []Diagnostic
	for _, continuation := range scan.Continuations {
		out = append(out, diag(Warn, continuation.File, "T15", fmt.Sprintf("action continuation on line %d not supported (multi-line action)", continuation.Line)))
	}
	return out
}

func contextAliasSet(scan Scan) map[string]bool {
	out := map[string]bool{}
	for _, page := range scan.Pages {
		if page.Type != "context" {
			continue
		}
		out["@"+page.Slug] = true
		for _, alias := range page.Aliases {
			out[alias] = true
		}
	}
	return out
}

func today(opts Options) time.Time {
	value := opts.Today
	if value == "" {
		value = time.Now().UTC().Format("2006-01-02")
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Now().UTC()
	}
	return t
}

func daysBefore(value string, today time.Time) (int, bool) {
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return 0, false
	}
	return int(today.Sub(t).Hours() / 24), true
}

func diag(level Level, file string, code string, message string) Diagnostic {
	return Diagnostic{Level: level, File: file, Code: code, Message: message}
}

func sortDiagnostics(diagnostics []Diagnostic) {
	sort.Slice(diagnostics, func(i, j int) bool {
		if diagnostics[i].File == diagnostics[j].File {
			return diagnostics[i].Message < diagnostics[j].Message
		}
		return diagnostics[i].File < diagnostics[j].File
	})
}

func repoContentDir(repoRoot string) string {
	return filepath.Join(repoRoot, "content")
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}
