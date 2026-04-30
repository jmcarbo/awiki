package template

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitProvenance_ByteEqualsPythonOracle(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "template.json")
	if err := InitProvenance(p, "url", "ref", "ver", "abc123"); err != nil {
		t.Fatalf("InitProvenance: %v", err)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := `{
  "schema_version": 1,
  "repo": "url",
  "original_repo": "url",
  "ref": "ref",
  "version": "ver",
  "commit": "abc123",
  "applied_migrations": [],
  "deleted": [],
  "bootstrap_steps_done": []
}
`
	if string(got) != want {
		t.Fatalf("InitProvenance bytes mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestInitProvenance_RealisticFields(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "template.json")
	if err := InitProvenance(p, "https://github.com/jmcarbo/awiki", "main", "1.0.0", "abc123def456"); err != nil {
		t.Fatalf("InitProvenance: %v", err)
	}
	prov, err := LoadProvenance(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if prov.Repo != "https://github.com/jmcarbo/awiki" {
		t.Errorf("repo=%q", prov.Repo)
	}
	if prov.OriginalRepo != prov.Repo {
		t.Errorf("original_repo != repo")
	}
	if prov.Commit != "abc123def456" {
		t.Errorf("commit=%q", prov.Commit)
	}
}

func TestGetProvenanceField_Commit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "template.json")
	_ = InitProvenance(p, "url", "ref", "ver", "abc123")
	got, err := GetProvenanceField(p, "commit")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "abc123" {
		t.Errorf("commit=%q want abc123", got)
	}
}

func TestGetProvenanceField_Unknown(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "template.json")
	_ = InitProvenance(p, "url", "ref", "ver", "abc123")
	_, err := GetProvenanceField(p, "nonexistent")
	if !errors.Is(err, ErrFieldNotFound) {
		t.Errorf("err=%v want ErrFieldNotFound", err)
	}
}

func TestSetProvenanceField_Commit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "template.json")
	_ = InitProvenance(p, "url", "ref", "ver", "abc123")
	if err := SetProvenanceField(p, "commit", "def456"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, _ := GetProvenanceField(p, "commit")
	if got != "def456" {
		t.Errorf("commit=%q want def456", got)
	}
	got, _ = GetProvenanceField(p, "original_repo")
	if got != "url" {
		t.Errorf("original_repo=%q want url (untouched)", got)
	}
}

func TestSetProvenanceField_OriginalRepoImmutable(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "template.json")
	_ = InitProvenance(p, "url", "ref", "ver", "abc123")
	err := SetProvenanceField(p, "original_repo", "other")
	if !errors.Is(err, ErrImmutableField) {
		t.Errorf("err=%v want ErrImmutableField", err)
	}
}

func TestSetProvenanceField_SchemaVersionImmutable(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "template.json")
	_ = InitProvenance(p, "url", "ref", "ver", "abc123")
	err := SetProvenanceField(p, "schema_version", "99")
	if !errors.Is(err, ErrImmutableField) {
		t.Errorf("err=%v want ErrImmutableField", err)
	}
}

func TestAppendProvenanceMigration(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "template.json")
	_ = InitProvenance(p, "url", "ref", "ver", "abc123")
	if err := AppendProvenanceMigration(p, "0001-foo", "applied", ""); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := AppendProvenanceMigration(p, "0002-bar", "skipped", "user --skip-migration"); err != nil {
		t.Fatalf("append: %v", err)
	}
	prov, _ := LoadProvenance(p)
	if len(prov.AppliedMigrations) != 2 {
		t.Fatalf("applied_migrations len=%d want 2", len(prov.AppliedMigrations))
	}
	if prov.AppliedMigrations[1].Reason != "user --skip-migration" {
		t.Errorf("reason=%q", prov.AppliedMigrations[1].Reason)
	}
	// First migration omitted reason; check JSON does not include the key.
	data, _ := os.ReadFile(p)
	if !strings.Contains(string(data), `"0001-foo"`) {
		t.Error("expected 0001-foo in serialized output")
	}
	// The first migration entry has no reason; ensure the entry doesn't
	// contain a "reason" key (omitempty).
	var raw struct {
		AM []map[string]any `json:"applied_migrations"`
	}
	_ = json.Unmarshal(data, &raw)
	if _, ok := raw.AM[0]["reason"]; ok {
		t.Error("first migration should not have reason key (omitempty)")
	}
	if raw.AM[1]["reason"] != "user --skip-migration" {
		t.Errorf("second migration reason=%v", raw.AM[1]["reason"])
	}
}

func TestAppendProvenanceBootstrapStep_WithContentHash(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "template.json")
	_ = InitProvenance(p, "url", "ref", "ver", "abc123")
	if err := AppendProvenanceBootstrapStep(p, "domain", "applied", "", "sha256:deadbeef"); err != nil {
		t.Fatalf("append: %v", err)
	}
	prov, _ := LoadProvenance(p)
	if len(prov.BootstrapStepsDone) != 1 {
		t.Fatalf("len=%d want 1", len(prov.BootstrapStepsDone))
	}
	s := prov.BootstrapStepsDone[0]
	if s.ID != "domain" || s.Status != "applied" || s.ContentHash != "sha256:deadbeef" {
		t.Errorf("step=%+v", s)
	}
}

func TestHasStepApplied_Match(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "template.json")
	_ = InitProvenance(p, "url", "ref", "ver", "abc123")
	_ = AppendProvenanceBootstrapStep(p, "domain", "applied", "", "sha256:deadbeef")
	ok, err := HasStepApplied(p, "domain", "sha256:deadbeef")
	if err != nil || !ok {
		t.Errorf("ok=%v err=%v want ok=true", ok, err)
	}
}

func TestHasStepApplied_HashMismatch(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "template.json")
	_ = InitProvenance(p, "url", "ref", "ver", "abc123")
	_ = AppendProvenanceBootstrapStep(p, "domain", "applied", "", "sha256:deadbeef")
	ok, err := HasStepApplied(p, "domain", "sha256:00ff")
	if err != nil || ok {
		t.Errorf("ok=%v err=%v want ok=false", ok, err)
	}
}

func TestHasStepApplied_StatusSkipped(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "template.json")
	_ = InitProvenance(p, "url", "ref", "ver", "abc123")
	_ = AppendProvenanceBootstrapStep(p, "domain", "skipped", "user", "sha256:deadbeef")
	ok, _ := HasStepApplied(p, "domain", "sha256:deadbeef")
	if ok {
		t.Error("skipped step must not match has-step-applied")
	}
}

func TestListProvenanceSteps(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "template.json")
	_ = InitProvenance(p, "url", "ref", "ver", "abc123")
	_ = AppendProvenanceBootstrapStep(p, "domain", "applied", "", "sha256:deadbeef")
	_ = AppendProvenanceBootstrapStep(p, "stage-commit", "applied", "", "sha256:cafe")
	out, err := ListProvenanceSteps(p)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := "domain applied sha256:deadbeef\nstage-commit applied sha256:cafe\n"
	if out != want {
		t.Errorf("List=%q want %q", out, want)
	}
}

func TestLoadProvenance_MissingFile_ReturnsEmpty(t *testing.T) {
	prov, err := LoadProvenance("/definitely/missing/template.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if prov.SchemaVersion != 0 {
		t.Errorf("expected zero-value provenance")
	}
}

func TestSaveLoadProvenance_RoundTrip_PreservesEmptyArrays(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "template.json")
	_ = InitProvenance(p, "url", "ref", "ver", "abc123")
	// Round-trip: load + save should produce identical bytes.
	want, _ := os.ReadFile(p)
	prov, _ := LoadProvenance(p)
	_ = SaveProvenance(p, prov)
	got, _ := os.ReadFile(p)
	if string(got) != string(want) {
		t.Errorf("round-trip mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
