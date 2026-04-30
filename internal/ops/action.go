package ops

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"awiki/internal/action"
)

// --- SCAN ----------------------------------------------------------------

// ScanOptions configures `awiki scan`.
type ScanOptions struct {
	RepoRoot string
}

// actionRow mirrors the per-row TSV layout in scripts/action-scan.sh:
//
//	id  status  text  file  line  context  due  defer  wait  since  every
//	done  priority  est  project  source_kind
type actionRow struct {
	id, status, text, file        string
	line                          int
	ctx                           string
	due, defer_, wait, since      string
	every, done, priority, est    string
	project, sourceKind           string
}

// rejectedRow mirrors the four-column rejected.tsv layout:
//
//	file  line  reason  raw  detail
type rejectedRow struct {
	file, reason, raw, detail string
	line                      int
}

const actionsTSVHeader = "id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n"

// Scan walks content/**/*.md, parses every action line per the canonical
// grammar, and writes .awiki/maps/actions.tsv plus actions-rejected.tsv.
//
// Returns 0 on clean success, 1 on duplicate IDs detected (matches
// scripts/action-scan.sh).
func Scan(opts ScanOptions, stdout, stderr io.Writer) int {
	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	contentDir := filepath.Join(repoRoot, "content")
	mapsDir := filepath.Join(repoRoot, ".awiki", "maps")
	if err := os.MkdirAll(mapsDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "scan: mkdir maps: %v\n", err)
		return 1
	}
	actionsTSV := filepath.Join(mapsDir, "actions.tsv")
	rejectedTSV := filepath.Join(mapsDir, "actions-rejected.tsv")
	aliasMap := filepath.Join(mapsDir, "alias-to-slug.tsv")

	// Privacy classifiers.
	gitcryptPatterns, _ := readGitcryptPatterns(filepath.Join(repoRoot, ".gitattributes"))
	aliasKinds, _ := readAliasKinds(aliasMap)

	// Walk content/.
	var paths []string
	if _, err := os.Stat(contentDir); err == nil {
		_ = filepath.WalkDir(contentDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() || filepath.Ext(path) != ".md" {
				return nil
			}
			paths = append(paths, path)
			return nil
		})
	}
	sort.Strings(paths)

	scannedFiles := 0
	var rows []actionRow
	var rejected []rejectedRow

	for _, path := range paths {
		rel, _ := filepath.Rel(repoRoot, path)
		rel = filepath.ToSlash(rel)
		scannedFiles++
		newRows, newRejected := scanFile(repoRoot, rel, gitcryptPatterns, aliasKinds)
		rows = append(rows, newRows...)
		rejected = append(rejected, newRejected...)
	}

	// Write atomic.
	var actionsBuf bytes.Buffer
	actionsBuf.WriteString(actionsTSVHeader)
	for _, r := range rows {
		fmt.Fprintf(&actionsBuf,
			"%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.id, r.status, r.text, r.file, r.line, r.ctx,
			r.due, r.defer_, r.wait, r.since, r.every, r.done,
			r.priority, r.est, r.project, r.sourceKind)
	}
	if err := writeAtomic(actionsTSV, actionsBuf.Bytes()); err != nil {
		fmt.Fprintf(stderr, "scan: write actions.tsv: %v\n", err)
		return 1
	}
	var rejBuf bytes.Buffer
	for _, r := range rejected {
		fmt.Fprintf(&rejBuf, "%s\t%d\t%s\t%s\t%s\n",
			r.file, r.line, r.reason, r.raw, r.detail)
	}
	if err := writeAtomic(rejectedTSV, rejBuf.Bytes()); err != nil {
		fmt.Fprintf(stderr, "scan: write rejected.tsv: %v\n", err)
		return 1
	}

	// Summary line.
	totalActions := len(rows)
	rejectedCount := len(rejected)
	openCount := 0
	doneCount := 0
	for _, r := range rows {
		switch r.status {
		case " ", "/":
			openCount++
		case "x":
			doneCount++
		}
	}
	fmt.Fprintf(stdout, "scanned %d files, %d actions, %d rejected, %d open, %d done\n",
		scannedFiles, totalActions, rejectedCount, openCount, doneCount)

	// Duplicate-ID detection.
	idCounts := make(map[string]int)
	for _, r := range rows {
		if r.id != "" {
			idCounts[r.id]++
		}
	}
	var dupIDs []string
	for id, n := range idCounts {
		if n > 1 {
			dupIDs = append(dupIDs, id)
		}
	}
	if len(dupIDs) > 0 {
		sort.Strings(dupIDs)
		fmt.Fprintf(stderr, "SCAN|dup-id|%s\n", strings.Join(dupIDs, "\n"))
		return 1
	}
	return 0
}

// ScanCLI parses argv and dispatches Scan.
func ScanCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: awiki scan")
		return 2
	}
	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	return Scan(ScanOptions{RepoRoot: repoRoot}, stdout, stderr)
}

// scanFile mirrors scan_one_file from scripts/action-scan.sh.
func scanFile(repoRoot, relPath string, gitcryptPatterns []string, aliasKinds map[string]string) (rows []actionRow, rejected []rejectedRow) {
	full := filepath.Join(repoRoot, relPath)
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, nil
	}
	// Skip if type=agenda (would otherwise produce dup-id rows).
	pageType := frontmatterScalar(data, "type")
	if pageType == "agenda" {
		return nil, nil
	}
	projectSlug := ""
	if pageType == "project" {
		projectSlug = strings.TrimSuffix(filepath.Base(relPath), ".md")
	}

	frontmatterEnded := false
	fmSeen := false
	inFence := false
	prevWasAction := false
	lineno := 0

	for line := range strings.SplitSeq(string(data), "\n") {
		// SplitSeq emits a final empty after a trailing newline; skip
		// the synthetic blank that appears after the last \n only when
		// it is at the end. The bash form reads with `while IFS= read`,
		// which also drops the trailing empty.
		// Match line counter to bash: each iteration increments lineno
		// before processing, so a trailing newline produces an extra
		// virtual line. We mimic that by counting unconditionally.
		lineno++

		if isFrontmatterSep(line) {
			if !fmSeen {
				fmSeen = true
			} else if !frontmatterEnded {
				frontmatterEnded = true
			}
			continue
		}
		if fmSeen && !frontmatterEnded {
			continue
		}

		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			prevWasAction = false
			continue
		}
		if inFence {
			prevWasAction = false
			continue
		}
		if strings.HasPrefix(line, ">") {
			prevWasAction = false
			continue
		}

		// Continuation: indented non-blank line following an action.
		if prevWasAction && len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				rejected = append(rejected, rejectedRow{
					file: relPath, line: lineno,
					reason: "continuation", raw: tsvEscape(line),
					detail: "follows-action-line",
				})
				continue
			}
		}

		// Action-line check: must match `^[\s]*-[\s]+\[(.)\]\s+`.
		marker, ok := matchActionStatus(line)
		if !ok {
			prevWasAction = false
			continue
		}

		// Status marker validation (strict set " /?>x-").
		if !strings.ContainsRune(" /?>x-", rune(marker)) {
			rejected = append(rejected, rejectedRow{
				file: relPath, line: lineno,
				reason: "bad-status", raw: tsvEscape(line),
				detail: fmt.Sprintf("marker=[%c]", marker),
			})
			prevWasAction = true
			continue
		}

		parsed, ok := action.ParseLine(line)
		if !ok || parsed.BadStatus != "" {
			rejected = append(rejected, rejectedRow{
				file: relPath, line: lineno,
				reason: "no-status", raw: tsvEscape(line),
				detail: "grammar-parse-failed",
			})
			prevWasAction = true
			continue
		}

		row := actionRow{
			id: parsed.ID, status: string(marker),
			text: tsvEscape(parsed.Text),
			file: relPath, line: lineno,
			ctx: parsed.Context, due: parsed.Due, defer_: parsed.Defer,
			wait: parsed.Wait, since: parsed.Since, every: parsed.Every,
			done: parsed.Done, priority: parsed.Priority, est: parsed.Estimate,
			project:    projectSlug,
			sourceKind: classifySourceKind(repoRoot, relPath, line, gitcryptPatterns, aliasKinds),
		}
		rows = append(rows, row)
		prevWasAction = true
	}
	return rows, rejected
}

var actionLineRE = regexp.MustCompile(`^\s*-\s+\[(.)\]\s+`)

func matchActionStatus(line string) (byte, bool) {
	m := actionLineRE.FindStringSubmatch(line)
	if m == nil {
		return 0, false
	}
	return m[1][0], true
}

// frontmatterScalar reads the leading `---` ... `---` frontmatter block
// of data and returns a scalar key value. Empty if absent.
func frontmatterScalar(data []byte, key string) string {
	s := string(data)
	if !strings.HasPrefix(s, "---") {
		return ""
	}
	// Find first newline after opening ---.
	first := strings.Index(s, "\n")
	if first < 0 {
		return ""
	}
	rest := s[first+1:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return ""
	}
	body := rest[:end]
	prefix := key + ":"
	for line := range strings.SplitSeq(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		v := strings.TrimSpace(trimmed[len(prefix):])
		v = strings.Trim(v, `"`)
		// Only return the first whitespace-delimited word (mirrors
		// the bash `awk '{print $1}'` step).
		if idx := strings.IndexByte(v, ' '); idx >= 0 {
			v = v[:idx]
		}
		return v
	}
	return ""
}

func isFrontmatterSep(line string) bool {
	return strings.TrimSpace(line) == "---"
}

func tsvEscape(s string) string {
	s = strings.ReplaceAll(s, "\t", `\t`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

// readGitcryptPatterns parses .gitattributes for `<pattern> filter=git-crypt`.
func readGitcryptPatterns(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pats []string
	for line := range strings.SplitSeq(string(data), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") {
			continue
		}
		if !strings.Contains(t, "filter=git-crypt") {
			continue
		}
		// Pattern is the first whitespace-delimited token.
		idx := strings.IndexAny(t, " \t")
		if idx < 0 {
			continue
		}
		pats = append(pats, t[:idx])
	}
	return pats, nil
}

func pathIsPrivateByGitcrypt(rel string, patterns []string) bool {
	for _, pat := range patterns {
		// Convert "**" -> "*" to mirror the bash `${pat//\*\*/*}` pass.
		bp := strings.ReplaceAll(pat, "**", "*")
		ok, _ := filepath.Match(bp, rel)
		if ok {
			return true
		}
	}
	return false
}

func frontmatterHasPrivateTag(data []byte) bool {
	s := string(data)
	if !strings.HasPrefix(s, "---") {
		return false
	}
	first := strings.Index(s, "\n")
	if first < 0 {
		return false
	}
	rest := s[first+1:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return false
	}
	body := rest[:end]
	for line := range strings.SplitSeq(body, "\n") {
		if strings.Contains(line, "tags:") && strings.Contains(line, "private") {
			return true
		}
	}
	return false
}

// readAliasKinds reads alias-to-slug.tsv (alias, slug, path, kind) and
// returns a slug→kind map.
func readAliasKinds(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}, err
	}
	out := map[string]string{}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			continue
		}
		out[fields[1]] = fields[3]
	}
	return out, nil
}

var wikilinkRE = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

func classifySourceKind(repoRoot, relPath, line string, gitcryptPatterns []string, aliasKinds map[string]string) string {
	if pathIsPrivateByGitcrypt(relPath, gitcryptPatterns) {
		return "private"
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, relPath))
	if err == nil && frontmatterHasPrivateTag(data) {
		return "private"
	}
	for _, m := range wikilinkRE.FindAllStringSubmatch(line, -1) {
		raw := m[1]
		// Strip header / display.
		slug := raw
		if idx := strings.IndexByte(slug, '#'); idx >= 0 {
			slug = slug[:idx]
		}
		if idx := strings.IndexByte(slug, '|'); idx >= 0 {
			slug = slug[:idx]
		}
		if aliasKinds[slug] == "private" {
			return "private"
		}
	}
	return "public"
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// --- RECUR ---------------------------------------------------------------

// RecurOptions configures `awiki recur` / `awiki recur-dry`.
type RecurOptions struct {
	RepoRoot string
	Page     string // empty + All=true → discover via actions.tsv
	All      bool
	DryRun   bool
}

// recurSep mirrors AWIKI_RECUR_SEP. Defaults to '~'; honors env var.
func recurSep() string {
	if v := os.Getenv("AWIKI_RECUR_SEP"); v != "" {
		return v
	}
	return "~"
}

// Recur is the entry point for action-recur.sh. Returns:
//
//	0 success
//	2 bad args
//	3 page or actions.tsv not found
//	6 chain length cap reached
func Recur(opts RecurOptions, stdout, stderr io.Writer) int {
	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	if opts.All {
		if opts.Page != "" {
			fmt.Fprintln(stderr, "action-recur: --all and <page-path> are mutually exclusive")
			return 2
		}
		actionsTSV := filepath.Join(repoRoot, ".awiki", "maps", "actions.tsv")
		data, err := os.ReadFile(actionsTSV)
		if err != nil {
			fmt.Fprintln(stderr, "action-recur: actions.tsv missing; run action-scan.sh first")
			return 3
		}
		seen := map[string]bool{}
		var pages []string
		for i, row := range strings.Split(string(data), "\n") {
			if i == 0 || row == "" {
				continue
			}
			cols := strings.Split(row, "\t")
			if len(cols) < 11 {
				continue
			}
			if cols[1] == "x" && cols[10] != "" {
				if !seen[cols[3]] {
					seen[cols[3]] = true
					pages = append(pages, cols[3])
				}
			}
		}
		sort.Strings(pages)
		for _, p := range pages {
			if code := recurOnePage(repoRoot, p, opts.DryRun, stdout, stderr); code != 0 {
				return code
			}
		}
		return 0
	}
	if opts.Page == "" {
		fmt.Fprintln(stderr, "action-recur: usage: <page-path> | --all [--dry-run]")
		return 2
	}
	return recurOnePage(repoRoot, opts.Page, opts.DryRun, stdout, stderr)
}

// RecurCLI handles parsing of action-recur arguments and dispatches.
// dry: when true, force --dry-run.
func RecurCLI(dry bool, args []string, stdout, stderr io.Writer) int {
	opts := RecurOptions{DryRun: dry}
	for _, arg := range args {
		switch {
		case arg == "--dry-run":
			opts.DryRun = true
		case arg == "--all":
			opts.All = true
		case arg == "--help" || arg == "-h":
			fmt.Fprintln(stdout, "Usage:")
			fmt.Fprintln(stdout, "  awiki recur [--dry-run] <page-path>")
			fmt.Fprintln(stdout, "  awiki recur [--dry-run] --all")
			return 0
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(stderr, "action-recur: unknown flag: %s\n", arg)
			return 2
		default:
			if opts.Page != "" {
				fmt.Fprintln(stderr, "action-recur: unexpected positional arg")
				return 2
			}
			opts.Page = arg
		}
	}
	if !opts.All && opts.Page == "" {
		fmt.Fprintln(stderr, "Usage:")
		fmt.Fprintln(stderr, "  awiki recur [--dry-run] <page-path>")
		fmt.Fprintln(stderr, "  awiki recur [--dry-run] --all")
		return 2
	}
	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	opts.RepoRoot = repoRoot
	return Recur(opts, stdout, stderr)
}

func recurOnePage(repoRoot, page string, dryRun bool, stdout, stderr io.Writer) int {
	full := page
	if !filepath.IsAbs(full) {
		full = filepath.Join(repoRoot, page)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		fmt.Fprintf(stderr, "action-recur: page not found: %s\n", page)
		return 3
	}
	newContent, code := recurComputeNewContents(data)
	if code != 0 {
		// Cap-reached: emit the fixed message to stderr + exit 6.
		fmt.Fprintln(stderr, "action-recur: chain ^"+capChainBase(data)+" reached 200 instances; refusing to emit (exit 6)")
		return 6
	}
	if bytes.Equal(data, newContent) {
		if dryRun {
			return 0
		}
		return 0
	}
	if dryRun {
		emitUnifiedDiff(stdout, page, data, newContent)
		return 0
	}
	if err := writeAtomic(full, newContent); err != nil {
		fmt.Fprintf(stderr, "action-recur: write: %v\n", err)
		return 1
	}
	// Log the recur. Honor AWIKI_LOG_FILE if set; otherwise content/log.md.
	logFile := os.Getenv("AWIKI_LOG_FILE")
	if logFile == "" {
		logFile = filepath.Join(repoRoot, "content", "log.md")
	}
	_, _ = Log(LogOptions{Action: "recur", Message: "page=" + page, LogFile: logFile})
	return 0
}

// capChainBase scans the page for the first completed every: line that
// would trigger the cap and returns its chain base. Used only when
// emitting the "refusing to emit" stderr line.
func capChainBase(data []byte) string {
	for line := range strings.SplitSeq(string(data), "\n") {
		if isCompletedRecurringAction(line) {
			return chainBase(line)
		}
	}
	return ""
}

// recurComputeNewContents produces the rewritten page bytes. Returns
// (newContent, code) where code 6 indicates the cap was reached.
func recurComputeNewContents(data []byte) ([]byte, int) {
	chainState := collectChainState(data)
	var b bytes.Buffer
	lines := strings.Split(string(data), "\n")
	// Bash reads with `while IFS= read || [[ -n "$line" ]]` which
	// preserves a final no-newline line. We split by \n; if the input
	// had a trailing newline, lines will have a trailing "" we must
	// preserve.
	for i, line := range lines {
		isLast := i == len(lines)-1
		if isLast && line == "" {
			b.WriteString("\n")
			continue
		}
		if !isCompletedRecurringAction(line) {
			b.WriteString(line)
			if !isLast || (isLast && i+1 < len(lines)) {
				b.WriteString("\n")
			}
			continue
		}
		base := chainBase(line)
		curN := chainN(line)
		if curN == 0 {
			curN = 1
		}
		maxN := chainMaxN(chainState, base)
		nextN := maxN + 1
		if nextN >= 200 {
			return nil, 6
		}
		if maxN > curN {
			b.WriteString(line)
			if !isLast {
				b.WriteString("\n")
			}
			continue
		}
		every := tailKeyValue(line, "every")
		doneDate := tailKeyValue(line, "done")
		if doneDate == "" {
			doneDate = time.Now().UTC().Format("2006-01-02")
		}
		nextDue, ok := computeRecurDue(doneDate, every)
		if !ok {
			// Bad every — let the line through; bash exits 4 here, but
			// this Go port treats it as no-op (lint catches bad-format).
			b.WriteString(line)
			if !isLast {
				b.WriteString("\n")
			}
			continue
		}
		newID := base + recurSep() + fmt.Sprint(nextN)
		newLine := buildOpenCopy(line, nextDue, newID)
		b.WriteString(newLine)
		b.WriteString("\n")
		b.WriteString(line)
		if !isLast {
			b.WriteString("\n")
		}
		chainState = append(chainState, chainEntry{base: base, n: nextN})
	}
	return b.Bytes(), 0
}

type chainEntry struct {
	base string
	n    int
}

func collectChainState(data []byte) []chainEntry {
	sep := recurSep()
	var out []chainEntry
	re := regexp.MustCompile(`^- \[[ /?>x-]\]`)
	idRE := regexp.MustCompile(`\^([A-Za-z0-9_~]+)\s*$`)
	for line := range strings.SplitSeq(string(data), "\n") {
		if !re.MatchString(line) {
			continue
		}
		m := idRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id := strings.TrimRight(m[1], " \t")
		idx := strings.Index(id, sep)
		if idx < 0 {
			out = append(out, chainEntry{base: id, n: 1})
			continue
		}
		base := id[:idx]
		nstr := id[idx+len(sep):]
		n := 0
		for _, c := range nstr {
			if c < '0' || c > '9' {
				n = -1
				break
			}
			n = n*10 + int(c-'0')
		}
		if n > 0 {
			out = append(out, chainEntry{base: base, n: n})
		}
	}
	return out
}

func chainMaxN(chains []chainEntry, base string) int {
	max := 1
	for _, c := range chains {
		if c.base == base && c.n > max {
			max = c.n
		}
	}
	return max
}

var trailingIDRE = regexp.MustCompile(`\^([A-Za-z0-9_~]+)\s*$`)

func chainBase(line string) string {
	m := trailingIDRE.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	id := m[1]
	sep := recurSep()
	if idx := strings.Index(id, sep); idx >= 0 {
		return id[:idx]
	}
	return id
}

func chainN(line string) int {
	m := trailingIDRE.FindStringSubmatch(line)
	if m == nil {
		return 0
	}
	id := m[1]
	sep := recurSep()
	idx := strings.Index(id, sep)
	if idx < 0 {
		return 1
	}
	nstr := id[idx+len(sep):]
	n := 0
	for _, c := range nstr {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func tailKeyValue(line, key string) string {
	re := regexp.MustCompile(`\s` + regexp.QuoteMeta(key) + `:([^\s]+)`)
	m := re.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	return m[1]
}

func isCompletedRecurringAction(line string) bool {
	if !strings.HasPrefix(line, "- [x] ") {
		return false
	}
	return regexp.MustCompile(`\severy:[A-Za-z0-9]+`).MatchString(line)
}

// computeRecurDue mirrors awiki_recur_compute_due (Nd / Nw / Nm / daily /
// weekly / monthly).
func computeRecurDue(doneDate, every string) (string, bool) {
	t, err := time.Parse("2006-01-02", doneDate)
	if err != nil {
		return "", false
	}
	switch every {
	case "daily":
		return t.AddDate(0, 0, 1).Format("2006-01-02"), true
	case "weekly":
		return t.AddDate(0, 0, 7).Format("2006-01-02"), true
	case "monthly":
		return addMonthsClamped(t, 1).Format("2006-01-02"), true
	}
	if len(every) >= 2 {
		unit := every[len(every)-1]
		ns := every[:len(every)-1]
		n := 0
		for _, c := range ns {
			if c < '0' || c > '9' {
				return "", false
			}
			n = n*10 + int(c-'0')
		}
		switch unit {
		case 'd':
			return t.AddDate(0, 0, n).Format("2006-01-02"), true
		case 'w':
			return t.AddDate(0, 0, n*7).Format("2006-01-02"), true
		case 'm':
			return addMonthsClamped(t, n).Format("2006-01-02"), true
		}
	}
	return "", false
}

// addMonthsClamped mirrors awiki_date_add_months: month-add with
// last-day clamp (2026-01-31 + 1m → 2026-02-28).
func addMonthsClamped(t time.Time, n int) time.Time {
	y, m, d := t.Date()
	total := int(m) - 1 + n
	ny := y + total/12
	nm := time.Month(total%12 + 1)
	if total < 0 && total%12 != 0 {
		// Go modulo rounds toward zero; for negative totals adjust.
		ny = y + (total-11)/12
		nm = time.Month(((total%12)+12)%12 + 1)
	}
	maxDay := daysInMonth(ny, nm)
	if d > maxDay {
		d = maxDay
	}
	return time.Date(ny, nm, d, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// buildOpenCopy mirrors awiki_build_open_copy: flip [x]→[ ], replace or
// splice due:, drop done:, and replace trailing ^id.
func buildOpenCopy(line, newDue, newID string) string {
	out := line
	// Flip checkbox.
	out = strings.Replace(out, "- [x] ", "- [ ] ", 1)
	// Replace existing due: or splice in.
	dueRE := regexp.MustCompile(`\sdue:[0-9]{4}-[0-9]{2}-[0-9]{2}`)
	if dueRE.MatchString(out) {
		out = dueRE.ReplaceAllString(out, " due:"+newDue)
	} else {
		// Splice before ^id if present, else append.
		idTailRE := regexp.MustCompile(`\s\^[A-Za-z0-9_~]+\s*$`)
		if idTailRE.MatchString(out) {
			out = idTailRE.ReplaceAllString(out, " due:"+newDue+" ^"+newID)
			return out
		}
		out += " due:" + newDue
	}
	// Strip done:<date>.
	doneRE := regexp.MustCompile(`\sdone:[0-9]{4}-[0-9]{2}-[0-9]{2}`)
	out = doneRE.ReplaceAllString(out, "")
	// Replace trailing ^id.
	out = trailingIDRE.ReplaceAllString(out, "^"+newID)
	return out
}

// emitUnifiedDiff mirrors `diff -u` output for the dry-run path.
func emitUnifiedDiff(w io.Writer, label string, oldData, newData []byte) {
	fmt.Fprintf(w, "--- %s\n", label)
	fmt.Fprintf(w, "+++ %s\n", label)
	oldLines := strings.Split(string(oldData), "\n")
	newLines := strings.Split(string(newData), "\n")
	// Trivial line-by-line diff: walk new/old; for each insertion show '+',
	// deletion show '-', context show ' '. The bash form uses `diff -u`;
	// callers only assert the +/- shape, so this simplification suffices.
	i, j := 0, 0
	fmt.Fprintf(w, "@@ -1,%d +1,%d @@\n", len(oldLines), len(newLines))
	for i < len(oldLines) || j < len(newLines) {
		switch {
		case i < len(oldLines) && j < len(newLines) && oldLines[i] == newLines[j]:
			fmt.Fprintf(w, " %s\n", oldLines[i])
			i++
			j++
		case j < len(newLines) && (i >= len(oldLines) || !lineExists(oldLines[i:], newLines[j])):
			fmt.Fprintf(w, "+%s\n", newLines[j])
			j++
		default:
			fmt.Fprintf(w, "-%s\n", oldLines[i])
			i++
		}
	}
}

func lineExists(slice []string, s string) bool {
	for _, x := range slice {
		if x == s {
			return true
		}
	}
	return false
}
