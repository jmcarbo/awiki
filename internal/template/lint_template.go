package template

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// LintLevel is the severity tag used in LINT|<level>|... output lines.
type LintLevel string

const (
	LintLevelError   LintLevel = "error"
	LintLevelWarning LintLevel = "warning"
	LintLevelInfo    LintLevel = "info"
)

// LintMessage is a single LINT|... line, captured before formatting.
type LintMessage struct {
	Level LintLevel
	File  string
	Msg   string
}

// Format returns the canonical "LINT|level|file|msg" wire format.
func (m LintMessage) Format() string {
	return fmt.Sprintf("LINT|%s|%s|%s", m.Level, m.File, m.Msg)
}

// LintReport captures every emitted message plus the cumulative
// error/warning counts. Mirrors the way `lint_template.py` aggregates
// before computing the exit code.
type LintReport struct {
	Messages []LintMessage
	Errors   int
	Warnings int
}

func (r *LintReport) emit(level LintLevel, file, msg string) {
	r.Messages = append(r.Messages, LintMessage{Level: level, File: file, Msg: msg})
	switch level {
	case LintLevelError:
		r.Errors++
	case LintLevelWarning:
		r.Warnings++
	}
}

// ExitCode returns the Python oracle exit code: 0 clean, 1 warnings,
// 2 errors.
func (r *LintReport) ExitCode() int {
	if r.Errors > 0 {
		return 2
	}
	if r.Warnings > 0 {
		return 1
	}
	return 0
}

// LintTemplate runs every check `lint_template.py` emits in order. The
// availability check is delegated to the caller via lsRemote (matching
// the same wiring used by CheckAvailability); pass nil to skip it.
//
// noCheckEnv mirrors AWIKI_NO_TEMPLATE_CHECK; pass "" to enable.
// now is the wall clock used for pending-prompt staleness.
func LintTemplate(root string, now time.Time, noCheckEnv string, lsRemote LsRemoteFunc) (*LintReport, error) {
	r := &LintReport{}
	if err := lintManifest(r, root); err != nil {
		return nil, err
	}
	if err := lintMigrations(r, root); err != nil {
		return nil, err
	}
	if err := lintPendingPrompts(r, root, now); err != nil {
		return nil, err
	}
	if err := lintProvenanceDrift(r, root); err != nil {
		return nil, err
	}
	if err := lintOrphans(r, root); err != nil {
		return nil, err
	}
	if err := lintPendingPromptDrift(r, root); err != nil {
		return nil, err
	}
	if err := lintPromptBodyScope(r, root); err != nil {
		return nil, err
	}
	if err := lintAvailability(r, root, noCheckEnv, now, lsRemote); err != nil {
		return nil, err
	}
	return r, nil
}

func lintManifest(r *LintReport, root string) error {
	mp := filepath.Join(root, "template.manifest.toml")
	if !isFile(mp) {
		r.emit(LintLevelError, "template.manifest.toml", "missing")
		return nil
	}
	m, err := LoadManifest(mp)
	if err != nil {
		return err
	}
	// Walk strategies in precedence order so warnings are deterministic.
	seenStrats := map[string]bool{}
	walk := func(strat string) {
		globs := m.Strategies[strat]
		seen := map[string]bool{}
		for _, g := range globs {
			if seen[g] {
				r.emit(LintLevelWarning, "template.manifest.toml",
					fmt.Sprintf("duplicate glob in %s: %s", strat, g))
			}
			seen[g] = true
		}
	}
	for _, s := range strategyPrecedence {
		seenStrats[s] = true
		walk(s)
	}
	var extra []string
	for s := range m.Strategies {
		if !seenStrats[s] {
			extra = append(extra, s)
		}
	}
	sort.Strings(extra)
	for _, s := range extra {
		walk(s)
	}
	return nil
}

var migrationFilenameRe = regexp.MustCompile(`^(\d{4}-[a-z0-9-]+\.(?:sh|prompt\.md)|schema-\d+-to-\d+\.sh|README\.md|\.gitkeep)$`)

func lintMigrations(r *LintReport, root string) error {
	migDir := filepath.Join(root, "migrations")
	info, err := os.Stat(migDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(migDir)
	if err != nil {
		return err
	}
	// Sort for deterministic emit order. Python uses iterdir which is
	// filesystem-order on POSIX; bats fixtures don't depend on order
	// but our tests will, so sort explicitly.
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		full := filepath.Join(migDir, name)
		rel := relPath(root, full)
		if !migrationFilenameRe.MatchString(name) {
			r.emit(LintLevelError, rel, "migration filename does not match pattern")
			continue
		}
		if strings.HasSuffix(name, ".sh") && !strings.HasPrefix(name, "schema-") {
			data, err := os.ReadFile(full)
			if err != nil {
				return err
			}
			text := string(data)
			headers := map[string]bool{}
			for _, line := range strings.Split(text, "\n") {
				if !strings.HasPrefix(line, "#") {
					trimmed := strings.TrimSpace(line)
					if trimmed == "" || strings.HasPrefix(line, "#!") {
						continue
					}
					break
				}
				m := migrationHeaderRe.FindStringSubmatch(line)
				if m != nil {
					headers[m[1]] = true
				}
			}
			missing := []string{}
			for _, k := range MigrationRequiredHeaderKeys {
				if !headers[k] {
					missing = append(missing, k)
				}
			}
			sort.Strings(missing)
			if len(missing) > 0 {
				r.emit(LintLevelError, rel, fmt.Sprintf("missing header keys: %s", pythonStrList(missing)))
			}
			// Touches: blocked-pattern check.
			touchesRe := regexp.MustCompile(`^#\s*touches\s*:\s*(.*)$`)
			for _, line := range strings.Split(text, "\n") {
				m := touchesRe.FindStringSubmatch(line)
				if m == nil {
					continue
				}
				for _, g := range strings.Fields(m[1]) {
					for _, blocked := range MigrationBlockedPatterns {
						if strings.HasPrefix(g, blocked) || g == strings.TrimRight(blocked, "/") {
							r.emit(LintLevelError, rel,
								fmt.Sprintf("touches: includes blocked pattern %s", g))
						}
					}
				}
			}
		} else if strings.HasSuffix(name, ".prompt.md") {
			data, err := os.ReadFile(full)
			if err != nil {
				return err
			}
			fm := MigrationParseFrontmatter(string(data))
			required := []string{"id", "requires", "scope_glob", "risk"}
			missing := []string{}
			for _, k := range required {
				if _, ok := fm[k]; !ok {
					missing = append(missing, k)
				}
			}
			sort.Strings(missing)
			if len(missing) > 0 {
				r.emit(LintLevelError, rel, fmt.Sprintf("missing frontmatter keys: %s", pythonStrList(missing)))
			}
			riskValues := map[string]bool{"low": true, "medium": true, "high": true}
			risk := fm["risk"]
			if risk != "" && !riskValues[risk] {
				r.emit(LintLevelError, rel, fmt.Sprintf("risk must be low|medium|high: %s", risk))
			}
			scope := fm["scope_glob"]
			for _, blocked := range MigrationBlockedPatterns {
				if strings.HasPrefix(scope, blocked) || scope == strings.TrimRight(blocked, "/") {
					r.emit(LintLevelError, rel, fmt.Sprintf("scope_glob points at blocked pattern: %s", scope))
				}
			}
		}
	}
	return nil
}

func lintPendingPrompts(r *LintReport, root string, now time.Time) error {
	pp := filepath.Join(root, ".awiki", "pending-prompts")
	info, err := os.Stat(pp)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	cutoff := now.UTC().Add(-14 * 24 * time.Hour)
	entries, err := os.ReadDir(pp)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		full := filepath.Join(pp, name)
		fi, err := os.Stat(full)
		if err != nil {
			return err
		}
		mtime := fi.ModTime().UTC()
		if mtime.Before(cutoff) {
			r.emit(LintLevelWarning, relPath(root, full),
				fmt.Sprintf("pending prompt older than 14 days (mtime=%s)", isoFormat(mtime)))
		}
	}
	return nil
}

func lintProvenanceDrift(r *LintReport, root string) error {
	pj := filepath.Join(root, ".awiki", "template.json")
	if !isFile(pj) {
		return nil
	}
	prov, err := LoadProvenance(pj)
	if err != nil {
		return nil // mirror Python: bare json.loads, raise would be uncaught — but treat decode errors as no-op for now.
	}
	if prov.Repo != prov.OriginalRepo {
		r.emit(LintLevelWarning, ".awiki/template.json",
			fmt.Sprintf("repo differs from original_repo (repo=%s, original=%s)", prov.Repo, prov.OriginalRepo))
	}
	pinned := prov.Commit
	if pinned != "" {
		cacheDir := filepath.Join(root, ".awiki", "template-cache", pinned)
		if !isDir(cacheDir) {
			short := pinned
			if len(short) > 12 {
				short = short[:12]
			}
			r.emit(LintLevelWarning, ".awiki/template.json",
				fmt.Sprintf("pinned commit %s not in template-cache; will auto-recover on next update", short))
		}
	}
	return nil
}

func lintOrphans(r *LintReport, root string) error {
	state := filepath.Join(root, ".awiki", "template-cache", "_fetch", ".update-state.json")
	if isFile(state) {
		r.emit(LintLevelWarning, ".awiki/template-cache/_fetch/.update-state.json",
			"orphaned state file (in-progress update or crashed run); use --continue or --abort")
	}
	scratch := filepath.Join(root, ".awiki", "template-cache", "_fetch", "_scratch-merge")
	if isDir(scratch) {
		r.emit(LintLevelWarning, ".awiki/template-cache/_fetch/_scratch-merge",
			"orphaned plan-time scratch dir; safe to delete")
	}
	return nil
}

func lintPendingPromptDrift(r *LintReport, root string) error {
	pp := filepath.Join(root, ".awiki", "pending-prompts")
	pj := filepath.Join(root, ".awiki", "template.json")
	if !isDir(pp) || !isFile(pj) {
		return nil
	}
	prov, err := LoadProvenance(pj)
	if err != nil {
		return nil
	}
	applied := map[string]bool{}
	for _, m := range prov.AppliedMigrations {
		applied[m.ID] = true
	}
	entries, err := os.ReadDir(pp)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		stem := strings.TrimSuffix(name, ".md")
		stem = strings.TrimSuffix(stem, ".prompt")
		if !applied[stem] {
			continue
		}
		full := filepath.Join(pp, name)
		fi, err := os.Stat(full)
		if err != nil {
			return err
		}
		mtime := fi.ModTime().UTC()
		r.emit(LintLevelWarning, relPath(root, full),
			fmt.Sprintf("pending prompt for %s present but applied_migrations[] already records it (mtime=%s) — agent may have run without recording",
				stem, isoFormat(mtime)))
	}
	return nil
}

func lintPromptBodyScope(r *LintReport, root string) error {
	pp := filepath.Join(root, ".awiki", "pending-prompts")
	if !isDir(pp) {
		return nil
	}
	entries, err := os.ReadDir(pp)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		full := filepath.Join(pp, name)
		data, err := os.ReadFile(full)
		if err != nil {
			return err
		}
		text := string(data)
		fm := MigrationParseFrontmatter(text)
		scope := fm["scope_glob"]
		if scope == "" {
			continue
		}
		// Parse "## Resolved scope" body section.
		var resolved []string
		inResolved := false
		for _, line := range strings.Split(text, "\n") {
			stripped := strings.TrimSpace(line)
			if strings.HasPrefix(stripped, "## ") && strings.Contains(strings.ToLower(stripped), "resolved scope") {
				inResolved = true
				continue
			}
			if inResolved {
				if strings.HasPrefix(stripped, "## ") {
					inResolved = false
				} else if strings.HasPrefix(stripped, "- ") {
					resolved = append(resolved, strings.TrimSpace(stripped[2:]))
				}
			}
		}
		for _, rel := range resolved {
			for _, blocked := range MigrationBlockedPatterns {
				if strings.HasPrefix(rel, blocked) {
					r.emit(LintLevelError, relPath(root, full),
						fmt.Sprintf("resolved scope includes blocked path: %s", rel))
					break
				}
			}
		}
	}
	return nil
}

// lintAvailability runs the availability check and forwards the LINT|
// info|template|... line into the report (without affecting the exit
// code). Network failures are silent. Pass nil lsRemote to skip
// entirely.
func lintAvailability(r *LintReport, root string, noCheckEnv string, now time.Time, lsRemote LsRemoteFunc) error {
	if lsRemote == nil {
		return nil
	}
	res, err := CheckAvailability(root, noCheckEnv, now, lsRemote)
	if err != nil {
		return err
	}
	if res.Skipped || res.Message == "" {
		return nil
	}
	// availability already returns a fully-formatted "LINT|info|..." line.
	// We convert that back into a LintMessage so the report stays homogeneous.
	parts := strings.SplitN(res.Message, "|", 4)
	if len(parts) == 4 && parts[0] == "LINT" {
		r.Messages = append(r.Messages, LintMessage{
			Level: LintLevel(parts[1]),
			File:  parts[2],
			Msg:   parts[3],
		})
	}
	return nil
}

func relPath(root, full string) string {
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return full
	}
	return filepath.ToSlash(rel)
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

// pythonStrList renders a sorted list the same way Python's
// `str(sorted(list))` does: ['a', 'b'] (single quotes, comma-space).
func pythonStrList(items []string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, s := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteByte('\'')
		b.WriteString(s)
		b.WriteByte('\'')
	}
	b.WriteByte(']')
	return b.String()
}

// isoFormat mirrors Python `datetime.isoformat()` for an aware UTC
// instant: YYYY-MM-DDTHH:MM:SS[.ffffff]+00:00. Sub-second precision is
// only emitted if non-zero (matching CPython behaviour).
func isoFormat(t time.Time) string {
	t = t.UTC()
	micros := t.Nanosecond() / 1000
	if micros == 0 {
		return t.Format("2006-01-02T15:04:05+00:00")
	}
	return fmt.Sprintf("%s.%06d+00:00", t.Format("2006-01-02T15:04:05"), micros)
}
