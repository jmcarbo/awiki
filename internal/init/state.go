// Package init implements the `awiki init` verb — the deterministic
// fusion of the prose `just init` flow (BOOTSTRAP.md) and the
// `just init-agent` shell delegation. The verb walks the 14 bootstrap
// steps in order, persisting answers + per-step status to
// `.awiki/init-state.json` so re-runs (`--continue`) pick up where
// the prior invocation halted.
//
// state.go covers the on-disk state file: schema, load/save helpers,
// per-step status mutation. Idempotent verbs read this file at start,
// skip steps already marked `applied`, and append/update entries as
// they execute.
package initverb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SchemaVersion is the on-disk schema for .awiki/init-state.json.
// Bump when the struct shape changes incompatibly.
const SchemaVersion = 1

// StatusPending denotes a step that has not been executed yet.
// StatusApplied: ran successfully.
// StatusSkipped: explicitly skipped (--skip-step or step prerequisite
// declined).
// StatusFailed: ran but reported an error; --continue will retry.
const (
	StatusPending = "pending"
	StatusApplied = "applied"
	StatusSkipped = "skipped"
	StatusFailed  = "failed"
)

// Answers captures every interactive answer the verb collects. Fields
// align 1:1 with BOOTSTRAP.md prompts so the YAML config schema
// (--config) maps directly. Empty strings/false bools mean "not yet
// answered" or "default no" depending on context — the step impls own
// the interpretation.
type Answers struct {
	Domain         string `json:"domain,omitempty" yaml:"domain"`
	WikiName       string `json:"wiki_name,omitempty" yaml:"wiki_name"`
	Purpose        string `json:"purpose,omitempty" yaml:"purpose"`
	Privacy        string `json:"privacy,omitempty" yaml:"privacy"`
	TrackProcessed bool   `json:"track_processed" yaml:"track_processed"`
	Theme          string `json:"theme,omitempty" yaml:"theme"`
	PublishLog     bool   `json:"publish_log" yaml:"publish_log"`
	WireQmdMCP     bool   `json:"wire_qmd_mcp" yaml:"wire_qmd_mcp"`
	WireAwikiMCP   bool   `json:"wire_awiki_mcp" yaml:"wire_awiki_mcp"`
	StageCommit    bool   `json:"stage_commit" yaml:"stage_commit"`
}

// StepRecord is one entry under State.Steps.
type StepRecord struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	AppliedAt string `json:"applied_at,omitempty"`
	Note      string `json:"note,omitempty"`
}

// State is the document persisted to .awiki/init-state.json. The
// canonical name is "init-state" (not "init") to keep it disjoint from
// the unrelated `.awiki/template.json` provenance file written in
// Step 12.
type State struct {
	SchemaVersion int          `json:"schema_version"`
	StartedAt     string       `json:"started_at"`
	Agent         string       `json:"agent,omitempty"`
	Answers       Answers      `json:"answers"`
	Steps         []StepRecord `json:"steps"`
}

// StatePath returns the canonical location of the state file under
// repoRoot. Callers must ensure the parent .awiki directory exists
// before save.
func StatePath(repoRoot string) string {
	return filepath.Join(repoRoot, ".awiki", "init-state.json")
}

// LoadState reads the state file. Returns (nil, nil) when the file
// does not exist — caller treats that as "no prior run".
func LoadState(repoRoot string) (*State, error) {
	path := StatePath(repoRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("decode init-state: %w", err)
	}
	if s.SchemaVersion == 0 {
		s.SchemaVersion = SchemaVersion
	}
	return &s, nil
}

// SaveState writes s to .awiki/init-state.json with deterministic
// indent=2 formatting plus a trailing newline (mirrors the convention
// every other awiki state file uses).
func SaveState(repoRoot string, s *State) error {
	path := StatePath(repoRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// ResetState removes the state file. Returns nil when the file is
// already absent — re-running `--reset` is idempotent.
func ResetState(repoRoot string) error {
	path := StatePath(repoRoot)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

// NewState constructs a fresh State with started_at stamped at now.
// agent is recorded verbatim (empty string when no --agent flag).
func NewState(now time.Time, agent string) *State {
	return &State{
		SchemaVersion: SchemaVersion,
		StartedAt:     now.UTC().Format(time.RFC3339),
		Agent:         agent,
		Answers:       Answers{},
		Steps:         []StepRecord{},
	}
}

// FindStep returns a pointer to the StepRecord for id, or nil when
// absent. The pointer is into the underlying slice — callers can
// mutate Status/Note/AppliedAt in place and SaveState afterwards.
func (s *State) FindStep(id string) *StepRecord {
	for i := range s.Steps {
		if s.Steps[i].ID == id {
			return &s.Steps[i]
		}
	}
	return nil
}

// UpsertStep appends a new StepRecord with the given fields, or
// replaces an existing record with the same ID. Returns a pointer to
// the (now stored) record.
func (s *State) UpsertStep(id, status, note string, now time.Time) *StepRecord {
	rec := s.FindStep(id)
	stamp := ""
	if status == StatusApplied || status == StatusSkipped || status == StatusFailed {
		stamp = now.UTC().Format(time.RFC3339)
	}
	if rec == nil {
		s.Steps = append(s.Steps, StepRecord{
			ID:        id,
			Status:    status,
			AppliedAt: stamp,
			Note:      note,
		})
		return &s.Steps[len(s.Steps)-1]
	}
	rec.Status = status
	rec.Note = note
	if stamp != "" {
		rec.AppliedAt = stamp
	}
	return rec
}

// IsApplied returns true when the step has run to completion. Used by
// --continue to skip already-finished work.
func (s *State) IsApplied(id string) bool {
	rec := s.FindStep(id)
	return rec != nil && rec.Status == StatusApplied
}
