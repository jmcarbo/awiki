package template

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// MigrationBlockedPatterns mirrors BLOCKED_PATTERNS in
// scripts/_template_helpers/migration.py. A migration's `touches:`
// glob (or a prompt's `scope_glob`) may not start with any of these.
var MigrationBlockedPatterns = []string{"secrets/", "themes/", ".awiki/", ".git/"}

// MigrationRequiredHeaderKeys mirrors REQUIRED_HEADER_KEYS in the
// Python oracle.
var MigrationRequiredHeaderKeys = []string{"migration", "requires", "touches", "idempotent"}

// migrationHeaderRe matches `# key: value` header lines in a migration
// shell script. The Python oracle uses `^#\s*([a-z_]+)\s*:\s*(.*)$`.
var migrationHeaderRe = regexp.MustCompile(`^#\s*([a-z_]+)\s*:\s*(.*)$`)

// ParseMigrationHeader reads the comment header of a migration script
// and returns the {key: value} map. Mirrors `parse_header`.
func ParseMigrationHeader(scriptPath string) (map[string]string, error) {
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		// Trim trailing CR; Python rstrip('\n') leaves CR alone, but
		// Windows-line-ended scripts shouldn't break parsing.
		line = strings.TrimRight(line, "\r")
		if !strings.HasPrefix(line, "#") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if strings.HasPrefix(line, "#!") {
				continue
			}
			break
		}
		m := migrationHeaderRe.FindStringSubmatch(line)
		if m != nil {
			out[m[1]] = strings.TrimSpace(m[2])
		}
	}
	return out, nil
}

// ValidateMigrationHeader returns the sorted list of missing required
// header keys, or an empty slice if all are present.
func ValidateMigrationHeader(h map[string]string) []string {
	var missing []string
	for _, k := range MigrationRequiredHeaderKeys {
		if _, ok := h[k]; !ok {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	return missing
}

// ValidateMigrationTouches returns the first blocked glob, or "" + nil
// if every glob is allowed. Mirrors `cmd_validate_touches`.
func ValidateMigrationTouches(h map[string]string) (blocked string) {
	touches := strings.Fields(h["touches"])
	for _, glob := range touches {
		for _, blockedPat := range MigrationBlockedPatterns {
			if strings.HasPrefix(glob, blockedPat) || glob == strings.TrimRight(blockedPat, "/") {
				return glob
			}
		}
	}
	return ""
}

// MigrationGlobMatch mirrors the Python helper `_glob_match`. It is
// fnmatch with one round of `**` expansion (prefix/suffix).
func MigrationGlobMatch(glob, path string) bool {
	if strings.Contains(glob, "**") {
		parts := strings.SplitN(glob, "**", 2)
		if len(parts) == 2 {
			prefix := strings.TrimRight(parts[0], "/")
			suffix := strings.TrimLeft(parts[1], "/")
			if prefix != "" && !(path == prefix || strings.HasPrefix(path, prefix+"/")) {
				return false
			}
			if suffix != "" && !fnmatch("*"+suffix, path) {
				return false
			}
			return true
		}
	}
	return fnmatch(glob, path)
}

// MigrationParseFrontmatter mirrors `_parse_yaml_frontmatter`: returns
// the key-value pairs in the leading "--- ... ---" block, with the same
// ad-hoc strip rules the Python helper applies (strip quotes, strip
// brackets — yes, even though that's a footgun for lists; we mirror the
// oracle exactly).
func MigrationParseFrontmatter(text string) map[string]string {
	out := map[string]string{}
	inFM := false
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "---" {
			if inFM {
				break
			}
			inFM = true
			continue
		}
		if inFM && strings.Contains(line, ":") {
			idx := strings.Index(line, ":")
			k := strings.TrimSpace(line[:idx])
			v := strings.TrimSpace(line[idx+1:])
			v = strings.Trim(v, `"`)
			v = strings.Trim(v, `'`)
			v = strings.Trim(v, `[`)
			v = strings.Trim(v, `]`)
			out[k] = v
		}
	}
	return out
}

// PromptStageScope captures the result of resolving a prompt's
// scope_glob against the user tree.
type PromptStageScope struct {
	ID    string
	Risk  string
	Scope string
	// Matches lists relative paths (sorted) under userTree that match
	// scope_glob, in the same order Python's rglob would yield.
	Matches []string
}

// ResolvePromptScope walks userTree and collects the relative paths
// whose path matches scope_glob. Mirrors the resolution loop in
// `cmd_stage_prompt`. Files are returned in lex-sorted relative-path
// order (more deterministic than Python's filesystem-order rglob, which
// is fine for callers that don't depend on the iteration order).
func ResolvePromptScope(userTree, scope string) ([]string, error) {
	var out []string
	if scope == "" {
		return nil, nil
	}
	err := filepath.WalkDir(userTree, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(userTree, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if MigrationGlobMatch(scope, rel) {
			out = append(out, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// PromptStageInput is the read-side bundle for StagePrompt.
type PromptStageInput struct {
	PromptPath  string
	UserTree    string
	PendingDir  string
	GitAttrFile string // empty = skip encryption filter
}

// PromptCheckAttr captures the slice of CheckAttrAll the migration
// staging step uses (filter only, not -a). The bash + Python oracle
// uses `git -c core.attributesfile=<f> check-attr filter <path>`. We
// reuse the SyncGit interface by exposing a helper that asks for the
// "filter:" line specifically.
type PromptCheckAttr interface {
	// CheckAttrFilter returns the single-line output of `git -c
	// core.attributesfile=<attrFile> check-attr filter <path>`. Used
	// by stage-prompt to detect encrypted matches.
	CheckAttrFilter(attrFile, path string) (out string, err error)
}

// StagePrompt mirrors `migration.py stage-prompt`. It validates the
// prompt's scope, optionally rejects encrypted-match paths, then writes
// the augmented body into pendingDir. Returns the staged file path on
// success.
func StagePrompt(in PromptStageInput, attr PromptCheckAttr) (stagedPath string, sid string, risk string, matches []string, err error) {
	data, err := os.ReadFile(in.PromptPath)
	if err != nil {
		return "", "", "", nil, err
	}
	text := string(data)
	fm := MigrationParseFrontmatter(text)

	stem := strings.TrimSuffix(filepath.Base(in.PromptPath), filepath.Ext(in.PromptPath))
	stem = strings.TrimSuffix(stem, ".prompt")
	sid = fm["id"]
	if sid == "" {
		sid = stem
	}
	scope := fm["scope_glob"]
	risk = fm["risk"]
	if risk == "" {
		risk = "medium"
	}

	if scope == "" {
		return "", "", "", nil, fmt.Errorf("prompt missing scope_glob: %s", in.PromptPath)
	}
	for _, blocked := range MigrationBlockedPatterns {
		if strings.HasPrefix(scope, blocked) || scope == strings.TrimRight(blocked, "/") {
			return "", "", "", nil, fmt.Errorf("scope_glob blocked: %s", scope)
		}
	}

	matches, err = ResolvePromptScope(in.UserTree, scope)
	if err != nil {
		return "", "", "", nil, err
	}

	if in.GitAttrFile != "" && attr != nil {
		if isFile(in.GitAttrFile) {
			for _, rel := range matches {
				out, err := attr.CheckAttrFilter(in.GitAttrFile, rel)
				if err != nil {
					return "", "", "", nil, err
				}
				if strings.Contains(out, "filter: git-crypt") {
					return "", "", "", nil, fmt.Errorf("scope_glob would match encrypted path: %s", rel)
				}
			}
		}
	}

	if err := os.MkdirAll(in.PendingDir, 0o755); err != nil {
		return "", "", "", nil, err
	}
	dest := filepath.Join(in.PendingDir, filepath.Base(in.PromptPath))
	body := text
	var b strings.Builder
	b.WriteString(body)
	b.WriteString("\n\n## Resolved scope\n")
	for _, m := range matches {
		b.WriteString("- ")
		b.WriteString(m)
		b.WriteString("\n")
	}
	b.WriteString("\n## Acceptance\nAfter completing, run: `just lint`.")
	b.WriteString("\n\n## Trust note\nConfirm intent with user before bulk edits.")
	b.WriteString("\n\n## Risk\n")
	b.WriteString(risk)
	b.WriteString("\n")
	if err := os.WriteFile(dest, []byte(b.String()), 0o644); err != nil {
		return "", "", "", nil, err
	}
	return dest, sid, risk, matches, nil
}

// MigrationRunner captures the operations the `run` verb needs that
// reach outside the package: the bash interpreter and git status /
// restore. Production wires `os/exec` adapters; tests inject fakes.
type MigrationRunner interface {
	// RunBash invokes `bash <script>` (optionally with --dry-run)
	// inside cwd, with env strictly equal to env. Returns the exit
	// code; a non-nil error indicates the binary is missing.
	RunBash(script, cwd string, args, env []string) (code int, err error)
	// GitStatusPorcelain returns `git -C <repo> status --porcelain`
	// stdout (one record per line).
	GitStatusPorcelain(repoRoot string) (out string, err error)
	// GitRestoreFromHead reverts tracked paths to HEAD content via
	// `git -C <repo> restore --source=HEAD -- <paths...>`.
	GitRestoreFromHead(repoRoot string, paths []string) error
}

// MigrationRunInput bundles the inputs RunMigration needs.
type MigrationRunInput struct {
	Script     string
	RepoRoot   string
	OldVersion string
	NewVersion string
	// PathEnv is the value to use for PATH; pass os.Getenv("PATH") in
	// production or a deterministic fixture in tests.
	PathEnv string
	// HomeEnv is the value to use for HOME (Python defaults to
	// Path.home()).
	HomeEnv string
	// LangEnv is the value of LANG (Python default "C.UTF-8").
	LangEnv string
	// LCAllEnv is the value of LC_ALL (Python default "C.UTF-8").
	LCAllEnv string
}

// MigrationRunResult captures the post-run audit findings.
type MigrationRunResult struct {
	// Code is the exit code of the migration's real run. 0 = success.
	// Non-zero codes are surfaced to the caller without audit.
	Code int
	// AwikiWrites lists paths under .awiki/ that the migration
	// touched (Python halts on these).
	AwikiWrites []string
	// OutOfScope lists paths the migration touched that don't match
	// the touches: globs (Python halts).
	OutOfScope []string
}

// RunMigration mirrors `migration.py run`. Returns:
//   - Code != 0 when the dry-run or real run failed; AwikiWrites /
//     OutOfScope are empty in that case.
//   - Code == 0 with AwikiWrites or OutOfScope populated when the
//     post-run audit caught a violation; the runner has already
//     reverted the offending paths (callers should print the same
//     halt message Python does).
//   - Code == 0 with both lists empty on a clean run.
func RunMigration(r MigrationRunner, in MigrationRunInput) (*MigrationRunResult, error) {
	if r == nil {
		return nil, errors.New("migration: runner is nil")
	}
	header, err := ParseMigrationHeader(in.Script)
	if err != nil {
		return nil, err
	}
	if blocked := ValidateMigrationTouches(header); blocked != "" {
		return nil, fmt.Errorf("touches blocked: %s", blocked)
	}
	touches := strings.Fields(header["touches"])

	preOut, err := r.GitStatusPorcelain(in.RepoRoot)
	if err != nil {
		return nil, err
	}
	preLines := map[string]bool{}
	for _, line := range strings.Split(preOut, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		preLines[line] = true
	}

	env := buildMigrationEnv(in)

	// Dry-run.
	code, err := r.RunBash(in.Script, in.RepoRoot, []string{"--dry-run"}, env)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return &MigrationRunResult{Code: code}, nil
	}

	// Real run.
	code, err = r.RunBash(in.Script, in.RepoRoot, nil, env)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return &MigrationRunResult{Code: code}, nil
	}

	// Post-run audit.
	postOut, err := r.GitStatusPorcelain(in.RepoRoot)
	if err != nil {
		return nil, err
	}

	scriptRel := ""
	if absScript, err := filepath.Abs(in.Script); err == nil {
		if absRoot, err := filepath.Abs(in.RepoRoot); err == nil {
			if rel, err := filepath.Rel(absRoot, absScript); err == nil && !strings.HasPrefix(rel, "..") {
				scriptRel = filepath.ToSlash(rel)
			}
		}
	}

	var (
		oosTracked   []string
		oosUntracked []string
		awTracked    []string
		awUntracked  []string
	)
	for _, line := range strings.Split(postOut, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if preLines[line] {
			continue
		}
		if len(line) < 3 {
			continue
		}
		xy := line[:2]
		rel := strings.TrimSpace(line[3:])
		if idx := strings.Index(rel, "->"); idx >= 0 {
			rel = strings.TrimSpace(rel[idx+2:])
		}
		if scriptRel != "" && rel == scriptRel {
			continue
		}
		isUntracked := xy == "??"
		if strings.HasPrefix(rel, ".awiki/") {
			if isUntracked {
				awUntracked = append(awUntracked, rel)
			} else {
				awTracked = append(awTracked, rel)
			}
			continue
		}
		matched := false
		for _, g := range touches {
			if MigrationGlobMatch(g, rel) {
				matched = true
				break
			}
		}
		if !matched {
			if isUntracked {
				oosUntracked = append(oosUntracked, rel)
			} else {
				oosTracked = append(oosTracked, rel)
			}
		}
	}

	res := &MigrationRunResult{}
	if len(awTracked) > 0 || len(awUntracked) > 0 {
		all := append([]string{}, awTracked...)
		all = append(all, awUntracked...)
		res.AwikiWrites = all
		// Revert.
		if len(awTracked) > 0 {
			_ = r.GitRestoreFromHead(in.RepoRoot, awTracked)
		}
		for _, u := range awUntracked {
			_ = os.Remove(filepath.Join(in.RepoRoot, u))
		}
		return res, nil
	}
	if len(oosTracked) > 0 || len(oosUntracked) > 0 {
		all := append([]string{}, oosTracked...)
		all = append(all, oosUntracked...)
		res.OutOfScope = all
		if len(oosTracked) > 0 {
			_ = r.GitRestoreFromHead(in.RepoRoot, oosTracked)
		}
		for _, u := range oosUntracked {
			_ = os.Remove(filepath.Join(in.RepoRoot, u))
		}
		return res, nil
	}
	return res, nil
}

func buildMigrationEnv(in MigrationRunInput) []string {
	pathEnv := in.PathEnv
	if pathEnv == "" {
		pathEnv = "/usr/bin:/bin"
	}
	homeEnv := in.HomeEnv
	if homeEnv == "" {
		// Python falls back to Path.home(); mirror that by leaving
		// HOME unset is unsafe (some scripts read it). Empty string
		// here means "let the runner pick".
		homeEnv = ""
	}
	langEnv := in.LangEnv
	if langEnv == "" {
		langEnv = "C.UTF-8"
	}
	lcEnv := in.LCAllEnv
	if lcEnv == "" {
		lcEnv = "C.UTF-8"
	}
	env := []string{
		"PATH=" + pathEnv,
		"HOME=" + homeEnv,
		"AWIKI_REPO_ROOT=" + in.RepoRoot,
		"AWIKI_TEMPLATE_OLD_VERSION=" + in.OldVersion,
		"AWIKI_TEMPLATE_NEW_VERSION=" + in.NewVersion,
		"LANG=" + langEnv,
		"LC_ALL=" + lcEnv,
	}
	// Mirror Python's deterministic ordering for callers that compare.
	sortedEnv := slices.Clone(env)
	sort.Strings(sortedEnv)
	return sortedEnv
}
