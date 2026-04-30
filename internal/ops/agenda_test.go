package ops

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupAgendaRepo(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, ".awiki", "maps"))
	mustMkdir(t, filepath.Join(tmp, "content", "agenda"))
	mustMkdir(t, filepath.Join(tmp, "content", "projects"))
	mustMkdir(t, filepath.Join(tmp, "content", "contexts"))
	for _, region := range []string{"next-actions", "today", "waiting", "someday", "stuck-projects"} {
		body := "---\ntitle: \"" + region + "\"\ntype: agenda\nlast_updated: 2025-01-01\ndraft: false\n---\n\n" +
			"User notes above the managed region survive regen.\n\n" +
			"<!-- BEGIN agenda:" + region + " -->\n<!-- END agenda:" + region + " -->\n\n" +
			"User notes below the managed region also survive regen.\n"
		mustWrite(t, filepath.Join(tmp, "content", "agenda", region+".md"), body)
	}
	mustWrite(t, filepath.Join(tmp, ".awiki", "maps", "alias-to-slug.tsv"),
		"@phone\tphone\tcontent/contexts/phone.md\tpublic\n"+
			"@computer\tcomputer\tcontent/contexts/computer.md\tpublic\n"+
			"@home\thome\tcontent/contexts/home.md\tpublic\n")
	tsv := "id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n" +
		"a01\t \tcall dentist about crown\tcontent/projects/renovate-kitchen.md\t11\t@phone\t2026-05-01\t\t\t\t\t\t\t\trenovate-kitchen\tpublic\n" +
		"a02\t/\tdraft proposal\tcontent/projects/q3-launch.md\t11\t@computer\t\t\t\t\t\t\t\t\tq3-launch\tpublic\n" +
		"a03\t?\tq3 budget approval\tcontent/projects/q3-launch.md\t12\t\t\t\tbob-smith\t2026-04-22\t\t\t\t\tq3-launch\tpublic\n" +
		"a04\t>\treorganize garage someday\tcontent/projects/q3-launch.md\t13\t@home\t\t\t\t\t\t\t\t\tq3-launch\tpublic\n" +
		"a07\tx\tfile taxes\tcontent/projects/q3-launch.md\t14\t@computer\t2026-04-15\t\t\t\t\t2026-04-13\t\t\tq3-launch\tpublic\n" +
		"a10\t \tsensitive call\tcontent/private/secret-project.md\t11\t@phone\t\t\t\t\t\t\t\t\tsecret-project\tprivate\n"
	mustWrite(t, filepath.Join(tmp, ".awiki", "maps", "actions.tsv"), tsv)
	return tmp
}

func TestAgendaEmitsNextActionsWithContextHeading(t *testing.T) {
	tmp := setupAgendaRepo(t)
	var stdout, stderr bytes.Buffer
	code := Agenda(AgendaOptions{RepoRoot: tmp, Today: "2026-04-30"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "agenda", "next-actions.md"))
	if !strings.Contains(string(body), "### @phone") {
		t.Fatalf("missing context: %q", body)
	}
	if !strings.Contains(string(body), "call dentist about crown") {
		t.Fatalf("missing action: %q", body)
	}
}

func TestAgendaPrivacyFilterHidesPrivate(t *testing.T) {
	tmp := setupAgendaRepo(t)
	var stdout, stderr bytes.Buffer
	Agenda(AgendaOptions{RepoRoot: tmp, Today: "2026-04-30"}, &stdout, &stderr)
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "agenda", "next-actions.md"))
	if strings.Contains(string(body), "sensitive call") {
		t.Fatalf("private action leaked: %q", body)
	}
	if !strings.Contains(string(body), "action(s) hidden") {
		t.Fatalf("missing hidden placeholder: %q", body)
	}
}

func TestAgendaWaitingGroupsByPerson(t *testing.T) {
	tmp := setupAgendaRepo(t)
	var stdout, stderr bytes.Buffer
	Agenda(AgendaOptions{RepoRoot: tmp, Today: "2026-04-30"}, &stdout, &stderr)
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "agenda", "waiting.md"))
	if !strings.Contains(string(body), "bob-smith") {
		t.Fatalf("missing wait person: %q", body)
	}
}

func TestAgendaSomedayGroupsByProject(t *testing.T) {
	tmp := setupAgendaRepo(t)
	var stdout, stderr bytes.Buffer
	Agenda(AgendaOptions{RepoRoot: tmp, Today: "2026-04-30"}, &stdout, &stderr)
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "agenda", "someday.md"))
	if !strings.Contains(string(body), "reorganize garage") {
		t.Fatalf("missing someday action: %q", body)
	}
}

func TestAgendaPreservesUserNotes(t *testing.T) {
	tmp := setupAgendaRepo(t)
	var stdout, stderr bytes.Buffer
	Agenda(AgendaOptions{RepoRoot: tmp, Today: "2026-04-30"}, &stdout, &stderr)
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "agenda", "next-actions.md"))
	if !strings.Contains(string(body), "User notes above the managed region") {
		t.Fatalf("user notes lost: %q", body)
	}
	if !strings.Contains(string(body), "User notes below the managed region") {
		t.Fatalf("user notes (below) lost: %q", body)
	}
}

func TestAgendaRewritesLastUpdated(t *testing.T) {
	tmp := setupAgendaRepo(t)
	var stdout, stderr bytes.Buffer
	Agenda(AgendaOptions{RepoRoot: tmp, Today: "2026-04-30"}, &stdout, &stderr)
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "agenda", "next-actions.md"))
	if !strings.Contains(string(body), "last_updated: 2026-04-30") {
		t.Fatalf("last_updated not rewritten: %q", body)
	}
}

func TestAgendaIdempotent(t *testing.T) {
	tmp := setupAgendaRepo(t)
	var stdout, stderr bytes.Buffer
	Agenda(AgendaOptions{RepoRoot: tmp, Today: "2026-04-30"}, &stdout, &stderr)
	first, _ := os.ReadFile(filepath.Join(tmp, "content", "agenda", "next-actions.md"))
	stdout.Reset()
	stderr.Reset()
	Agenda(AgendaOptions{RepoRoot: tmp, Today: "2026-04-30"}, &stdout, &stderr)
	second, _ := os.ReadFile(filepath.Join(tmp, "content", "agenda", "next-actions.md"))
	if !bytes.Equal(first, second) {
		t.Fatalf("not idempotent\nFIRST:\n%s\n---\nSECOND:\n%s", first, second)
	}
}

func TestAgendaIncludePrivateBlocked(t *testing.T) {
	tmp := setupAgendaRepo(t)
	var stdout, stderr bytes.Buffer
	code := Agenda(AgendaOptions{RepoRoot: tmp, Today: "2026-04-30", IncludePrivate: true}, &stdout, &stderr)
	if code != 5 {
		t.Fatalf("expected 5, got %d", code)
	}
	if !strings.Contains(stderr.String(), "include-private-blocked") {
		t.Fatalf("missing error: %q", stderr.String())
	}
}

func TestAgendaMissingTSV(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content", "agenda"))
	var stdout, stderr bytes.Buffer
	code := Agenda(AgendaOptions{RepoRoot: tmp, Today: "2026-04-30"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "AGENDA|missing|") {
		t.Fatalf("missing record: %q", stderr.String())
	}
}

func TestAgendaNoLeftoverTmpFiles(t *testing.T) {
	tmp := setupAgendaRepo(t)
	var stdout, stderr bytes.Buffer
	Agenda(AgendaOptions{RepoRoot: tmp, Today: "2026-04-30"}, &stdout, &stderr)
	entries, _ := os.ReadDir(filepath.Join(tmp, "content", "agenda"))
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Fatalf("tmp file leaked: %s", e.Name())
		}
	}
}
