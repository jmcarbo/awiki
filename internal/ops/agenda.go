package ops

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"awiki/internal/region"
)

// AgendaOptions configures `awiki agenda`.
type AgendaOptions struct {
	RepoRoot       string
	Today          string // override for tests; empty → time.Now date
	IncludePrivate bool
}

// Agenda regenerates the five managed-region agenda pages from
// .awiki/maps/actions.tsv. Mirrors scripts/agenda.sh.
//
// Returns:
//
//	0 success
//	2 actions.tsv missing
//	5 INCLUDE_PRIVATE without git-crypt coverage
func Agenda(opts AgendaOptions, stdout, stderr io.Writer) int {
	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	today := opts.Today
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}
	actionsTSV := filepath.Join(repoRoot, ".awiki", "maps", "actions.tsv")
	agendaDir := filepath.Join(repoRoot, "content", "agenda")

	if _, err := os.Stat(actionsTSV); err != nil {
		fmt.Fprintf(stderr, "AGENDA|missing|%s — run action-scan.sh first\n", actionsTSV)
		return 2
	}

	includePrivate := opts.IncludePrivate || os.Getenv("AWIKI_AGENDA_INCLUDE_PRIVATE") == "1"
	if includePrivate {
		gitattrs, _ := os.ReadFile(filepath.Join(repoRoot, ".gitattributes"))
		re := regexp.MustCompile(`(?m)^\s*content/agenda/\*\*\s+filter=git-crypt`)
		if !re.Match(gitattrs) {
			fmt.Fprintln(stderr, "AGENDA|include-private-blocked|content/agenda/** not under git-crypt; refusing to inline private rows")
			return 5
		}
	}

	rows, err := readActionsTSV(actionsTSV)
	if err != nil {
		fmt.Fprintf(stderr, "agenda: read tsv: %v\n", err)
		return 1
	}

	bodies := map[string]string{
		"next-actions":   buildNextActions(rows, today),
		"today":          buildToday(rows, today),
		"waiting":        buildWaiting(rows),
		"someday":        buildSomeday(rows),
		"stuck-projects": buildStuckProjects(repoRoot, rows, today),
	}

	for _, region := range []string{"next-actions", "today", "waiting", "someday", "stuck-projects"} {
		if err := rewriteAgendaRegion(agendaDir, region, bodies[region], today); err != nil {
			fmt.Fprintf(stderr, "agenda: rewrite %s: %v\n", region, err)
			return 1
		}
	}

	fmt.Fprintf(stdout, "agenda regenerated: 5 regions, last_updated=%s\n", today)
	return 0
}

// AgendaCLI parses argv and dispatches Agenda.
func AgendaCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: awiki agenda")
		return 2
	}
	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	return Agenda(AgendaOptions{RepoRoot: repoRoot}, stdout, stderr)
}

// readActionsTSV parses .awiki/maps/actions.tsv into a slice of column
// arrays. The first row is the header.
func readActionsTSV(path string) ([][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows [][]string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 1024*1024), 4*1024*1024)
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		line := scanner.Text()
		if line == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		// Pad to 16 columns to mirror the bash awk safety.
		for len(cols) < 16 {
			cols = append(cols, "")
		}
		rows = append(rows, cols)
	}
	return rows, scanner.Err()
}

// Column indices (0-based; bash uses 1-based).
const (
	colID = iota
	colStatus
	colText
	colFile
	colLine
	colContext
	colDue
	colDefer
	colWait
	colSince
	colEvery
	colDone
	colPriority
	colEst
	colProject
	colSourceKind
)

// formatActionLine mirrors the awk formatter in scripts/agenda.sh.
func formatActionLine(row []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- [%s] %s", row[colStatus], row[colText])
	if row[colContext] != "" {
		b.WriteString(" ")
		b.WriteString(row[colContext])
	}
	if row[colDue] != "" {
		b.WriteString(" due:")
		b.WriteString(row[colDue])
	}
	if row[colDefer] != "" {
		b.WriteString(" defer:")
		b.WriteString(row[colDefer])
	}
	if row[colWait] != "" {
		w := strings.TrimPrefix(row[colWait], "[[")
		w = strings.TrimSuffix(w, "]]")
		b.WriteString(" wait:[[")
		b.WriteString(w)
		b.WriteString("]]")
	}
	if row[colSince] != "" {
		b.WriteString(" since:")
		b.WriteString(row[colSince])
	}
	if row[colEvery] != "" {
		b.WriteString(" every:")
		b.WriteString(row[colEvery])
	}
	if row[colDone] != "" {
		b.WriteString(" done:")
		b.WriteString(row[colDone])
	}
	if row[colPriority] != "" {
		b.WriteString(" priority:")
		b.WriteString(row[colPriority])
	}
	if row[colEst] != "" {
		b.WriteString(" est:")
		b.WriteString(row[colEst])
	}
	if row[colID] != "" {
		b.WriteString(" ^")
		b.WriteString(row[colID])
	}
	return b.String()
}

// privacyPartition splits rows into public + private count.
func privacyPartition(rows [][]string) (public [][]string, privateCount int) {
	for _, r := range rows {
		if r[colSourceKind] == "private" {
			privateCount++
			continue
		}
		public = append(public, r)
	}
	return
}

func emitHiddenPlaceholder(b *strings.Builder, count int) {
	if count == 0 {
		return
	}
	fmt.Fprintf(b, "\n> _%d action(s) hidden — origin under private/encrypted path._\n", count)
}

func filterStatus(rows [][]string, statuses string) [][]string {
	keep := map[string]bool{}
	for _, s := range strings.Split(statuses, ",") {
		keep[s] = true
	}
	var out [][]string
	for _, r := range rows {
		if keep[r[colStatus]] {
			out = append(out, r)
		}
	}
	return out
}

func buildNextActions(rows [][]string, today string) string {
	src := filterStatus(rows, " ,/")
	public, privateCount := privacyPartition(src)
	var b strings.Builder
	b.WriteString("## By Context\n\n")
	var filtered [][]string
	for _, r := range public {
		if r[colDefer] == "" || r[colDefer] <= today {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) > 0 {
		ctxSet := map[string]bool{}
		for _, r := range filtered {
			if r[colContext] != "" {
				ctxSet[r[colContext]] = true
			}
		}
		var contexts []string
		for c := range ctxSet {
			contexts = append(contexts, c)
		}
		sort.Strings(contexts)
		for _, c := range contexts {
			fmt.Fprintf(&b, "### %s\n\n", c)
			for _, r := range filtered {
				if r[colContext] == c {
					b.WriteString(formatActionLine(r))
					b.WriteString("\n")
				}
			}
			b.WriteString("\n")
		}
		// no-context bucket.
		var noCtx [][]string
		for _, r := range filtered {
			if r[colContext] == "" {
				noCtx = append(noCtx, r)
			}
		}
		if len(noCtx) > 0 {
			b.WriteString("### (no context)\n\n")
			for _, r := range noCtx {
				b.WriteString(formatActionLine(r))
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}
	emitHiddenPlaceholder(&b, privateCount)
	return b.String()
}

func buildToday(rows [][]string, today string) string {
	src := filterStatus(rows, " ,/")
	public, privateCount := privacyPartition(src)
	var b strings.Builder
	b.WriteString("## Today\n\n")
	var filtered [][]string
	for _, r := range public {
		if (r[colDue] != "" && r[colDue] <= today) || (r[colDefer] != "" && r[colDefer] <= today) {
			filtered = append(filtered, r)
		}
	}
	for _, r := range filtered {
		b.WriteString(formatActionLine(r))
		b.WriteString("\n")
	}
	if len(filtered) > 0 {
		b.WriteString("\n")
	}
	emitHiddenPlaceholder(&b, privateCount)
	return b.String()
}

func buildWaiting(rows [][]string) string {
	src := filterStatus(rows, "?")
	public, privateCount := privacyPartition(src)
	var b strings.Builder
	b.WriteString("## Waiting\n\n")
	if len(public) > 0 {
		set := map[string]bool{}
		for _, r := range public {
			if r[colWait] != "" {
				set[r[colWait]] = true
			}
		}
		var people []string
		for p := range set {
			people = append(people, p)
		}
		sort.Strings(people)
		for _, p := range people {
			fmt.Fprintf(&b, "### %s\n\n", p)
			for _, r := range public {
				if r[colWait] == p {
					b.WriteString(formatActionLine(r))
					b.WriteString("\n")
				}
			}
			b.WriteString("\n")
		}
		var noPerson [][]string
		for _, r := range public {
			if r[colWait] == "" {
				noPerson = append(noPerson, r)
			}
		}
		if len(noPerson) > 0 {
			b.WriteString("### (no wait person)\n\n")
			for _, r := range noPerson {
				b.WriteString(formatActionLine(r))
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}
	emitHiddenPlaceholder(&b, privateCount)
	return b.String()
}

func buildSomeday(rows [][]string) string {
	src := filterStatus(rows, ">")
	public, privateCount := privacyPartition(src)
	var b strings.Builder
	b.WriteString("## Someday\n\n")
	if len(public) > 0 {
		set := map[string]bool{}
		for _, r := range public {
			if r[colProject] != "" {
				set[r[colProject]] = true
			}
		}
		var projects []string
		for p := range set {
			projects = append(projects, p)
		}
		sort.Strings(projects)
		for _, p := range projects {
			fmt.Fprintf(&b, "### [[%s]]\n\n", p)
			for _, r := range public {
				if r[colProject] == p {
					b.WriteString(formatActionLine(r))
					b.WriteString("\n")
				}
			}
			b.WriteString("\n")
		}
		var unassigned [][]string
		for _, r := range public {
			if r[colProject] == "" {
				unassigned = append(unassigned, r)
			}
		}
		if len(unassigned) > 0 {
			b.WriteString("### (unassigned)\n\n")
			for _, r := range unassigned {
				b.WriteString(formatActionLine(r))
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}
	emitHiddenPlaceholder(&b, privateCount)
	return b.String()
}

func buildStuckProjects(repoRoot string, rows [][]string, today string) string {
	var b strings.Builder
	b.WriteString("## Stuck projects\n\n")
	projDir := filepath.Join(repoRoot, "content", "projects")
	if _, err := os.Stat(projDir); err != nil {
		return b.String()
	}
	cutoff := ""
	if t, err := time.Parse("2006-01-02", today); err == nil {
		cutoff = t.AddDate(0, 0, -14).Format("2006-01-02")
	}
	entries, _ := os.ReadDir(projDir)
	var names []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		full := filepath.Join(projDir, name)
		data, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		if !regexp.MustCompile(`(?m)^status:\s*active\s*$`).Match(data) {
			continue
		}
		slug := strings.TrimSuffix(name, ".md")
		openCount := 0
		for _, r := range rows {
			if r[colProject] == slug && (r[colStatus] == " " || r[colStatus] == "/") {
				openCount++
			}
		}
		if openCount == 0 {
			fmt.Fprintf(&b, "- [[%s]] — no open actions\n", slug)
			continue
		}
		if cutoff == "" {
			continue
		}
		lastUpd := ""
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "last_updated:") {
				v := strings.TrimSpace(line[len("last_updated:"):])
				v = strings.Trim(v, `"'`)
				v = strings.TrimSpace(v)
				lastUpd = v
				break
			}
		}
		if lastUpd != "" && lastUpd < cutoff {
			doneSince := 0
			for _, r := range rows {
				if r[colProject] == slug && r[colStatus] == "x" && r[colDone] != "" && r[colDone] > lastUpd {
					doneSince++
				}
			}
			if doneSince == 0 {
				fmt.Fprintf(&b, "- [[%s]] — last_updated %s, no [x] since\n", slug, lastUpd)
			}
		}
	}
	return b.String()
}

// rewriteAgendaRegion replaces the body inside `<!-- BEGIN agenda:<id> -->`
// markers in <agendaDir>/<id>.md and rewrites `last_updated:` in the
// frontmatter. Atomic write.
func rewriteAgendaRegion(agendaDir, id, body, today string) error {
	page := filepath.Join(agendaDir, id+".md")
	data, err := os.ReadFile(page)
	if err != nil {
		// Bash: emit "AGENDA|missing-page|<f>" to stderr, but do not
		// fail the run. Best-effort skip.
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	body = strings.TrimRight(body, "\n")
	// region.ManagedReplace requires at least one body line between
	// BEGIN/END to match. Empty placeholder pages have BEGIN immediately
	// followed by END on the next line; pre-pad those so the replace
	// path triggers (mirrors the awk getline-loop in scripts/agenda.sh).
	src := ensureRegionBody(string(data), id)
	updated, err := region.ManagedReplace(src, "agenda", id, body)
	if err != nil {
		return err
	}
	updated = rewriteLastUpdated(updated, today)
	tmp := page + ".tmp"
	if err := os.WriteFile(tmp, []byte(updated), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, page)
}

// ensureRegionBody pads an empty placeholder region (BEGIN immediately
// followed by END) so the canonical region.ManagedReplace pattern can
// match it. Idempotent for non-empty regions.
func ensureRegionBody(s, id string) string {
	begin := "<!-- BEGIN agenda:" + id + " -->"
	end := "<!-- END agenda:" + id + " -->"
	empty := begin + "\n" + end
	padded := begin + "\n\n" + end
	return strings.Replace(s, empty, padded, 1)
}

func rewriteLastUpdated(s, today string) string {
	// Operate only inside the leading frontmatter block.
	if !strings.HasPrefix(s, "---\n") {
		return s
	}
	end := strings.Index(s[4:], "\n---")
	if end < 0 {
		return s
	}
	front := s[:4+end]
	tail := s[4+end:]
	out := make([]string, 0, 32)
	for _, line := range strings.Split(front, "\n") {
		if strings.HasPrefix(line, "last_updated:") {
			line = "last_updated: " + today
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n") + tail
}
