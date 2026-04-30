package ops

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- SCAN tests ---

func TestScanEmptyRepo(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content"))
	var stdout, stderr bytes.Buffer
	code := Scan(ScanOptions{RepoRoot: tmp}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "scanned 0 files") {
		t.Fatalf("missing summary: %q", stdout.String())
	}
	tsv := filepath.Join(tmp, ".awiki", "maps", "actions.tsv")
	body, _ := os.ReadFile(tsv)
	if !strings.HasPrefix(string(body), "id\tstatus\ttext\tfile\tline\tcontext") {
		t.Fatalf("missing TSV header: %q", body)
	}
}

func TestScanSimpleAction(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content", "projects"))
	mustWrite(t, filepath.Join(tmp, "content", "projects", "garden.md"),
		"---\ntitle: \"Garden\"\ntype: project\n---\n\n- [ ] water plants @home due:2026-05-01 ^a01\n")
	var stdout, stderr bytes.Buffer
	code := Scan(ScanOptions{RepoRoot: tmp}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	body, _ := os.ReadFile(filepath.Join(tmp, ".awiki", "maps", "actions.tsv"))
	got := string(body)
	if !strings.Contains(got, "a01\t \twater plants") {
		t.Fatalf("missing action row: %q", got)
	}
	if !strings.Contains(got, "garden\tpublic") {
		t.Fatalf("missing project + source_kind: %q", got)
	}
}

func TestScanRejectsCodeBlockAction(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content"))
	mustWrite(t, filepath.Join(tmp, "content", "page.md"),
		"---\ntitle: \"P\"\ntype: entity\n---\n\n```\n- [ ] not a real task\n```\n")
	var stdout, stderr bytes.Buffer
	code := Scan(ScanOptions{RepoRoot: tmp}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	body, _ := os.ReadFile(filepath.Join(tmp, ".awiki", "maps", "actions.tsv"))
	if strings.Contains(string(body), "not a real task") {
		t.Fatalf("code-block action leaked: %q", body)
	}
}

func TestScanRejectsBlockquoteAction(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content"))
	mustWrite(t, filepath.Join(tmp, "content", "page.md"),
		"---\ntitle: \"P\"\ntype: entity\n---\n\n> - [ ] quoted line\n")
	var stdout, stderr bytes.Buffer
	Scan(ScanOptions{RepoRoot: tmp}, &stdout, &stderr)
	body, _ := os.ReadFile(filepath.Join(tmp, ".awiki", "maps", "actions.tsv"))
	if strings.Contains(string(body), "quoted line") {
		t.Fatalf("blockquote action leaked: %q", body)
	}
}

func TestScanSkipsAgendaPages(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content", "agenda"))
	mustWrite(t, filepath.Join(tmp, "content", "agenda", "next-actions.md"),
		"---\ntitle: \"Next\"\ntype: agenda\n---\n\n- [ ] agenda page action ^x01\n")
	var stdout, stderr bytes.Buffer
	Scan(ScanOptions{RepoRoot: tmp}, &stdout, &stderr)
	body, _ := os.ReadFile(filepath.Join(tmp, ".awiki", "maps", "actions.tsv"))
	if strings.Contains(string(body), "agenda page action") {
		t.Fatalf("agenda action leaked: %q", body)
	}
}

func TestScanPrivacyByGitcrypt(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content", "private"))
	mustWrite(t, filepath.Join(tmp, ".gitattributes"),
		"content/private/** filter=git-crypt diff=git-crypt\n")
	mustWrite(t, filepath.Join(tmp, "content", "private", "secret.md"),
		"---\ntitle: \"S\"\ntype: project\n---\n\n- [ ] sensitive call ^p01\n")
	var stdout, stderr bytes.Buffer
	Scan(ScanOptions{RepoRoot: tmp}, &stdout, &stderr)
	body, _ := os.ReadFile(filepath.Join(tmp, ".awiki", "maps", "actions.tsv"))
	if !strings.Contains(string(body), "private\n") {
		t.Fatalf("expected private classification: %q", body)
	}
}

func TestScanDuplicateIDs(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content"))
	mustWrite(t, filepath.Join(tmp, "content", "page.md"),
		"---\ntitle: \"P\"\ntype: entity\n---\n\n- [ ] first ^dupx\n- [ ] second ^dupx\n")
	var stdout, stderr bytes.Buffer
	code := Scan(ScanOptions{RepoRoot: tmp}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "SCAN|dup-id|") {
		t.Fatalf("missing dup-id: %q", stderr.String())
	}
}

func TestScanIdempotent(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content", "projects"))
	mustWrite(t, filepath.Join(tmp, "content", "projects", "garden.md"),
		"---\ntitle: \"Garden\"\ntype: project\n---\n\n- [ ] water plants @home ^a01\n- [/] mulch @home ^a02\n")
	var stdout, stderr bytes.Buffer
	Scan(ScanOptions{RepoRoot: tmp}, &stdout, &stderr)
	first, _ := os.ReadFile(filepath.Join(tmp, ".awiki", "maps", "actions.tsv"))
	stdout.Reset()
	stderr.Reset()
	Scan(ScanOptions{RepoRoot: tmp}, &stdout, &stderr)
	second, _ := os.ReadFile(filepath.Join(tmp, ".awiki", "maps", "actions.tsv"))
	if !bytes.Equal(first, second) {
		t.Fatalf("scan not idempotent:\n%s\n---\n%s", first, second)
	}
}

func TestScanCounters(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content", "projects"))
	mustWrite(t, filepath.Join(tmp, "content", "projects", "garden.md"),
		"---\ntitle: \"Garden\"\ntype: project\n---\n\n- [ ] open ^o01\n- [/] doing ^o02\n- [x] done ^d01\n- [?] waiting ^w01\n- [>] someday ^s01\n")
	var stdout, stderr bytes.Buffer
	Scan(ScanOptions{RepoRoot: tmp}, &stdout, &stderr)
	if !strings.Contains(stdout.String(), "5 actions") {
		t.Fatalf("expected 5 actions: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "2 open") {
		t.Fatalf("expected 2 open: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "1 done") {
		t.Fatalf("expected 1 done: %q", stdout.String())
	}
}

// --- RECUR tests ---

func recurSeed(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content", "projects"))
	mustMkdir(t, filepath.Join(tmp, ".awiki", "maps"))
	page := filepath.Join(tmp, "content", "projects", "garden.md")
	mustWrite(t, page, "---\ntitle: \"Garden\"\ntype: project\nlast_updated: 2026-05-04\n---\n\n## Done\n\n- [x] water plants @home every:1w due:2026-05-04 done:2026-05-04 ^a05\n")
	mustWrite(t, filepath.Join(tmp, ".awiki", "maps", "actions.tsv"),
		"id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n"+
			"a05\tx\twater plants\tcontent/projects/garden.md\t8\thome\t\t\t\t\t1w\t2026-05-04\t\t\tgarden\tpublic\n")
	return tmp
}

func TestRecurEvery1Week(t *testing.T) {
	tmp := recurSeed(t)
	var stdout, stderr bytes.Buffer
	code := Recur(RecurOptions{
		RepoRoot: tmp,
		Page:     "content/projects/garden.md",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "projects", "garden.md"))
	if !strings.Contains(string(body), "- [ ] water plants @home every:1w due:2026-05-11 ^a05~2") {
		t.Fatalf("recur output wrong: %q", body)
	}
}

func TestRecurDryRun(t *testing.T) {
	tmp := recurSeed(t)
	var stdout, stderr bytes.Buffer
	code := Recur(RecurOptions{
		RepoRoot: tmp,
		Page:     "content/projects/garden.md",
		DryRun:   true,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stdout.String(), "+- [ ] water plants @home every:1w due:2026-05-11") {
		t.Fatalf("missing diff line: %q", stdout.String())
	}
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "projects", "garden.md"))
	if strings.Contains(string(body), "[ ] water plants") {
		t.Fatalf("dry-run mutated page: %q", body)
	}
}

func TestRecurEvery1MonthClampJanToFeb(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content", "projects"))
	mustMkdir(t, filepath.Join(tmp, ".awiki", "maps"))
	page := filepath.Join(tmp, "content", "projects", "billing.md")
	mustWrite(t, page,
		"---\ntitle: \"Billing\"\ntype: project\n---\n\n## Done\n\n- [x] file VAT @computer every:1m due:2026-01-31 done:2026-01-31 ^v01\n")
	mustWrite(t, filepath.Join(tmp, ".awiki", "maps", "actions.tsv"),
		"id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n"+
			"v01\tx\tfile VAT\tcontent/projects/billing.md\t6\tcomputer\t\t\t\t\t1m\t2026-01-31\t\t\tbilling\tpublic\n")
	var stdout, stderr bytes.Buffer
	code := Recur(RecurOptions{RepoRoot: tmp, Page: "content/projects/billing.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	body, _ := os.ReadFile(page)
	if !strings.Contains(string(body), "due:2026-02-28 ^v01~2") {
		t.Fatalf("month clamp wrong: %q", body)
	}
}

func TestRecurEvery3Days(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content", "projects"))
	mustMkdir(t, filepath.Join(tmp, ".awiki", "maps"))
	page := filepath.Join(tmp, "content", "projects", "study.md")
	mustWrite(t, page,
		"---\ntitle: \"Study\"\ntype: project\n---\n\n## Done\n\n- [x] review flashcards @computer every:3d due:2026-04-27 done:2026-04-27 ^s01\n")
	mustWrite(t, filepath.Join(tmp, ".awiki", "maps", "actions.tsv"),
		"id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n"+
			"s01\tx\treview flashcards\tcontent/projects/study.md\t6\tcomputer\t\t\t\t\t3d\t2026-04-27\t\t\tstudy\tpublic\n")
	var stdout, stderr bytes.Buffer
	code := Recur(RecurOptions{RepoRoot: tmp, Page: "content/projects/study.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	body, _ := os.ReadFile(page)
	if !strings.Contains(string(body), "due:2026-04-30 ^s01~2") {
		t.Fatalf("3d compute wrong: %q", body)
	}
}

func TestRecurIdempotent(t *testing.T) {
	tmp := recurSeed(t)
	var stdout, stderr bytes.Buffer
	Recur(RecurOptions{RepoRoot: tmp, Page: "content/projects/garden.md"}, &stdout, &stderr)
	body1, _ := os.ReadFile(filepath.Join(tmp, "content", "projects", "garden.md"))
	stdout.Reset()
	stderr.Reset()
	Recur(RecurOptions{RepoRoot: tmp, Page: "content/projects/garden.md"}, &stdout, &stderr)
	body2, _ := os.ReadFile(filepath.Join(tmp, "content", "projects", "garden.md"))
	openCount1 := strings.Count(string(body1), "- [ ] water plants")
	openCount2 := strings.Count(string(body2), "- [ ] water plants")
	if openCount1 != 1 || openCount2 != 1 {
		t.Fatalf("not idempotent: first=%d second=%d", openCount1, openCount2)
	}
}

func TestRecurChainCap(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content", "projects"))
	mustMkdir(t, filepath.Join(tmp, ".awiki", "maps"))
	page := filepath.Join(tmp, "content", "projects", "cap.md")
	var b strings.Builder
	b.WriteString("---\ntitle: \"Cap\"\ntype: project\n---\n\n## Done\n\n")
	b.WriteString("- [x] tick @computer every:1d due:2026-04-27 done:2026-04-27 ^h01\n")
	for n := 2; n < 200; n++ {
		b.WriteString("- [ ] tick @computer every:1d due:2026-04-27 ^h01~")
		b.WriteString(itoa(n))
		b.WriteString("\n")
	}
	mustWrite(t, page, b.String())
	var stdout, stderr bytes.Buffer
	code := Recur(RecurOptions{RepoRoot: tmp, Page: "content/projects/cap.md"}, &stdout, &stderr)
	if code != 6 {
		t.Fatalf("expected 6, got %d (stderr=%q)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "refusing to emit") {
		t.Fatalf("missing cap message: %q", stderr.String())
	}
}

func TestRecurAlternateSep(t *testing.T) {
	t.Setenv("AWIKI_RECUR_SEP", "__")
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content", "projects"))
	mustMkdir(t, filepath.Join(tmp, ".awiki", "maps"))
	page := filepath.Join(tmp, "content", "projects", "sep.md")
	mustWrite(t, page,
		"---\ntitle: \"Sep\"\ntype: project\n---\n\n## Done\n\n- [x] water plants @home every:1w due:2026-05-04 done:2026-05-04 ^a05\n")
	mustWrite(t, filepath.Join(tmp, ".awiki", "maps", "actions.tsv"),
		"id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n"+
			"a05\tx\twater plants\tcontent/projects/sep.md\t6\thome\t\t\t\t\t1w\t2026-05-04\t\t\tsep\tpublic\n")
	var stdout, stderr bytes.Buffer
	code := Recur(RecurOptions{RepoRoot: tmp, Page: "content/projects/sep.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	body, _ := os.ReadFile(page)
	if !strings.Contains(string(body), "^a05__2") {
		t.Fatalf("missing __ separator: %q", body)
	}
	if strings.Contains(string(body), "^a05~2") {
		t.Fatalf("legacy ~ separator leaked: %q", body)
	}
}

func TestRecurCLIRejectsUnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RecurCLI(false, []string{"--bogus", "x"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown flag") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestRecurCLIHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RecurCLI(false, []string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestComputeRecurDueDailyWeeklyMonthly(t *testing.T) {
	cases := []struct {
		done, every, want string
	}{
		{"2026-04-27", "daily", "2026-04-28"},
		{"2026-04-27", "weekly", "2026-05-04"},
		{"2026-04-27", "monthly", "2026-05-27"},
		{"2024-01-31", "1m", "2024-02-29"},
	}
	for _, tc := range cases {
		got, ok := computeRecurDue(tc.done, tc.every)
		if !ok {
			t.Errorf("computeRecurDue(%q, %q) = !ok", tc.done, tc.every)
			continue
		}
		if got != tc.want {
			t.Errorf("computeRecurDue(%q, %q) = %q want %q", tc.done, tc.every, got, tc.want)
		}
	}
}

func itoa(n int) string {
	return time.Time{}.Add(0).Format("") + intToString(n)
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
