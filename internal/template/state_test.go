package template

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func pinNow(t *testing.T, ts time.Time) {
	t.Helper()
	prev := Now
	Now = func() time.Time { return ts }
	t.Cleanup(func() { Now = prev })
}

func TestInitState_RequiredFieldsPresent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	if err := InitState(p, "aaa", "bbb", "awiki-template-update/bbb"); err != nil {
		t.Fatalf("InitState: %v", err)
	}
	s, err := LoadState(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Phase != "fetch" {
		t.Errorf("phase=%q want fetch", s.Phase)
	}
	if s.Status != "committed" {
		t.Errorf("status=%q", s.Status)
	}
	if s.CommitOld != "aaa" || s.CommitNew != "bbb" {
		t.Errorf("commits=%s,%s", s.CommitOld, s.CommitNew)
	}
	if s.Branch != "awiki-template-update/bbb" {
		t.Errorf("branch=%q", s.Branch)
	}
	if s.LastCompletedCommit != nil {
		t.Errorf("last_completed_commit=%v want nil", s.LastCompletedCommit)
	}
	if len(s.AppliedMigrationsPending) != 0 {
		t.Errorf("applied_migrations_pending=%v", s.AppliedMigrationsPending)
	}
	if len(s.BootstrapStepsPending) != 0 {
		t.Errorf("bootstrap_steps_pending=%v", s.BootstrapStepsPending)
	}
	if len(s.DeletionsUserDecisions) != 0 {
		t.Errorf("deletions_user_decisions=%v", s.DeletionsUserDecisions)
	}
	if len(s.DeletedPending) != 0 {
		t.Errorf("deleted_pending=%v", s.DeletedPending)
	}
}

func TestInitState_ByteEqualsPythonOracle(t *testing.T) {
	pinNow(t, time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC))
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	_ = InitState(p, "aaa", "bbb", "br")
	got, _ := os.ReadFile(p)
	want := `{
  "phase": "fetch",
  "status": "committed",
  "commit_old": "aaa",
  "commit_new": "bbb",
  "branch": "br",
  "started_at": "2026-04-30T12:00:00+00:00",
  "last_completed_commit": null,
  "applied_migrations_pending": [],
  "bootstrap_steps_pending": [],
  "deletions_user_decisions": {},
  "deleted_pending": []
}
`
	if string(got) != want {
		t.Errorf("InitState bytes mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestValidateState_OkOnFreshInit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	_ = InitState(p, "aaa", "bbb", "br")
	if err := ValidateState(p); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestValidateState_FailsOnMissingFields(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(p, []byte(`{"phase": "fetch"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateState(p)
	if err == nil {
		t.Fatal("expected validate error")
	}
	if !strings.Contains(err.Error(), "missing fields") {
		t.Errorf("err=%v", err)
	}
}

func TestValidateState_FailsOnMissingFile(t *testing.T) {
	err := ValidateState("/definitely/not/here.json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "state file not found") {
		t.Errorf("err=%v", err)
	}
}

func TestValidateState_FailsOnInvalidStatus(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	_ = InitState(p, "aaa", "bbb", "br")
	// Hand-edit the status to invalid.
	data, _ := os.ReadFile(p)
	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	raw["status"] = "bogus"
	out, _ := json.MarshalIndent(raw, "", "  ")
	out = append(out, '\n')
	_ = os.WriteFile(p, out, 0o644)

	err := ValidateState(p)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid status") {
		t.Errorf("err=%v", err)
	}
}

func TestSetStatePhase(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	_ = InitState(p, "aaa", "bbb", "br")
	if err := SetStatePhase(p, "commit-a", "started"); err != nil {
		t.Fatalf("SetStatePhase: %v", err)
	}
	s, _ := LoadState(p)
	if s.Phase != "commit-a" || s.Status != "started" {
		t.Errorf("phase=%q status=%q", s.Phase, s.Status)
	}
}

func TestSetStatePhase_RejectsInvalidStatus(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	_ = InitState(p, "aaa", "bbb", "br")
	if err := SetStatePhase(p, "x", "bogus"); err == nil {
		t.Error("expected reject")
	}
}

func TestSetStateLastCompleted(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	_ = InitState(p, "aaa", "bbb", "br")
	if err := SetStateLastCompleted(p, "deadbeef"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	s, _ := LoadState(p)
	if s.LastCompletedCommit == nil || *s.LastCompletedCommit != "deadbeef" {
		t.Errorf("last_completed_commit=%v", s.LastCompletedCommit)
	}
}

func TestAddMigrationPending(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	_ = InitState(p, "aaa", "bbb", "br")
	if err := AddMigrationPending(p, "0001-foo", "applied", ""); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := AddMigrationPending(p, "0002-bar", "skipped", "user"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	s, _ := LoadState(p)
	if len(s.AppliedMigrationsPending) != 2 {
		t.Fatalf("len=%d", len(s.AppliedMigrationsPending))
	}
	if s.AppliedMigrationsPending[1].Reason != "user" {
		t.Errorf("reason=%q", s.AppliedMigrationsPending[1].Reason)
	}
}

func TestAddBootstrapPending(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	_ = InitState(p, "aaa", "bbb", "br")
	if err := AddBootstrapPending(p, "domain", "applied", "", "sha256:dead"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	s, _ := LoadState(p)
	if len(s.BootstrapStepsPending) != 1 {
		t.Fatalf("len=%d", len(s.BootstrapStepsPending))
	}
	if s.BootstrapStepsPending[0].ContentHash != "sha256:dead" {
		t.Errorf("hash=%q", s.BootstrapStepsPending[0].ContentHash)
	}
}

func TestAddDeletionDecision(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	_ = InitState(p, "aaa", "bbb", "br")
	if err := AddDeletionDecision(p, "scripts/old.sh", "remove"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := AddDeletionDecision(p, "scripts/keep.sh", "preserve-local"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	s, _ := LoadState(p)
	if s.DeletionsUserDecisions["scripts/old.sh"] != "remove" {
		t.Errorf("decision=%q", s.DeletionsUserDecisions["scripts/old.sh"])
	}
	if s.DeletionsUserDecisions["scripts/keep.sh"] != "preserve-local" {
		t.Errorf("decision=%q", s.DeletionsUserDecisions["scripts/keep.sh"])
	}
}

func TestAddDeletedPending(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	_ = InitState(p, "aaa", "bbb", "br")
	if err := AddDeletedPending(p, "scripts/old.sh", "removed-by-user"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	s, _ := LoadState(p)
	if len(s.DeletedPending) != 1 {
		t.Fatalf("len=%d", len(s.DeletedPending))
	}
	if s.DeletedPending[0].Path != "scripts/old.sh" {
		t.Errorf("path=%q", s.DeletedPending[0].Path)
	}
}

func TestGetStateField_LastCompletedNullPrintsNone(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	_ = InitState(p, "aaa", "bbb", "br")
	got, err := GetStateField(p, "last_completed_commit")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	// Python prints "None" for null; our oracle parity matches.
	if got != "None" {
		t.Errorf("got=%q want None", got)
	}
}

func TestGetStateField_PhaseBranch(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	_ = InitState(p, "aaa", "bbb", "br")
	got, _ := GetStateField(p, "phase")
	if got != "fetch" {
		t.Errorf("got=%q", got)
	}
	got, _ = GetStateField(p, "branch")
	if got != "br" {
		t.Errorf("got=%q", got)
	}
}

func TestSaveLoadState_RoundTripPreservesEmpty(t *testing.T) {
	pinNow(t, time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC))
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	_ = InitState(p, "aaa", "bbb", "br")
	want, _ := os.ReadFile(p)
	s, _ := LoadState(p)
	_ = SaveState(p, s)
	got, _ := os.ReadFile(p)
	if string(got) != string(want) {
		t.Errorf("round-trip mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
