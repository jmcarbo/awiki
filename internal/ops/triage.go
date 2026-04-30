package ops

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"awiki/internal/fsutil"
)

// TriageOutcome is one of the seven recognised outcome verbs.
type TriageOutcome string

const (
	OutcomeTrash           TriageOutcome = "trash"
	OutcomeDoNow           TriageOutcome = "do-now"
	OutcomeAct             TriageOutcome = "act"
	OutcomeDeferScheduled  TriageOutcome = "defer-scheduled"
	OutcomeWaiting         TriageOutcome = "waiting"
	OutcomeReference       TriageOutcome = "reference"
	OutcomeSomeday         TriageOutcome = "someday"
)

// TriageResult is the structured trailer payload emitted on success.
type TriageResult struct {
	ActionsTaken []string `json:"actions_taken"`
	CreatedPages []string `json:"created_pages"`
	UpdatedPages []string `json:"updated_pages"`
}

// triageState bundles the mutable bookkeeping for one outcome handler.
type triageState struct {
	repoRoot     string
	now          time.Time
	actionsTaken []string
	createdPages []string
	updatedPages []string
	idGen        func() (string, error)
}

func (s *triageState) today() string {
	return s.now.UTC().Format("2006-01-02")
}

// TriageOptions configures one non-interactive triage application.
type TriageOptions struct {
	RepoRoot string
	ID       string
	Outcome  TriageOutcome
	Params   map[string]string
	// Now overrides time.Now (for tests).
	Now time.Time
	// LockTimeout overrides AWIKI_LOCK_TIMEOUT_USER. Zero falls back to
	// fsutil.TimeoutUser.
	LockTimeout time.Duration
}

// validOutcomes mirrors $OUTCOMES in scripts/triage.sh.
var validOutcomes = map[TriageOutcome]bool{
	OutcomeTrash:          true,
	OutcomeDoNow:          true,
	OutcomeAct:            true,
	OutcomeDeferScheduled: true,
	OutcomeWaiting:        true,
	OutcomeReference:      true,
	OutcomeSomeday:        true,
}

// TriageApply runs a single non-interactive outcome application. Mirrors
// `bash scripts/triage.sh <id> <outcome> [k=v ...]`. Returns:
//
//	0 success (TRIAGE-RESULT|... emitted on stdout)
//	2 usage / unknown outcome
//	4 validation error (bad slug / date / missing param)
//	5 reference target already exists
//	7 lock contention
//	8 mint exhausted
//	9 stale id / id resolution failure
func TriageApply(opts TriageOptions, stdout, stderr io.Writer) int {
	if !validOutcomes[opts.Outcome] {
		fmt.Fprintf(stderr, "triage: unknown outcome: %s\n", opts.Outcome)
		return 2
	}
	if opts.RepoRoot == "" {
		opts.RepoRoot, _ = os.Getwd()
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	timeout := opts.LockTimeout
	if timeout == 0 {
		timeout = fsutil.TimeoutUser
		if env := os.Getenv("AWIKI_LOCK_TIMEOUT_USER"); env != "" {
			if n, err := strconv.Atoi(env); err == nil {
				timeout = time.Duration(n) * time.Second
			}
		}
	}
	if opts.Params == nil {
		opts.Params = map[string]string{}
	}

	var rc int
	var result *TriageResult
	lockPath := filepath.Join(opts.RepoRoot, ".awiki", "lock")
	err := fsutil.WithExclusiveLock(lockPath, timeout, func() error {
		c, r := triageApplyLocked(opts, stdout, stderr)
		rc = c
		result = r
		return nil
	})
	if err != nil {
		var ce *fsutil.ContentionError
		if errAsContention(err, &ce) {
			return ce.ExitCode()
		}
		fmt.Fprintf(stderr, "triage: %v\n", err)
		return 1
	}
	if rc != 0 {
		return rc
	}
	// Increment counter and maybe trigger agenda rebuild — runs after the
	// user lock is released. Non-fatal on errors.
	maybeRebuildAgenda(opts.RepoRoot, opts.Now, stdout, stderr)
	if result != nil {
		emitTriageTrailer(stdout, result)
	}
	return 0
}

func errAsContention(err error, target **fsutil.ContentionError) bool {
	if err == nil {
		return false
	}
	if ce, ok := err.(*fsutil.ContentionError); ok {
		*target = ce
		return true
	}
	return false
}

func triageApplyLocked(opts TriageOptions, stdout, stderr io.Writer) (int, *TriageResult) {
	st := &triageState{
		repoRoot: opts.RepoRoot,
		now:      opts.Now,
	}
	st.idGen = func() (string, error) { return mintActionID(st.repoRoot) }

	// 1. Resolve source.
	src, code := resolveTriageSource(opts.RepoRoot, opts.ID, opts.Params["lineno"], stderr)
	if code != 0 {
		return code, nil
	}
	// 2. Inbox TOCTOU.
	if src.kind == "inbox-line" {
		if code := verifyInboxLineID(opts.ID, src.path, src.lineno, stdout); code != 0 {
			return code, nil
		}
	}
	// 3. Outcome dispatch.
	if code := dispatchOutcome(st, opts.Outcome, src, opts.Params, stderr); code != 0 {
		return code, nil
	}
	// 4. Log via Log().
	logMsg := fmt.Sprintf("%s | %s | %s",
		string(opts.Outcome), opts.Params["project_slug"], opts.Params["ref_slug"])
	if _, err := Log(LogOptions{
		Action:  "triage",
		Message: logMsg,
		Now:     st.now,
		LogFile: resolveLogFile(opts.RepoRoot),
	}); err != nil {
		// Log failure is non-fatal in bash (it errors out before TRIAGE-RESULT),
		// but bash does abort under `set -e`. Match: return 1.
		fmt.Fprintf(stderr, "triage: log: %v\n", err)
		return 1, nil
	}
	return 0, &TriageResult{
		ActionsTaken: nilToEmpty(st.actionsTaken),
		CreatedPages: nilToEmpty(st.createdPages),
		UpdatedPages: nilToEmpty(st.updatedPages),
	}
}

func nilToEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// resolveLogFile mirrors the bash log resolution: AWIKI_LOG_FILE wins,
// otherwise <repo>/content/log.md (relative to cwd in bash).
func resolveLogFile(repoRoot string) string {
	if env := os.Getenv("AWIKI_LOG_FILE"); env != "" {
		return env
	}
	return filepath.Join(repoRoot, "content", "log.md")
}

// triageSource bundles resolver output.
type triageSource struct {
	kind   string // "inbox-line" | "raw-file" | "action-line"
	path   string
	lineno int
	text   string
}

func resolveTriageSource(repoRoot, id, linenoParam string, stderr io.Writer) (triageSource, int) {
	switch {
	case strings.HasPrefix(id, "inbox-"):
		path := filepath.Join(repoRoot, "content", "inbox.md")
		linenoStr := linenoParam
		if linenoStr == "" {
			re := regexp.MustCompile(`^inbox-[a-z0-9]{10}-([0-9]+)$`)
			m := re.FindStringSubmatch(id)
			if m == nil {
				fmt.Fprintf(stderr, "triage: inbox id missing lineno: %s\n", id)
				return triageSource{}, 9
			}
			linenoStr = m[1]
		}
		lineno, err := strconv.Atoi(linenoStr)
		if err != nil || lineno < 1 {
			fmt.Fprintf(stderr, "triage: bad lineno: %s\n", linenoStr)
			return triageSource{}, 9
		}
		text := extractInboxText(path, lineno)
		return triageSource{kind: "inbox-line", path: path, lineno: lineno, text: text}, 0
	case strings.HasPrefix(id, "file-"):
		want := strings.TrimPrefix(id, "file-")
		root := filepath.Join(repoRoot, "raw", "inbox", "interactive")
		var found string
		_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(repoRoot, p)
			h := sha1.Sum([]byte(rel))
			fsha := hex.EncodeToString(h[:])[:10]
			if fsha == want {
				found = rel
				return io.EOF
			}
			return nil
		})
		if found == "" {
			fmt.Fprintf(stderr, "triage: file id does not resolve: %s\n", id)
			return triageSource{}, 9
		}
		return triageSource{kind: "raw-file", path: filepath.Join(repoRoot, found), text: filepath.Base(found)}, 0
	default:
		// action-line id, look up in actions.tsv.
		tsv := filepath.Join(repoRoot, ".awiki", "maps", "actions.tsv")
		f, err := os.Open(tsv)
		if err != nil {
			fmt.Fprintf(stderr, "triage: action id not found in actions.tsv: %s\n", id)
			return triageSource{}, 9
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 1024*1024), 4*1024*1024)
		first := true
		for sc.Scan() {
			if first {
				first = false
				continue
			}
			cols := strings.Split(sc.Text(), "\t")
			if len(cols) < 5 {
				continue
			}
			if cols[0] != id {
				continue
			}
			lineno, _ := strconv.Atoi(cols[4])
			return triageSource{
				kind:   "action-line",
				path:   filepath.Join(repoRoot, cols[3]),
				lineno: lineno,
				text:   cols[2],
			}, 0
		}
		fmt.Fprintf(stderr, "triage: action id not found in actions.tsv: %s\n", id)
		return triageSource{}, 9
	}
}

// extractInboxText strips the leading "- YYYY-MM-DD HH:MM " from an inbox
// line at <lineno>. Mirrors awiki_extract_inbox_text in scripts/triage.sh.
func extractInboxText(path string, lineno int) string {
	raw, ok := readLine(path, lineno)
	if !ok {
		return ""
	}
	re := regexp.MustCompile(`^- [0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2} (.*)$`)
	m := re.FindStringSubmatch(raw)
	if m != nil {
		return m[1]
	}
	return raw
}

func readLine(path string, lineno int) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 4*1024*1024)
	n := 0
	for sc.Scan() {
		n++
		if n == lineno {
			return sc.Text(), true
		}
	}
	return "", false
}

// verifyInboxLineID re-reads the line at <lineno>, recomputes sha1[:10],
// compares to the embedded hash. Emits structured diagnostic and returns 9
// on any mismatch (matches bash awiki_verify_inbox_line_id).
func verifyInboxLineID(id, path string, lineno int, stdout io.Writer) int {
	re := regexp.MustCompile(`^inbox-([a-z0-9]{10})-([0-9]+)$`)
	m := re.FindStringSubmatch(id)
	if m == nil {
		fmt.Fprintln(stdout, `{"ok":false,"stale_id":true,"reason":"id_shape"}`)
		return 9
	}
	expectSha := m[1]
	expectLine := m[2]
	if expectLine != strconv.Itoa(lineno) {
		fmt.Fprintln(stdout, `{"ok":false,"stale_id":true,"reason":"lineno_mismatch"}`)
		return 9
	}
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintln(stdout, `{"ok":false,"stale_id":true,"reason":"inbox_missing"}`)
		return 9
	}
	total := countLines(path)
	if lineno < 1 || lineno > total {
		fmt.Fprintln(stdout, `{"ok":false,"stale_id":true,"reason":"lineno_out_of_range"}`)
		return 9
	}
	line, ok := readLine(path, lineno)
	if !ok {
		fmt.Fprintln(stdout, `{"ok":false,"stale_id":true,"reason":"lineno_out_of_range"}`)
		return 9
	}
	h := sha1.Sum([]byte(line))
	actualSha := hex.EncodeToString(h[:])[:10]
	if actualSha != expectSha {
		fmt.Fprintln(stdout, `{"ok":false,"stale_id":true,"reason":"sha_mismatch"}`)
		return 9
	}
	return 0
}

func countLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 4*1024*1024)
	n := 0
	for sc.Scan() {
		n++
	}
	return n
}

// === validation ===

var (
	projectSlugRE = regexp.MustCompile(`^[a-z0-9_][a-z0-9_-]{0,63}$`)
	plainSlugRE   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	isoDateRE     = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
)

func checkProjectSlug(s string, stderr io.Writer) int {
	if !projectSlugRE.MatchString(s) {
		fmt.Fprintf(stderr, "triage: bad project_slug: %s\n", s)
		return 4
	}
	return 0
}

func checkSlug(label, s string, stderr io.Writer) int {
	if !plainSlugRE.MatchString(s) {
		fmt.Fprintf(stderr, "triage: bad %s: %s\n", label, s)
		return 4
	}
	return 0
}

func checkPageType(s string, stderr io.Writer) int {
	switch s {
	case "entity", "concept", "topic", "source":
		return 0
	}
	fmt.Fprintf(stderr, "triage: bad page_type: %s\n", s)
	return 4
}

func checkISODate(label, s string, stderr io.Writer) int {
	if !isoDateRE.MatchString(s) {
		fmt.Fprintf(stderr, "triage: bad date (%s): %s\n", label, s)
		return 4
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		fmt.Fprintf(stderr, "triage: bad date (%s): %s\n", label, s)
		return 4
	}
	// Re-parse and reformat to catch out-of-range like 2026-02-30.
	t, _ := time.Parse("2006-01-02", s)
	if t.Format("2006-01-02") != s {
		fmt.Fprintf(stderr, "triage: bad date (%s): %s\n", label, s)
		return 4
	}
	return 0
}

// === ID minting ===

// mintActionID generates an 8-char [a-z0-9] id, retrying up to 5 times to
// avoid collisions with existing ids in actions.tsv.
func mintActionID(repoRoot string) (string, error) {
	tsv := filepath.Join(repoRoot, ".awiki", "maps", "actions.tsv")
	existing := map[string]bool{}
	if data, err := os.ReadFile(tsv); err == nil {
		first := true
		for _, line := range strings.Split(string(data), "\n") {
			if first {
				first = false
				continue
			}
			if i := strings.IndexByte(line, '\t'); i > 0 {
				existing[line[:i]] = true
			}
		}
	}
	for range 5 {
		id, err := randomActionID()
		if err != nil {
			return "", err
		}
		if !existing[id] {
			return id, nil
		}
	}
	return "", fmt.Errorf("triage: failed to mint unique id after 5 retries")
}

func randomActionID() (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	buf := make([]byte, 8)
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	for i, b := range raw {
		buf[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(buf), nil
}

// === project page ensure ===

// ensureProjectPage idempotently creates content/projects/<slug>.md when
// missing. Returns (path, created).
func ensureProjectPage(repoRoot, slug string, now time.Time) (string, bool, error) {
	path := filepath.Join(repoRoot, "content", "projects", slug+".md")
	if _, err := os.Stat(path); err == nil {
		return path, false, nil
	} else if !os.IsNotExist(err) {
		return path, false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, false, err
	}
	statusDefault := "active"
	title := slug
	switch slug {
	case "_someday":
		statusDefault = "someday"
		title = "Someday/Maybe (catch-all)"
	case "_loose":
		statusDefault = "active"
		title = "Loose actions (catch-all)"
	}
	today := now.UTC().Format("2006-01-02")
	body := fmt.Sprintf(`---
title: "%s"
date: %s
last_updated: %s
type: project
status: %s
outcome: ""
tags: []
draft: false
---

## Open Actions

## Done
`, title, today, today, statusDefault)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return path, false, err
	}
	return path, true, nil
}

func recordProjectPage(st *triageState, projPath string, created bool) {
	if created {
		st.createdPages = append(st.createdPages, projPath)
	} else {
		st.updatedPages = append(st.updatedPages, projPath)
	}
}

// appendUnderHeading inserts a single line right after the first occurrence
// of "## <heading>". If no heading is present, appends "\n## <heading>\n<line>"
// at end of file. Atomic write via tmp+rename.
func appendUnderHeading(path, heading, line string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	target := "## " + heading
	lines := strings.Split(string(src), "\n")
	out := make([]string, 0, len(lines)+2)
	inserted := false
	for _, ln := range lines {
		out = append(out, ln)
		if !inserted && strings.TrimRight(ln, " \t") == target {
			out = append(out, line)
			inserted = true
		}
	}
	if !inserted {
		out = append(out, "")
		out = append(out, target)
		out = append(out, line)
	}
	return atomicWrite(path, []byte(strings.Join(out, "\n")))
}

func atomicWrite(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// removeInboxLine drops line N atomically.
func removeInboxLine(path string, lineno int) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(src), "\n")
	out := make([]string, 0, len(lines))
	for i, ln := range lines {
		if i+1 == lineno {
			continue
		}
		out = append(out, ln)
	}
	return atomicWrite(path, []byte(strings.Join(out, "\n")))
}

// strikethroughInboxLine wraps line N with `~~...~~` atomically.
func strikethroughInboxLine(path string, lineno int) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(src), "\n")
	for i := range lines {
		if i+1 == lineno {
			lines[i] = "~~" + lines[i] + "~~"
		}
	}
	return atomicWrite(path, []byte(strings.Join(lines, "\n")))
}

// === outcome dispatch ===

func dispatchOutcome(st *triageState, outcome TriageOutcome, src triageSource, p map[string]string, stderr io.Writer) int {
	switch outcome {
	case OutcomeTrash:
		return outcomeTrash(st, src, stderr)
	case OutcomeDoNow:
		return outcomeDoNow(st, src, p, stderr)
	case OutcomeAct:
		return outcomeAct(st, src, p, stderr)
	case OutcomeDeferScheduled:
		return outcomeDeferScheduled(st, src, p, stderr)
	case OutcomeWaiting:
		return outcomeWaiting(st, src, p, stderr)
	case OutcomeReference:
		return outcomeReference(st, src, p, stderr)
	case OutcomeSomeday:
		return outcomeSomeday(st, src, p, stderr)
	}
	return 2
}

func outcomeTrash(st *triageState, src triageSource, stderr io.Writer) int {
	switch src.kind {
	case "inbox-line":
		if err := strikethroughInboxLine(src.path, src.lineno); err != nil {
			fmt.Fprintf(stderr, "triage: %v\n", err)
			return 1
		}
		st.actionsTaken = append(st.actionsTaken, fmt.Sprintf("strikethrough inbox line %d", src.lineno))
		st.updatedPages = append(st.updatedPages, src.path)
	case "raw-file":
		trash := filepath.Join(st.repoRoot, "raw", "inbox", ".trash")
		if err := os.MkdirAll(trash, 0o755); err != nil {
			fmt.Fprintf(stderr, "triage: %v\n", err)
			return 1
		}
		dst := filepath.Join(trash, filepath.Base(src.path))
		if err := os.Rename(src.path, dst); err != nil {
			fmt.Fprintf(stderr, "triage: %v\n", err)
			return 1
		}
		st.actionsTaken = append(st.actionsTaken,
			fmt.Sprintf("moved raw capture to .trash: %s", filepath.Base(src.path)))
	}
	return 0
}

func outcomeDoNow(st *triageState, src triageSource, p map[string]string, stderr io.Writer) int {
	if code := checkProjectSlug(p["project_slug"], stderr); code != 0 {
		return code
	}
	if code := checkSlug("context_slug", p["context_slug"], stderr); code != 0 {
		return code
	}
	projPath, created, err := ensureProjectPage(st.repoRoot, p["project_slug"], st.now)
	if err != nil {
		fmt.Fprintf(stderr, "triage: %v\n", err)
		return 1
	}
	recordProjectPage(st, projPath, created)
	id, err := st.idGen()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 8
	}
	line := fmt.Sprintf("- [x] %s @%s done:%s ^%s", src.text, p["context_slug"], st.today(), id)
	if err := appendUnderHeading(projPath, "Done", line); err != nil {
		fmt.Fprintf(stderr, "triage: %v\n", err)
		return 1
	}
	st.actionsTaken = append(st.actionsTaken,
		fmt.Sprintf("do-now @%s -> %s (^%s)", p["context_slug"], p["project_slug"], id))
	if src.kind == "inbox-line" {
		if err := removeInboxLine(src.path, src.lineno); err != nil {
			fmt.Fprintf(stderr, "triage: %v\n", err)
			return 1
		}
		st.updatedPages = append(st.updatedPages, src.path)
	}
	return 0
}

func outcomeAct(st *triageState, src triageSource, p map[string]string, stderr io.Writer) int {
	slug := p["project_slug"]
	if slug == "" {
		slug = "_loose"
	}
	if code := checkProjectSlug(slug, stderr); code != 0 {
		return code
	}
	if p["context_slug"] != "" {
		if code := checkSlug("context_slug", p["context_slug"], stderr); code != 0 {
			return code
		}
	}
	projPath, created, err := ensureProjectPage(st.repoRoot, slug, st.now)
	if err != nil {
		fmt.Fprintf(stderr, "triage: %v\n", err)
		return 1
	}
	recordProjectPage(st, projPath, created)
	id, err := st.idGen()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 8
	}
	ctxToken := ""
	if p["context_slug"] != "" {
		ctxToken = " @" + p["context_slug"]
	}
	line := fmt.Sprintf("- [ ] %s%s ^%s", src.text, ctxToken, id)
	if err := appendUnderHeading(projPath, "Open Actions", line); err != nil {
		fmt.Fprintf(stderr, "triage: %v\n", err)
		return 1
	}
	st.actionsTaken = append(st.actionsTaken,
		fmt.Sprintf("act%s -> %s (^%s)", ctxToken, slug, id))
	if src.kind == "inbox-line" {
		if err := removeInboxLine(src.path, src.lineno); err != nil {
			fmt.Fprintf(stderr, "triage: %v\n", err)
			return 1
		}
		st.updatedPages = append(st.updatedPages, src.path)
	}
	return 0
}

func outcomeDeferScheduled(st *triageState, src triageSource, p map[string]string, stderr io.Writer) int {
	slug := p["project_slug"]
	if slug == "" {
		slug = "_loose"
	}
	if code := checkProjectSlug(slug, stderr); code != 0 {
		return code
	}
	if p["context_slug"] != "" {
		if code := checkSlug("context_slug", p["context_slug"], stderr); code != 0 {
			return code
		}
	}
	dateToken := ""
	switch {
	case p["due"] != "":
		if code := checkISODate("due", p["due"], stderr); code != 0 {
			return code
		}
		dateToken = " due:" + p["due"]
	case p["defer"] != "":
		if code := checkISODate("defer", p["defer"], stderr); code != 0 {
			return code
		}
		dateToken = " defer:" + p["defer"]
	default:
		fmt.Fprintln(stderr, "triage: defer-scheduled requires due= or defer=")
		return 4
	}
	ctxToken := ""
	if p["context_slug"] != "" {
		ctxToken = " @" + p["context_slug"]
	}
	projPath, created, err := ensureProjectPage(st.repoRoot, slug, st.now)
	if err != nil {
		fmt.Fprintf(stderr, "triage: %v\n", err)
		return 1
	}
	recordProjectPage(st, projPath, created)
	id, err := st.idGen()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 8
	}
	line := fmt.Sprintf("- [ ] %s%s%s ^%s", src.text, ctxToken, dateToken, id)
	if err := appendUnderHeading(projPath, "Open Actions", line); err != nil {
		fmt.Fprintf(stderr, "triage: %v\n", err)
		return 1
	}
	st.actionsTaken = append(st.actionsTaken,
		fmt.Sprintf("defer-scheduled%s -> %s (^%s)", dateToken, slug, id))
	if src.kind == "inbox-line" {
		if err := removeInboxLine(src.path, src.lineno); err != nil {
			fmt.Fprintf(stderr, "triage: %v\n", err)
			return 1
		}
		st.updatedPages = append(st.updatedPages, src.path)
	}
	return 0
}

func outcomeWaiting(st *triageState, src triageSource, p map[string]string, stderr io.Writer) int {
	slug := p["project_slug"]
	if slug == "" {
		slug = "_loose"
	}
	if code := checkProjectSlug(slug, stderr); code != 0 {
		return code
	}
	if p["wait_for"] == "" {
		fmt.Fprintln(stderr, "triage: waiting requires wait_for=")
		return 4
	}
	if code := checkSlug("wait_for", p["wait_for"], stderr); code != 0 {
		return code
	}
	projPath, created, err := ensureProjectPage(st.repoRoot, slug, st.now)
	if err != nil {
		fmt.Fprintf(stderr, "triage: %v\n", err)
		return 1
	}
	recordProjectPage(st, projPath, created)
	id, err := st.idGen()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 8
	}
	line := fmt.Sprintf("- [?] %s wait:[[%s]] since:%s ^%s", src.text, p["wait_for"], st.today(), id)
	if err := appendUnderHeading(projPath, "Open Actions", line); err != nil {
		fmt.Fprintf(stderr, "triage: %v\n", err)
		return 1
	}
	st.actionsTaken = append(st.actionsTaken,
		fmt.Sprintf("waiting on [[%s]] -> %s (^%s)", p["wait_for"], slug, id))
	if src.kind == "inbox-line" {
		if err := removeInboxLine(src.path, src.lineno); err != nil {
			fmt.Fprintf(stderr, "triage: %v\n", err)
			return 1
		}
		st.updatedPages = append(st.updatedPages, src.path)
	}
	return 0
}

func outcomeReference(st *triageState, src triageSource, p map[string]string, stderr io.Writer) int {
	if code := checkPageType(p["page_type"], stderr); code != 0 {
		return code
	}
	if code := checkSlug("ref_slug", p["ref_slug"], stderr); code != 0 {
		return code
	}
	targetDir := filepath.Join(st.repoRoot, "content", p["page_type"]+"s")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "triage: %v\n", err)
		return 1
	}
	target := filepath.Join(targetDir, p["ref_slug"]+".md")
	canonical, _ := filepath.EvalSymlinks(targetDir)
	if canonical == "" {
		canonical = targetDir
	}
	repoCanon, _ := filepath.EvalSymlinks(st.repoRoot)
	if repoCanon == "" {
		repoCanon = st.repoRoot
	}
	expectedPrefix := filepath.Join(repoCanon, "content")
	if !strings.HasPrefix(canonical, expectedPrefix) {
		fmt.Fprintln(stderr, "triage: refusing to write outside content/<page_type>s/")
		return 4
	}
	if _, err := os.Stat(target); err == nil {
		fmt.Fprintf(stderr, "triage: reference target already exists: %s\n", target)
		return 5
	}
	today := st.today()
	body := fmt.Sprintf(`---
title: "%s"
date: %s
last_updated: %s
type: %s
tags: []
aliases: []
draft: false
---

(captured %s)
`, src.text, today, today, p["page_type"], today)
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		fmt.Fprintf(stderr, "triage: %v\n", err)
		return 1
	}
	st.actionsTaken = append(st.actionsTaken,
		fmt.Sprintf("reference -> %s/%s", p["page_type"], p["ref_slug"]))
	st.createdPages = append(st.createdPages, target)
	if src.kind == "inbox-line" {
		if err := removeInboxLine(src.path, src.lineno); err != nil {
			fmt.Fprintf(stderr, "triage: %v\n", err)
			return 1
		}
		st.updatedPages = append(st.updatedPages, src.path)
	}
	return 0
}

func outcomeSomeday(st *triageState, src triageSource, p map[string]string, stderr io.Writer) int {
	slug := p["project_slug"]
	if slug == "" {
		slug = "_someday"
	}
	if code := checkProjectSlug(slug, stderr); code != 0 {
		return code
	}
	projPath, created, err := ensureProjectPage(st.repoRoot, slug, st.now)
	if err != nil {
		fmt.Fprintf(stderr, "triage: %v\n", err)
		return 1
	}
	recordProjectPage(st, projPath, created)
	id, err := st.idGen()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 8
	}
	line := fmt.Sprintf("- [>] %s ^%s", src.text, id)
	if err := appendUnderHeading(projPath, "Open Actions", line); err != nil {
		fmt.Fprintf(stderr, "triage: %v\n", err)
		return 1
	}
	st.actionsTaken = append(st.actionsTaken,
		fmt.Sprintf("someday -> %s (^%s)", slug, id))
	if src.kind == "inbox-line" {
		if err := removeInboxLine(src.path, src.lineno); err != nil {
			fmt.Fprintf(stderr, "triage: %v\n", err)
			return 1
		}
		st.updatedPages = append(st.updatedPages, src.path)
	}
	return 0
}

// === TRIAGE-RESULT trailer ===

func emitTriageTrailer(stdout io.Writer, r *TriageResult) {
	fmt.Fprintf(stdout, "TRIAGE-RESULT|{%s,%s,%s}\n",
		jsonField("actions_taken", r.ActionsTaken),
		jsonField("created_pages", r.CreatedPages),
		jsonField("updated_pages", r.UpdatedPages))
}

func jsonField(name string, items []string) string {
	return fmt.Sprintf(`"%s":%s`, name, jsonStringArray(items))
}

func jsonStringArray(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, item := range items {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		// Escape \ and " (matching bash _triage_json_array).
		esc := strings.ReplaceAll(item, `\`, `\\`)
		esc = strings.ReplaceAll(esc, `"`, `\"`)
		b.WriteString(esc)
		b.WriteByte('"')
	}
	b.WriteByte(']')
	return b.String()
}

// === Threshold-resolved auto-rebuild ===

func resolveAgendaThreshold(repoRoot string) int {
	if v := os.Getenv("AWIKI_AGENDA_AFTER_N"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	cfg := filepath.Join(repoRoot, ".awiki", "config")
	if data, err := os.ReadFile(cfg); err == nil {
		// Last-occurrence wins (matches `awk ... | tail -1`).
		var last string
		for _, line := range strings.Split(string(data), "\n") {
			rest, ok := strings.CutPrefix(line, "AWIKI_AGENDA_AFTER_N=")
			if ok {
				last = rest
			}
		}
		if last != "" {
			if n, err := strconv.Atoi(last); err == nil {
				return n
			}
		}
	}
	return 5
}

// maybeRebuildAgenda increments .awiki/task-count and runs Agenda when the
// counter hits the resolved threshold. Best-effort logging mirrors bash.
func maybeRebuildAgenda(repoRoot string, now time.Time, stdout, stderr io.Writer) {
	countFile := filepath.Join(repoRoot, ".awiki", "task-count")
	n := 0
	if data, err := os.ReadFile(countFile); err == nil {
		if v, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			n = v
		}
	}
	n++
	_ = os.MkdirAll(filepath.Dir(countFile), 0o755)
	_ = os.WriteFile(countFile, []byte(strconv.Itoa(n)+"\n"), 0o644)

	threshold := resolveAgendaThreshold(repoRoot)
	if n < threshold {
		return
	}
	// Run agenda + log the rebuild.
	rc := Agenda(AgendaOptions{RepoRoot: repoRoot, Today: now.UTC().Format("2006-01-02")},
		io.Discard, io.Discard)
	logMsg := "rebuild"
	if rc != 0 {
		logMsg = fmt.Sprintf("rebuild | rc=%d", rc)
	}
	_, _ = Log(LogOptions{
		Action:  "agenda",
		Message: logMsg,
		Now:     now,
		LogFile: resolveLogFile(repoRoot),
	})
	_ = os.WriteFile(countFile, []byte("0\n"), 0o644)
}

// === Interactive walker ===

// triageWorkItem is one entry in the interactive walker's queue.
type triageWorkItem struct {
	kind   string // "inbox" | "raw"
	lineno int
	raw    string
	path   string
}

// TriageInteractive walks the inbox + raw/inbox/interactive/, prompting
// the operator for outcome + params per item, and applying each via
// TriageApply. EOF or empty outcome on any item skips it.
func TriageInteractive(opts TriageOptions, stdin io.Reader, stdout, stderr io.Writer) int {
	if opts.RepoRoot == "" {
		opts.RepoRoot, _ = os.Getwd()
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	br := bufio.NewReader(stdin)

	// 1. Snapshot inbox lines.
	inbox := filepath.Join(opts.RepoRoot, "content", "inbox.md")
	items := snapshotInbox(inbox)

	for _, item := range items {
		fmt.Fprintf(stdout, "\n--- inbox line %d ---\n%s\n", item.lineno, item.raw)
		outcome, ok := promptOutcome(br, stdout, stderr)
		if !ok || outcome == "" {
			continue
		}
		params := map[string]string{"lineno": strconv.Itoa(item.lineno)}
		if !promptParams(br, stdout, stderr, opts.RepoRoot, outcome, params) {
			continue
		}
		// Synthesize id.
		h := sha1.Sum([]byte(item.raw))
		id := fmt.Sprintf("inbox-%s-%d", hex.EncodeToString(h[:])[:10], item.lineno)
		sub := opts
		sub.ID = id
		sub.Outcome = TriageOutcome(outcome)
		sub.Params = params
		if rc := TriageApply(sub, stdout, stderr); rc != 0 {
			fmt.Fprintln(stderr, "(skipping; previous error)")
		}
	}

	// 2. Walk raw files.
	rawDir := filepath.Join(opts.RepoRoot, "raw", "inbox", "interactive")
	rawFiles := snapshotRaw(rawDir)
	for _, f := range rawFiles {
		fmt.Fprintf(stdout, "\n--- raw file %s ---\n", f)
		outcome, ok := promptOutcome(br, stdout, stderr)
		if !ok || outcome == "" {
			continue
		}
		params := map[string]string{}
		if !promptParams(br, stdout, stderr, opts.RepoRoot, outcome, params) {
			continue
		}
		rel, _ := filepath.Rel(opts.RepoRoot, f)
		h := sha1.Sum([]byte(rel))
		id := "file-" + hex.EncodeToString(h[:])[:10]
		sub := opts
		sub.ID = id
		sub.Outcome = TriageOutcome(outcome)
		sub.Params = params
		_ = TriageApply(sub, stdout, stderr)
	}
	return 0
}

func snapshotInbox(path string) []triageWorkItem {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 4*1024*1024)
	var items []triageWorkItem
	lineno := 0
	inFM := false
	fmSeen := false
	for sc.Scan() {
		lineno++
		line := sc.Text()
		if line == "---" {
			if !inFM && !fmSeen {
				inFM = true
				fmSeen = true
				continue
			} else if inFM {
				inFM = false
				continue
			}
		}
		if inFM {
			continue
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "~~") {
			continue
		}
		if !strings.HasPrefix(line, "-") {
			continue
		}
		items = append(items, triageWorkItem{kind: "inbox", lineno: lineno, raw: line})
	}
	return items
}

func snapshotRaw(dir string) []string {
	var out []string
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		out = append(out, p)
		return nil
	})
	sort.Strings(out)
	return out
}

func promptOutcome(br *bufio.Reader, stdout, stderr io.Writer) (string, bool) {
	for {
		fmt.Fprint(stdout, "Outcome (trash|do-now|act|defer-scheduled|waiting|reference|someday)? ")
		line, err := br.ReadString('\n')
		if err != nil && line == "" {
			return "", false
		}
		choice := strings.TrimRight(line, "\n\r")
		if validOutcomes[TriageOutcome(choice)] {
			return choice, true
		}
		if choice == "" {
			return "", true
		}
		fmt.Fprintln(stderr, "  unknown outcome; try again.")
	}
}

func promptParams(br *bufio.Reader, stdout, stderr io.Writer, repoRoot, outcome string, params map[string]string) bool {
	if _, ok := params["lineno"]; !ok {
		if !promptValue(br, stdout, stderr, "lineno", `^[0-9]+$`, "Lineno? ", params) {
			return false
		}
	}
	switch TriageOutcome(outcome) {
	case OutcomeTrash:
		// no extra params
	case OutcomeDoNow, OutcomeAct:
		if !promptSlug(br, stdout, stderr,
			"project_slug",
			fmt.Sprintf("Project slug (suggestions: %s)? ", slugHints(repoRoot, "projects")),
			true, params) {
			return false
		}
		if !promptSlug(br, stdout, stderr,
			"context_slug",
			fmt.Sprintf("Context slug (suggestions: %s)? ", slugHints(repoRoot, "contexts")),
			false, params) {
			return false
		}
	case OutcomeDeferScheduled:
		if !promptSlug(br, stdout, stderr, "project_slug", "Project slug? ", true, params) {
			return false
		}
		if !promptSlug(br, stdout, stderr, "context_slug", "Context slug? ", false, params) {
			return false
		}
		if !promptValue(br, stdout, stderr, "due", `^[0-9]{4}-[0-9]{2}-[0-9]{2}$`, "Due (YYYY-MM-DD, blank for defer)? ", params) {
			return false
		}
		if params["due"] == "" {
			if !promptValue(br, stdout, stderr, "defer", `^[0-9]{4}-[0-9]{2}-[0-9]{2}$`, "Defer (YYYY-MM-DD)? ", params) {
				return false
			}
		}
	case OutcomeWaiting:
		if !promptSlug(br, stdout, stderr, "project_slug", "Project slug? ", true, params) {
			return false
		}
		if !promptValue(br, stdout, stderr, "wait_for", `^[a-z0-9][a-z0-9-]{0,63}$`, "Wait-for entity slug? ", params) {
			return false
		}
	case OutcomeReference:
		if !promptValue(br, stdout, stderr, "page_type", `^(entity|concept|topic|source)$`, "Page type? ", params) {
			return false
		}
		if !promptValue(br, stdout, stderr, "ref_slug", `^[a-z0-9][a-z0-9-]{0,63}$`, "Reference slug? ", params) {
			return false
		}
	case OutcomeSomeday:
		if !promptSlug(br, stdout, stderr, "project_slug", "Project slug? ", true, params) {
			return false
		}
	}
	return true
}

func promptValue(br *bufio.Reader, stdout, stderr io.Writer, key, pattern, prompt string, params map[string]string) bool {
	re := regexp.MustCompile(pattern)
	for {
		fmt.Fprint(stdout, prompt)
		line, err := br.ReadString('\n')
		if err != nil && line == "" {
			return false
		}
		v := strings.TrimRight(line, "\n\r")
		if v == "" {
			params[key] = ""
			return true
		}
		if re.MatchString(v) {
			params[key] = v
			return true
		}
		fmt.Fprintln(stderr, "  invalid; try again.")
	}
}

func promptSlug(br *bufio.Reader, stdout, stderr io.Writer, key, prompt string, required bool, params map[string]string) bool {
	pattern := projectSlugRE
	if key != "project_slug" {
		pattern = plainSlugRE
	}
	for {
		fmt.Fprint(stdout, prompt)
		line, err := br.ReadString('\n')
		if err != nil && line == "" {
			return false
		}
		v := strings.TrimRight(line, "\n\r")
		if v == "" && !required {
			params[key] = ""
			return true
		}
		if v != "" && pattern.MatchString(v) {
			params[key] = v
			return true
		}
		fmt.Fprintln(stderr, "  invalid slug; try again.")
	}
}

func slugHints(repoRoot, kind string) string {
	dir := filepath.Join(repoRoot, "content", kind)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "none"
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "_index.md" {
			continue
		}
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		names = append(names, strings.TrimSuffix(name, ".md"))
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ",")
}

// === CLI ===

// TriageApplyCLI parses `awiki triage-apply <id> <outcome> [k=v ...]`.
func TriageApplyCLI(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("triage-apply", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: awiki triage-apply <id> <outcome> [k=v ...]")
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) < 2 {
		fs.Usage()
		return 2
	}
	id := rest[0]
	outcome := TriageOutcome(rest[1])
	params := map[string]string{}
	for _, kv := range rest[2:] {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			fmt.Fprintf(stderr, "triage: arg must be k=v: '%s'\n", kv)
			return 2
		}
		params[k] = v
	}
	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	return TriageApply(TriageOptions{
		RepoRoot: repoRoot,
		ID:       id,
		Outcome:  outcome,
		Params:   params,
	}, stdout, stderr)
}

// TriageCLI parses `awiki triage` (interactive walker).
func TriageCLI(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: awiki triage")
		return 2
	}
	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	return TriageInteractive(TriageOptions{RepoRoot: repoRoot}, stdin, stdout, stderr)
}
