package ops

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func setupTriageRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".awiki", "maps"))
	mustMkdir(t, filepath.Join(dir, "content", "projects"))
	mustMkdir(t, filepath.Join(dir, "content", "contexts"))
	mustMkdir(t, filepath.Join(dir, "raw", "inbox", "interactive"))
	mustMkdir(t, filepath.Join(dir, "raw", "inbox", "_trash"))
	mustWrite(t, filepath.Join(dir, ".awiki", "task-count"), "0\n")
	mustWrite(t, filepath.Join(dir, ".awiki", "last-review"), "2026-04-27T00:00:00Z")
	mustWrite(t, filepath.Join(dir, "content", "inbox.md"), `---
title: "Inbox"
type: inbox
draft: true
---

- 2026-04-27 14:32 call dentist about crown
- 2026-04-27 14:33 idea: rewrite onboarding email
`)
	t.Setenv("AWIKI_REPO_ROOT", dir)
	t.Setenv("AWIKI_LOG_FILE", filepath.Join(dir, ".awiki", "log"))
	return dir
}

// inboxIDForLine reads <inbox> line N and synthesises the inbox-<sha>-<lineno>
// id matching scripts/triage.sh's awiki_synthesize_inbox_id.
func inboxIDForLine(t *testing.T, inbox string, lineno int) string {
	t.Helper()
	line, ok := readLine(inbox, lineno)
	if !ok {
		t.Fatalf("inbox: line %d not found", lineno)
	}
	h := sha1.Sum([]byte(line))
	return fmt.Sprintf("inbox-%s-%d", hex.EncodeToString(h[:])[:10], lineno)
}

func TestTriageRejectsUnknownOutcome(t *testing.T) {
	repo := setupTriageRepo(t)
	var stdout, stderr bytes.Buffer
	rc := TriageApply(TriageOptions{
		RepoRoot: repo,
		ID:       "inbox-aaaaaaaaaa-7",
		Outcome:  "banana",
		Now:      time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC),
	}, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("rc=%d want 2; stderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unknown outcome") {
		t.Fatalf("missing unknown outcome msg: %s", stderr.String())
	}
}

func TestTriageTrashStrikethroughInboxLine(t *testing.T) {
	repo := setupTriageRepo(t)
	id := inboxIDForLine(t, filepath.Join(repo, "content", "inbox.md"), 7)
	var stdout, stderr bytes.Buffer
	rc := TriageApply(TriageOptions{
		RepoRoot: repo,
		ID:       id,
		Outcome:  OutcomeTrash,
		Params:   map[string]string{"lineno": "7"},
	}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
	}
	got := mustRead(t, filepath.Join(repo, "content", "inbox.md"))
	if !strings.Contains(got, "~~- 2026-04-27 14:32 call dentist about crown~~") {
		t.Fatalf("strikethrough not found:\n%s", got)
	}
	logBody := mustRead(t, filepath.Join(repo, ".awiki", "log"))
	if !strings.Contains(logBody, "triage | trash") {
		t.Fatalf("missing log line; got %q", logBody)
	}
}

func TestTriageDoNowAppendsToProject(t *testing.T) {
	repo := setupTriageRepo(t)
	mustWrite(t, filepath.Join(repo, "content", "projects", "dentist.md"),
		`---
title: "Dentist"
type: project
status: active
draft: false
---

## Open Actions

## Done
`)
	id := inboxIDForLine(t, filepath.Join(repo, "content", "inbox.md"), 7)
	rc := TriageApply(TriageOptions{
		RepoRoot: repo,
		ID:       id,
		Outcome:  OutcomeDoNow,
		Params: map[string]string{
			"project_slug": "dentist",
			"context_slug": "phone",
			"lineno":       "7",
		},
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	dent := mustRead(t, filepath.Join(repo, "content", "projects", "dentist.md"))
	re := regexp.MustCompile(`^- \[x\] call dentist about crown @phone done:\d{4}-\d{2}-\d{2} \^[a-z0-9]{8}$`)
	if !lineMatch(dent, re) {
		t.Fatalf("dentist.md missing [x] line:\n%s", dent)
	}
	inbox := mustRead(t, filepath.Join(repo, "content", "inbox.md"))
	if strings.Contains(inbox, "call dentist about crown") {
		t.Fatalf("inbox should not still contain captured line:\n%s", inbox)
	}
}

func TestTriageActLooseLazilyCreatesPage(t *testing.T) {
	repo := setupTriageRepo(t)
	id := inboxIDForLine(t, filepath.Join(repo, "content", "inbox.md"), 7)
	rc := TriageApply(TriageOptions{
		RepoRoot: repo,
		ID:       id,
		Outcome:  OutcomeAct,
		Params: map[string]string{
			"project_slug": "_loose",
			"context_slug": "phone",
			"lineno":       "7",
		},
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	loose := mustRead(t, filepath.Join(repo, "content", "projects", "_loose.md"))
	if !strings.Contains(loose, "type: project") {
		t.Fatalf("_loose.md missing type: project:\n%s", loose)
	}
	if !strings.Contains(loose, "status: active") {
		t.Fatalf("_loose.md missing status: active:\n%s", loose)
	}
	re := regexp.MustCompile(`^- \[ \] call dentist about crown @phone \^[a-z0-9]{8}$`)
	if !lineMatch(loose, re) {
		t.Fatalf("_loose.md missing [ ] line:\n%s", loose)
	}
}

func TestTriageSomedayLazilyCreates(t *testing.T) {
	repo := setupTriageRepo(t)
	id := inboxIDForLine(t, filepath.Join(repo, "content", "inbox.md"), 7)
	rc := TriageApply(TriageOptions{
		RepoRoot: repo,
		ID:       id,
		Outcome:  OutcomeSomeday,
		Params: map[string]string{
			"project_slug": "_someday",
			"lineno":       "7",
		},
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	pg := mustRead(t, filepath.Join(repo, "content", "projects", "_someday.md"))
	if !strings.Contains(pg, "status: someday") {
		t.Fatalf("_someday.md missing status: someday:\n%s", pg)
	}
	re := regexp.MustCompile(`^- \[>\] call dentist about crown \^[a-z0-9]{8}$`)
	if !lineMatch(pg, re) {
		t.Fatalf("_someday.md missing [>] line:\n%s", pg)
	}
}

func TestTriageDeferScheduledDue(t *testing.T) {
	repo := setupTriageRepo(t)
	mustWrite(t, filepath.Join(repo, "content", "projects", "dentist.md"),
		`---
title: "Dentist"
type: project
status: active
draft: false
---

## Open Actions

## Done
`)
	id := inboxIDForLine(t, filepath.Join(repo, "content", "inbox.md"), 7)
	rc := TriageApply(TriageOptions{
		RepoRoot: repo,
		ID:       id,
		Outcome:  OutcomeDeferScheduled,
		Params: map[string]string{
			"project_slug": "dentist",
			"context_slug": "phone",
			"due":          "2026-05-15",
			"lineno":       "7",
		},
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	dent := mustRead(t, filepath.Join(repo, "content", "projects", "dentist.md"))
	re := regexp.MustCompile(`^- \[ \] call dentist about crown @phone due:2026-05-15 \^[a-z0-9]{8}$`)
	if !lineMatch(dent, re) {
		t.Fatalf("dentist missing due: line:\n%s", dent)
	}
}

func TestTriageDeferScheduledRejectsBadDate(t *testing.T) {
	repo := setupTriageRepo(t)
	id := inboxIDForLine(t, filepath.Join(repo, "content", "inbox.md"), 7)
	var stderr bytes.Buffer
	rc := TriageApply(TriageOptions{
		RepoRoot: repo,
		ID:       id,
		Outcome:  OutcomeDeferScheduled,
		Params: map[string]string{
			"project_slug": "dentist",
			"due":          "2026-02-30",
			"lineno":       "7",
		},
	}, &bytes.Buffer{}, &stderr)
	if rc != 4 {
		t.Fatalf("rc=%d want 4; stderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "bad date") {
		t.Fatalf("missing bad date msg: %s", stderr.String())
	}
}

func TestTriageWaitingStampsSinceToday(t *testing.T) {
	repo := setupTriageRepo(t)
	mustWrite(t, filepath.Join(repo, "content", "projects", "q3.md"),
		`---
title: "Q3"
type: project
status: active
draft: false
---

## Open Actions

## Done
`)
	id := inboxIDForLine(t, filepath.Join(repo, "content", "inbox.md"), 7)
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	rc := TriageApply(TriageOptions{
		RepoRoot: repo,
		ID:       id,
		Outcome:  OutcomeWaiting,
		Now:      now,
		Params: map[string]string{
			"project_slug": "q3",
			"wait_for":     "bob-smith",
			"lineno":       "7",
		},
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	pg := mustRead(t, filepath.Join(repo, "content", "projects", "q3.md"))
	want := regexp.MustCompile(`^- \[\?\] call dentist about crown wait:\[\[bob-smith\]\] since:2026-04-27 \^[a-z0-9]{8}$`)
	if !lineMatch(pg, want) {
		t.Fatalf("q3 missing waiting line:\n%s", pg)
	}
}

func TestTriageReferenceWritesNewPage(t *testing.T) {
	repo := setupTriageRepo(t)
	id := inboxIDForLine(t, filepath.Join(repo, "content", "inbox.md"), 8)
	rc := TriageApply(TriageOptions{
		RepoRoot: repo,
		ID:       id,
		Outcome:  OutcomeReference,
		Params: map[string]string{
			"page_type": "concept",
			"ref_slug":  "onboarding-email-rewrite",
			"lineno":    "8",
		},
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	pg := mustRead(t, filepath.Join(repo, "content", "concepts", "onboarding-email-rewrite.md"))
	if !strings.Contains(pg, "type: concept") {
		t.Fatalf("missing type: concept:\n%s", pg)
	}
	inbox := mustRead(t, filepath.Join(repo, "content", "inbox.md"))
	if strings.Contains(inbox, "idea: rewrite onboarding email") {
		t.Fatalf("inbox still contains line 8:\n%s", inbox)
	}
}

func TestTriageRejectsBadProjectSlug(t *testing.T) {
	repo := setupTriageRepo(t)
	id := inboxIDForLine(t, filepath.Join(repo, "content", "inbox.md"), 7)
	rc := TriageApply(TriageOptions{
		RepoRoot: repo,
		ID:       id,
		Outcome:  OutcomeAct,
		Params: map[string]string{
			"project_slug": "../etc/passwd",
			"lineno":       "7",
		},
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if rc != 4 {
		t.Fatalf("rc=%d want 4", rc)
	}
}

func TestTriageRejectsStaleInboxID(t *testing.T) {
	repo := setupTriageRepo(t)
	id := inboxIDForLine(t, filepath.Join(repo, "content", "inbox.md"), 7)
	// Mutate the inbox line so the recomputed sha differs.
	body := mustRead(t, filepath.Join(repo, "content", "inbox.md"))
	body = strings.Replace(body,
		"- 2026-04-27 14:32 call dentist about crown",
		"- 2026-04-27 14:32 something else entirely", 1)
	mustWrite(t, filepath.Join(repo, "content", "inbox.md"), body)
	var stdout bytes.Buffer
	rc := TriageApply(TriageOptions{
		RepoRoot: repo,
		ID:       id,
		Outcome:  OutcomeAct,
		Params: map[string]string{
			"project_slug": "_loose",
			"context_slug": "phone",
			"lineno":       "7",
		},
	}, &stdout, &bytes.Buffer{})
	if rc != 9 {
		t.Fatalf("rc=%d want 9; stdout=%s", rc, stdout.String())
	}
	if !strings.Contains(stdout.String(), `"stale_id":true`) {
		t.Fatalf("expected stale_id JSON; got %s", stdout.String())
	}
	// Stale-id path must NOT emit TRIAGE-RESULT trailer.
	if strings.Contains(stdout.String(), "TRIAGE-RESULT|") {
		t.Fatalf("stale-id should suppress TRIAGE-RESULT; got %s", stdout.String())
	}
	// Inbox still contains the mutated line.
	if !strings.Contains(mustRead(t, filepath.Join(repo, "content", "inbox.md")),
		"something else entirely") {
		t.Fatalf("inbox mutation lost")
	}
}

func TestTriageEmitsResultTrailer(t *testing.T) {
	repo := setupTriageRepo(t)
	id := inboxIDForLine(t, filepath.Join(repo, "content", "inbox.md"), 7)
	var stdout bytes.Buffer
	rc := TriageApply(TriageOptions{
		RepoRoot: repo,
		ID:       id,
		Outcome:  OutcomeAct,
		Params: map[string]string{
			"project_slug": "renovate-kitchen",
			"context_slug": "phone",
			"lineno":       "7",
		},
	}, &stdout, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	out := stdout.String()
	if !strings.Contains(out, "TRIAGE-RESULT|") {
		t.Fatalf("missing TRIAGE-RESULT trailer; got %s", out)
	}
	if !strings.Contains(out, `"actions_taken":[`) {
		t.Fatalf("missing actions_taken array: %s", out)
	}
	if !strings.Contains(out, `"created_pages":[`) {
		t.Fatalf("missing created_pages array: %s", out)
	}
	if !strings.Contains(out, `"updated_pages":[`) {
		t.Fatalf("missing updated_pages array: %s", out)
	}
}

func TestTriageThresholdTriggersAgendaRebuild(t *testing.T) {
	repo := setupTriageRepo(t)
	t.Setenv("AWIKI_AGENDA_AFTER_N", "1")
	mustMkdir(t, filepath.Join(repo, "content", "agenda"))
	mustWrite(t, filepath.Join(repo, "content", "agenda", "next-actions.md"),
		`---
title: "Next actions"
type: agenda
last_updated: 2025-01-01
draft: false
---
<!-- BEGIN agenda:next-actions -->
<!-- END agenda:next-actions -->
`)
	mustWrite(t, filepath.Join(repo, "content", "agenda", "today.md"),
		`---
title: "Today"
type: agenda
last_updated: 2025-01-01
draft: false
---
<!-- BEGIN agenda:today -->
<!-- END agenda:today -->
`)
	mustWrite(t, filepath.Join(repo, "content", "agenda", "waiting.md"),
		`---
title: "Waiting"
type: agenda
last_updated: 2025-01-01
draft: false
---
<!-- BEGIN agenda:waiting -->
<!-- END agenda:waiting -->
`)
	mustWrite(t, filepath.Join(repo, "content", "agenda", "someday.md"),
		`---
title: "Someday"
type: agenda
last_updated: 2025-01-01
draft: false
---
<!-- BEGIN agenda:someday -->
<!-- END agenda:someday -->
`)
	mustWrite(t, filepath.Join(repo, "content", "agenda", "stuck-projects.md"),
		`---
title: "Stuck"
type: agenda
last_updated: 2025-01-01
draft: false
---
<!-- BEGIN agenda:stuck-projects -->
<!-- END agenda:stuck-projects -->
`)
	mustWrite(t, filepath.Join(repo, ".awiki", "maps", "actions.tsv"),
		"id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n")
	id := inboxIDForLine(t, filepath.Join(repo, "content", "inbox.md"), 7)
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	rc := TriageApply(TriageOptions{
		RepoRoot: repo,
		ID:       id,
		Outcome:  OutcomeTrash,
		Now:      now,
		Params:   map[string]string{"lineno": "7"},
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	pg := mustRead(t, filepath.Join(repo, "content", "agenda", "next-actions.md"))
	if !strings.Contains(pg, "last_updated: 2026-04-27") {
		t.Fatalf("agenda last_updated not advanced:\n%s", pg)
	}
	logBody := mustRead(t, filepath.Join(repo, ".awiki", "log"))
	if !strings.Contains(logBody, "agenda | rebuild") {
		t.Fatalf("missing agenda rebuild log: %s", logBody)
	}
	// Counter resets to 0.
	cnt := strings.TrimSpace(mustRead(t, filepath.Join(repo, ".awiki", "task-count")))
	if cnt != "0" {
		t.Fatalf("counter not reset: %q", cnt)
	}
}

func TestTriageThresholdConfigOverridesDefault(t *testing.T) {
	repo := setupTriageRepo(t)
	mustWrite(t, filepath.Join(repo, ".awiki", "config"), "AWIKI_AGENDA_AFTER_N=2\n")
	got := resolveAgendaThreshold(repo)
	if got != 2 {
		t.Fatalf("got=%d want 2", got)
	}
	t.Setenv("AWIKI_AGENDA_AFTER_N", "9")
	got = resolveAgendaThreshold(repo)
	if got != 9 {
		t.Fatalf("env should override config; got=%d", got)
	}
}

// === helpers ===

func mustRead(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(b)
}

func lineMatch(body string, re *regexp.Regexp) bool {
	for _, line := range strings.Split(body, "\n") {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}
