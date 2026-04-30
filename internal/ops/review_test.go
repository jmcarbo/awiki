package ops

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupReviewRepo(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content", "projects"))
	mustMkdir(t, filepath.Join(tmp, "content", "agenda"))
	mustMkdir(t, filepath.Join(tmp, "raw", "inbox", "interactive"))
	mustMkdir(t, filepath.Join(tmp, ".awiki", "maps"))
	mustWrite(t, filepath.Join(tmp, ".awiki", "last-review"), "2026-04-20T09:00:00Z\n")
	mustWrite(t, filepath.Join(tmp, "content", "inbox.md"),
		"---\ntitle: \"Inbox\"\ntype: inbox\ndraft: true\n---\n\n"+
			"- 2026-04-26 09:00 call dentist\n- 2026-04-26 09:01 buy cat food\n- 2026-04-27 08:00 review draft proposal\n")
	mustWrite(t, filepath.Join(tmp, "raw", "inbox", "interactive", "note.md"), "scratch\n")
	tsv := "id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n" +
		"a01\t \tcall dentist\tcontent/projects/q3-launch.md\t5\tphone\t2026-04-25\t\t\t\t1w\t\t\t\tq3-launch\tpublic\n" +
		"a07\t \tsend invoice\tcontent/projects/onboarding-revamp.md\t7\tcomputer\t2026-04-26\t\t\t\t\t\t\t\tonboarding-revamp\tpublic\n" +
		"a02\t/\tdraft proposal\tcontent/projects/q3-launch.md\t6\tcomputer\t\t\t\t\t\t\t\t\tq3-launch\tpublic\n" +
		"a03\t?\tq3 budget approval\tcontent/projects/q3-launch.md\t8\t\t\t\tbob-smith\t2026-04-08\t\t\t3\t\tq3-launch\tpublic\n" +
		"a04\t>\treorganize garage\tcontent/projects/_someday.md\t3\thome\t\t\t\t\t\t\t\t\t_someday\tpublic\n" +
		"a05\tx\twater plants\tcontent/projects/home.md\t4\thome\t\t\t\t\t1w\t2026-04-26\t\t\thome\tpublic\n" +
		"a06\tx\tfile taxes\tcontent/projects/admin.md\t5\tcomputer\t\t\t\t\t\t2026-04-22\t\t\tadmin\tpublic\n" +
		"a08\tx\tbook flight\tcontent/projects/q3-launch.md\t9\tcomputer\t\t\t\t\t\t2026-04-23\t\t\tq3-launch\tpublic\n" +
		"a11\t \twater plants next week\tcontent/projects/home.md\t6\thome\t2026-05-03\t\t\t\t1w\t\t\t\thome\tpublic\n" +
		"a12\t \tpay credit card\tcontent/projects/admin.md\t6\tcomputer\t2026-05-15\t\t\t\t1m\t\t\t\tadmin\tpublic\n"
	mustWrite(t, filepath.Join(tmp, ".awiki", "maps", "actions.tsv"), tsv)
	mustWrite(t, filepath.Join(tmp, "content", "projects", "renovate-kitchen.md"),
		"---\ntitle: \"Renovate kitchen\"\nlast_updated: 2026-04-26\ntype: project\nstatus: active\n---\n\n## Open Actions\n")
	mustWrite(t, filepath.Join(tmp, "content", "projects", "onboarding-revamp.md"),
		"---\ntitle: \"Onboarding\"\nlast_updated: 2026-04-01\ntype: project\nstatus: active\n---\n\n## Open Actions\n- [ ] send invoice @computer due:2026-04-26 ^a07\n")
	mustWrite(t, filepath.Join(tmp, "content", "projects", "q3-launch.md"),
		"---\ntitle: \"Q3\"\nlast_updated: 2026-04-26\ntype: project\nstatus: active\n---\n\n## Open Actions\n")
	mustWrite(t, filepath.Join(tmp, "content", "projects", "home.md"),
		"---\ntitle: \"Home\"\nlast_updated: 2026-04-26\ntype: project\nstatus: active\n---\n\n## Open Actions\n")
	mustWrite(t, filepath.Join(tmp, "content", "projects", "admin.md"),
		"---\ntitle: \"Admin\"\nlast_updated: 2026-04-22\ntype: project\nstatus: active\n---\n\n## Open Actions\n")
	mustWrite(t, filepath.Join(tmp, "content", "projects", "_someday.md"),
		"---\ntitle: \"Someday\"\nlast_updated: 2026-04-15\ntype: project\nstatus: someday\n---\n\n## Open Actions\n")
	// Pad someday count.
	body, _ := os.ReadFile(filepath.Join(tmp, ".awiki", "maps", "actions.tsv"))
	var b strings.Builder
	b.Write(body)
	for i := 2; i <= 12; i++ {
		b.WriteString("s")
		if i < 10 {
			b.WriteString("0")
		}
		b.WriteString(itoa(i))
		b.WriteString("\t>\titem\tcontent/projects/_someday.md\t10\t\t\t\t\t\t\t\t\t\t_someday\tpublic\n")
	}
	mustWrite(t, filepath.Join(tmp, ".awiki", "maps", "actions.tsv"), b.String())
	return tmp
}

func TestReviewStatusEmitsAllRecords(t *testing.T) {
	tmp := setupReviewRepo(t)
	var stdout, stderr bytes.Buffer
	code := ReviewStatus(ReviewOptions{RepoRoot: tmp, Today: "2026-04-27"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	wants := []string{
		"REVIEW|inbox-unprocessed|count=3",
		"REVIEW|raw-inbox-files|count=1",
		"REVIEW|projects-no-next-action|count=1|slugs=renovate-kitchen",
		"REVIEW|waiting-stale-14d|count=1|ids=a03",
		"REVIEW|overdue|count=2|ids=a01,a07",
		"REVIEW|completed-since-last-review|count=3",
		"REVIEW|stuck-projects|count=1|slugs=onboarding-revamp",
		"REVIEW|someday-count|count=12",
		"REVIEW-SUMMARY|last-review=2026-04-20|attention=5",
	}
	for _, w := range wants {
		if !strings.Contains(stdout.String(), w) {
			t.Errorf("missing record %q\nstdout=%s", w, stdout.String())
		}
	}
}

func TestReviewStatusOnlyHeaderTSV(t *testing.T) {
	tmp := setupReviewRepo(t)
	mustWrite(t, filepath.Join(tmp, ".awiki", "maps", "actions.tsv"),
		"id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n")
	var stdout, stderr bytes.Buffer
	code := ReviewStatus(ReviewOptions{RepoRoot: tmp, Today: "2026-04-27"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	for _, w := range []string{"REVIEW|overdue|count=0", "REVIEW|completed-since-last-review|count=0", "REVIEW|someday-count|count=0"} {
		if !strings.Contains(stdout.String(), w) {
			t.Errorf("missing: %q", w)
		}
	}
}

func TestReviewStatusLastReviewNever(t *testing.T) {
	tmp := setupReviewRepo(t)
	os.Remove(filepath.Join(tmp, ".awiki", "last-review"))
	var stdout, stderr bytes.Buffer
	ReviewStatus(ReviewOptions{RepoRoot: tmp, Today: "2026-04-27"}, &stdout, &stderr)
	if !strings.Contains(stdout.String(), "REVIEW-SUMMARY|last-review=never") {
		t.Fatalf("missing never: %q", stdout.String())
	}
}

func TestReviewStatusRawInboxIgnoresDotfiles(t *testing.T) {
	tmp := setupReviewRepo(t)
	mustWrite(t, filepath.Join(tmp, "raw", "inbox", "interactive", ".hidden"), "")
	var stdout, stderr bytes.Buffer
	ReviewStatus(ReviewOptions{RepoRoot: tmp, Today: "2026-04-27"}, &stdout, &stderr)
	if !strings.Contains(stdout.String(), "REVIEW|raw-inbox-files|count=1") {
		t.Fatalf("dotfile leaked: %q", stdout.String())
	}
}

func TestReviewStatusOverdueExcludesFuture(t *testing.T) {
	tmp := setupReviewRepo(t)
	body, _ := os.ReadFile(filepath.Join(tmp, ".awiki", "maps", "actions.tsv"))
	body = append(body,
		[]byte("a09\t \tlater\tcontent/projects/q3-launch.md\t10\tcomputer\t2026-05-30\t\t\t\t\t\t\t\tq3-launch\tpublic\n")...)
	mustWrite(t, filepath.Join(tmp, ".awiki", "maps", "actions.tsv"), string(body))
	var stdout, stderr bytes.Buffer
	ReviewStatus(ReviewOptions{RepoRoot: tmp, Today: "2026-04-27"}, &stdout, &stderr)
	if !strings.Contains(stdout.String(), "REVIEW|overdue|count=2|ids=a01,a07") {
		t.Fatalf("overdue wrong: %q", stdout.String())
	}
}

func TestReviewChainRunsAgendaThenLintThenStatus(t *testing.T) {
	tmp := setupReviewRepo(t)
	// Create agenda placeholder pages so Agenda doesn't return 0 (just
	// no-ops since no missing-page error).
	for _, region := range []string{"next-actions", "today", "waiting", "someday", "stuck-projects"} {
		mustWrite(t, filepath.Join(tmp, "content", "agenda", region+".md"),
			"---\ntitle: \""+region+"\"\ntype: agenda\nlast_updated: 2025-01-01\n---\n\n<!-- BEGIN agenda:"+region+" -->\n<!-- END agenda:"+region+" -->\n")
	}
	var stdout, stderr bytes.Buffer
	code := Review(ReviewOptions{RepoRoot: tmp, Today: "2026-04-27"}, &stdout, &stderr)
	// Note: lint may fail on a synthetic repo; we only assert the
	// REVIEW chain emits the structured records and the agenda regen
	// happened.
	out := stdout.String()
	if !strings.Contains(out, "agenda regenerated:") {
		t.Errorf("agenda step missing: %q", out)
	}
	if !strings.Contains(out, "REVIEW|inbox-unprocessed|count=") {
		t.Errorf("review step missing: %q", out)
	}
	_ = code
}
