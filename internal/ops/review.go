package ops

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"awiki/internal/lint"
)

// ReviewOptions configures `awiki review`. The full chain is
//
//	agenda → lint → review-status
//
// per scripts/agenda.sh + awiki lint + scripts/review-status.sh.
type ReviewOptions struct {
	RepoRoot string
	Today    string // override for tests; empty → time.Now date
}

// Review runs the review chain. Exits 0 on success; non-zero indicates
// genuine failure (agenda missing TSV, etc). Lint failures are surfaced
// in the lint section but do not abort the chain — the bash justfile
// recipe runs lint with `set -e` semantics, so a lint exit propagates,
// but the structured REVIEW| records are still emitted.
func Review(opts ReviewOptions, stdout, stderr io.Writer) int {
	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	// 1. Agenda regen.
	if code := Agenda(AgendaOptions{RepoRoot: repoRoot, Today: opts.Today}, stdout, stderr); code != 0 {
		return code
	}
	// 2. Lint.
	lintCollector, lintCode := lint.Run(lint.Options{
		RepoRoot:   repoRoot,
		ContentDir: filepath.Join(repoRoot, "content"),
		ToolRoot:   repoRoot,
	})
	for _, fix := range lintCollector.Fixes {
		fmt.Fprintln(stdout, fix.Record())
	}
	for _, d := range lintCollector.Diagnostics {
		fmt.Fprintln(stdout, d.Record())
	}
	fmt.Fprintln(stdout, lintCollector.Summary())
	// 3. review-status.
	if code := ReviewStatus(opts, stdout, stderr); code != 0 {
		return code
	}
	if lintCode != 0 {
		return lintCode
	}
	return 0
}

// ReviewCLI parses argv and dispatches Review.
func ReviewCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: awiki review")
		return 2
	}
	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	return Review(ReviewOptions{
		RepoRoot: repoRoot,
		Today:    os.Getenv("AWIKI_TODAY"),
	}, stdout, stderr)
}

// ReviewStatusCLI parses argv and dispatches just the ReviewStatus
// emitter (mirrors `bash scripts/review-status.sh`). Useful for callers
// that want the structured records without the agenda+lint chain that
// `awiki review` runs.
func ReviewStatusCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: awiki review-status")
		return 2
	}
	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	return ReviewStatus(ReviewOptions{
		RepoRoot: repoRoot,
		Today:    os.Getenv("AWIKI_TODAY"),
	}, stdout, stderr)
}

// ReviewStatus emits the eight REVIEW|... records + REVIEW-SUMMARY|.
// Mirrors scripts/review-status.sh byte-for-byte.
func ReviewStatus(opts ReviewOptions, stdout, stderr io.Writer) int {
	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	today := opts.Today
	if today == "" {
		today = os.Getenv("AWIKI_TODAY")
	}
	if today == "" {
		today = time.Now().UTC().Format("2006-01-02")
	}
	todayT, _ := time.Parse("2006-01-02", today)
	actionsTSV := filepath.Join(repoRoot, ".awiki", "maps", "actions.tsv")
	lastReviewFile := filepath.Join(repoRoot, ".awiki", "last-review")

	// 1. inbox-unprocessed.
	inboxCount := countInboxLines(filepath.Join(repoRoot, "content", "inbox.md"))
	fmt.Fprintf(stdout, "REVIEW|inbox-unprocessed|count=%d\n", inboxCount)

	// 2. raw-inbox-files.
	rawCount := countRawInbox(filepath.Join(repoRoot, "raw", "inbox", "interactive"))
	fmt.Fprintf(stdout, "REVIEW|raw-inbox-files|count=%d\n", rawCount)

	rows, _ := readActionsTSV(actionsTSV)

	// 3. projects-no-next-action.
	noNextSlugs := projectsNoNextAction(repoRoot, rows)
	if len(noNextSlugs) > 0 {
		fmt.Fprintf(stdout, "REVIEW|projects-no-next-action|count=%d|slugs=%s\n",
			len(noNextSlugs), strings.Join(noNextSlugs, ","))
	} else {
		fmt.Fprintln(stdout, "REVIEW|projects-no-next-action|count=0")
	}

	// 4. waiting-stale-14d.
	staleIDs := waitingStale14d(rows, todayT)
	if len(staleIDs) > 0 {
		fmt.Fprintf(stdout, "REVIEW|waiting-stale-14d|count=%d|ids=%s\n",
			len(staleIDs), strings.Join(staleIDs, ","))
	} else {
		fmt.Fprintln(stdout, "REVIEW|waiting-stale-14d|count=0")
	}

	// 5. overdue.
	overdueIDs := overdueActions(rows, today)
	if len(overdueIDs) > 0 {
		sort.Strings(overdueIDs)
		fmt.Fprintf(stdout, "REVIEW|overdue|count=%d|ids=%s\n",
			len(overdueIDs), strings.Join(overdueIDs, ","))
	} else {
		fmt.Fprintln(stdout, "REVIEW|overdue|count=0")
	}

	// 6. completed-since-last-review.
	lastReviewISO := readLastReview(lastReviewFile)
	lastReviewDate := lastReviewISO
	if idx := strings.Index(lastReviewDate, "T"); idx >= 0 {
		lastReviewDate = lastReviewDate[:idx]
	}
	if lastReviewDate == "" {
		lastReviewDate = "never"
	}
	completed := countCompletedSince(rows, lastReviewDate)
	fmt.Fprintf(stdout, "REVIEW|completed-since-last-review|count=%d\n", completed)

	// 7. stuck-projects.
	stuckSlugs := stuckProjects(repoRoot, rows, todayT)
	if len(stuckSlugs) > 0 {
		fmt.Fprintf(stdout, "REVIEW|stuck-projects|count=%d|slugs=%s\n",
			len(stuckSlugs), strings.Join(stuckSlugs, ","))
	} else {
		fmt.Fprintln(stdout, "REVIEW|stuck-projects|count=0")
	}

	// 8. someday-count.
	someday := 0
	for _, r := range rows {
		if r[colStatus] == ">" {
			someday++
		}
	}
	fmt.Fprintf(stdout, "REVIEW|someday-count|count=%d\n", someday)

	// summary.
	attention := len(overdueIDs) + len(staleIDs) + len(noNextSlugs) + len(stuckSlugs)
	fmt.Fprintf(stdout, "REVIEW-SUMMARY|last-review=%s|attention=%d\n", lastReviewDate, attention)
	return 0
}

func countInboxLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	fmCount := 0
	inBody := false
	dateRE := regexp.MustCompile(`^- \d{4}-\d{2}-\d{2}`)
	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "---" {
			fmCount++
			if fmCount == 2 {
				inBody = true
			}
			continue
		}
		if inBody && dateRE.MatchString(line) {
			count++
		}
	}
	return count
}

func countRawInbox(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		n++
	}
	return n
}

func projectsNoNextAction(repoRoot string, rows [][]string) []string {
	projDir := filepath.Join(repoRoot, "content", "projects")
	entries, err := os.ReadDir(projDir)
	if err != nil {
		return nil
	}
	openCounts := map[string]int{}
	for _, r := range rows {
		if r[colStatus] == " " || r[colStatus] == "/" {
			openCounts[r[colProject]]++
		}
	}
	var slugs []string
	var names []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		base := strings.TrimSuffix(name, ".md")
		switch base {
		case "_loose", "_someday", "_index":
			continue
		}
		data, err := os.ReadFile(filepath.Join(projDir, name))
		if err != nil {
			continue
		}
		if frontmatterField(data, "status") != "active" {
			continue
		}
		if openCounts[base] == 0 {
			slugs = append(slugs, base)
		}
	}
	return slugs
}

// frontmatterField scans the leading frontmatter for `key:` and returns
// the trimmed/quote-stripped value.
func frontmatterField(data []byte, key string) string {
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, key+":") {
			continue
		}
		v := strings.TrimSpace(line[len(key)+1:])
		v = strings.Trim(v, `"'`)
		return v
	}
	return ""
}

func waitingStale14d(rows [][]string, today time.Time) []string {
	var ids []string
	for _, r := range rows {
		if r[colStatus] != "?" {
			continue
		}
		if r[colSince] == "" {
			continue
		}
		t, err := time.Parse("2006-01-02", r[colSince])
		if err != nil {
			continue
		}
		days := int(today.Sub(t).Hours() / 24)
		if days > 14 {
			ids = append(ids, r[colID])
		}
	}
	return ids
}

func overdueActions(rows [][]string, today string) []string {
	var ids []string
	for _, r := range rows {
		if r[colStatus] != " " && r[colStatus] != "/" {
			continue
		}
		if r[colDue] == "" {
			continue
		}
		if r[colDue] < today {
			ids = append(ids, r[colID])
		}
	}
	return ids
}

func readLastReview(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func countCompletedSince(rows [][]string, cutoff string) int {
	if cutoff == "never" || cutoff == "" {
		// "no last-review baseline" → all completed.
		count := 0
		for _, r := range rows {
			if r[colStatus] == "x" {
				count++
			}
		}
		return count
	}
	count := 0
	for _, r := range rows {
		if r[colStatus] != "x" {
			continue
		}
		if r[colDone] == "" {
			continue
		}
		if r[colDone] >= cutoff {
			count++
		}
	}
	return count
}

func stuckProjects(repoRoot string, rows [][]string, today time.Time) []string {
	projDir := filepath.Join(repoRoot, "content", "projects")
	entries, err := os.ReadDir(projDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	var slugs []string
	for _, name := range names {
		base := strings.TrimSuffix(name, ".md")
		switch base {
		case "_loose", "_someday", "_index":
			continue
		}
		data, err := os.ReadFile(filepath.Join(projDir, name))
		if err != nil {
			continue
		}
		if frontmatterField(data, "status") != "active" {
			continue
		}
		lastUpd := frontmatterField(data, "last_updated")
		if lastUpd == "" {
			continue
		}
		t, err := time.Parse("2006-01-02", lastUpd)
		if err != nil {
			continue
		}
		days := int(today.Sub(t).Hours() / 24)
		if days <= 14 {
			continue
		}
		recentDone := 0
		for _, r := range rows {
			if r[colProject] == base && r[colStatus] == "x" && r[colDone] != "" && r[colDone] >= lastUpd {
				recentDone++
			}
		}
		if recentDone == 0 {
			slugs = append(slugs, base)
		}
	}
	return slugs
}
