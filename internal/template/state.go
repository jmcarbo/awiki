package template

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// StateRequiredFields names the fields a `validate` call asserts are
// present. Mirrors REQUIRED_FIELDS in scripts/_template_helpers/state.py.
var StateRequiredFields = []string{
	"phase", "status", "commit_old", "commit_new", "branch", "started_at",
	"last_completed_commit", "applied_migrations_pending",
	"bootstrap_steps_pending", "deletions_user_decisions", "deleted_pending",
}

// State models the orchestrator state file. Field order is
// load-bearing: it determines the byte layout `state.py save` produces
// for newly-initialized state files.
type State struct {
	Phase                    string                  `json:"phase"`
	Status                   string                  `json:"status"`
	CommitOld                string                  `json:"commit_old"`
	CommitNew                string                  `json:"commit_new"`
	Branch                   string                  `json:"branch"`
	StartedAt                string                  `json:"started_at"`
	LastCompletedCommit      *string                 `json:"last_completed_commit"`
	AppliedMigrationsPending []StatePendingMigration `json:"applied_migrations_pending"`
	BootstrapStepsPending    []StatePendingStep      `json:"bootstrap_steps_pending"`
	DeletionsUserDecisions   map[string]string       `json:"deletions_user_decisions"`
	DeletedPending           []StateDeletedPending   `json:"deleted_pending"`
}

// StatePendingMigration is one entry in applied_migrations_pending.
type StatePendingMigration struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// StatePendingStep is one entry in bootstrap_steps_pending.
type StatePendingStep struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Reason      string `json:"reason,omitempty"`
	ContentHash string `json:"content_hash,omitempty"`
}

// StateDeletedPending is one entry in deleted_pending.
type StateDeletedPending struct {
	Path   string `json:"path"`
	Reason string `json:"reason,omitempty"`
}

// Now is overridable for tests so we can pin the started_at field.
var Now = func() time.Time { return time.Now().UTC() }

// NewState returns a freshly-initialized state record matching
// `state.py init`.
func NewState(commitOld, commitNew, branch string) *State {
	return &State{
		Phase:                    "fetch",
		Status:                   "committed",
		CommitOld:                commitOld,
		CommitNew:                commitNew,
		Branch:                   branch,
		StartedAt:                Now().Format("2006-01-02T15:04:05-07:00"),
		LastCompletedCommit:      nil,
		AppliedMigrationsPending: []StatePendingMigration{},
		BootstrapStepsPending:    []StatePendingStep{},
		DeletionsUserDecisions:   map[string]string{},
		DeletedPending:           []StateDeletedPending{},
	}
}

// LoadState returns the parsed state at path, or an empty State{} when
// the file is missing (mirrors Python `load`).
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &State{}, nil
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("decode state: %w", err)
	}
	if s.AppliedMigrationsPending == nil {
		s.AppliedMigrationsPending = []StatePendingMigration{}
	}
	if s.BootstrapStepsPending == nil {
		s.BootstrapStepsPending = []StatePendingStep{}
	}
	if s.DeletionsUserDecisions == nil {
		s.DeletionsUserDecisions = map[string]string{}
	}
	if s.DeletedPending == nil {
		s.DeletedPending = []StateDeletedPending{}
	}
	return &s, nil
}

// SaveState writes s to path with json.MarshalIndent("", "  ") and a
// trailing newline (matches `state.py save`).
func SaveState(path string, s *State) error {
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

// InitState writes a fresh state file at path.
func InitState(path, commitOld, commitNew, branch string) error {
	return SaveState(path, NewState(commitOld, commitNew, branch))
}

// SetStatePhase mutates phase + status (`state.py set-phase`).
func SetStatePhase(path, phase, status string) error {
	if status != "started" && status != "committed" {
		return fmt.Errorf("invalid status: %s", status)
	}
	s, err := LoadState(path)
	if err != nil {
		return err
	}
	s.Phase = phase
	s.Status = status
	return SaveState(path, s)
}

// SetStateLastCompleted mutates last_completed_commit.
func SetStateLastCompleted(path, sha string) error {
	s, err := LoadState(path)
	if err != nil {
		return err
	}
	s.LastCompletedCommit = &sha
	return SaveState(path, s)
}

// AddMigrationPending appends to applied_migrations_pending.
func AddMigrationPending(path, id, status, reason string) error {
	if status != "applied" && status != "skipped" {
		return fmt.Errorf("invalid status: %s", status)
	}
	s, err := LoadState(path)
	if err != nil {
		return err
	}
	s.AppliedMigrationsPending = append(s.AppliedMigrationsPending, StatePendingMigration{
		ID: id, Status: status, Reason: reason,
	})
	return SaveState(path, s)
}

// AddBootstrapPending appends to bootstrap_steps_pending.
func AddBootstrapPending(path, id, status, reason, contentHash string) error {
	if status != "applied" && status != "skipped" {
		return fmt.Errorf("invalid status: %s", status)
	}
	s, err := LoadState(path)
	if err != nil {
		return err
	}
	s.BootstrapStepsPending = append(s.BootstrapStepsPending, StatePendingStep{
		ID: id, Status: status, Reason: reason, ContentHash: contentHash,
	})
	return SaveState(path, s)
}

// AddDeletionDecision sets deletions_user_decisions[rel] = decision.
func AddDeletionDecision(path, rel, decision string) error {
	if decision != "remove" && decision != "preserve-local" {
		return fmt.Errorf("invalid decision: %s", decision)
	}
	s, err := LoadState(path)
	if err != nil {
		return err
	}
	if s.DeletionsUserDecisions == nil {
		s.DeletionsUserDecisions = map[string]string{}
	}
	s.DeletionsUserDecisions[rel] = decision
	return SaveState(path, s)
}

// AddDeletedPending appends to deleted_pending.
func AddDeletedPending(path, rel, reason string) error {
	s, err := LoadState(path)
	if err != nil {
		return err
	}
	s.DeletedPending = append(s.DeletedPending, StateDeletedPending{
		Path: rel, Reason: reason,
	})
	return SaveState(path, s)
}

// GetStateField returns the printable representation of field, mirroring
// `state.py get`. Lists/maps are JSON-encoded; scalars stringified.
// Returns ErrFieldNotFound if the field is unknown.
func GetStateField(path, field string) (string, error) {
	s, err := LoadState(path)
	if err != nil {
		return "", err
	}
	switch field {
	case "phase":
		return s.Phase, nil
	case "status":
		return s.Status, nil
	case "commit_old":
		return s.CommitOld, nil
	case "commit_new":
		return s.CommitNew, nil
	case "branch":
		return s.Branch, nil
	case "started_at":
		return s.StartedAt, nil
	case "last_completed_commit":
		if s.LastCompletedCommit == nil {
			// Python prints "None" when the JSON value is null.
			return "None", nil
		}
		return *s.LastCompletedCommit, nil
	case "applied_migrations_pending":
		return jsonCompact(s.AppliedMigrationsPending)
	case "bootstrap_steps_pending":
		return jsonCompact(s.BootstrapStepsPending)
	case "deletions_user_decisions":
		return jsonCompact(s.DeletionsUserDecisions)
	case "deleted_pending":
		return jsonCompact(s.DeletedPending)
	default:
		return "", fmt.Errorf("%w: %s", ErrFieldNotFound, field)
	}
}

// ValidateState mirrors `state.py validate`. Returns nil iff every
// required field is present and status is valid.
func ValidateState(path string) error {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("state file not found: %s", path)
		}
		return err
	}
	// Re-decode as a generic map so we can detect missing keys (struct
	// decoding zero-fills, hiding the absence).
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decode state: %w", err)
	}
	missing := []string{}
	for _, k := range StateRequiredFields {
		if _, ok := raw[k]; !ok {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("missing fields: %v", missing)
	}
	var statusVal string
	if err := json.Unmarshal(raw["status"], &statusVal); err == nil {
		if statusVal != "started" && statusVal != "committed" {
			return fmt.Errorf("invalid status: %s", statusVal)
		}
	}
	return nil
}
