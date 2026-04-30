package template

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// SchemaVersionProvenance is the schema version stamped into newly
// initialized template.json files. Mirrors SCHEMA_VERSION in
// scripts/_template_helpers/provenance.py.
const SchemaVersionProvenance = 1

// ProvenanceImmutableFields is the set of fields the `set` verb
// refuses to mutate.
var ProvenanceImmutableFields = map[string]bool{
	"original_repo":  true,
	"schema_version": true,
}

// Provenance models template.json. Field declaration order is
// load-bearing: json.Marshal preserves struct field order, which lets
// us match `json.dumps(obj, indent=2)` output byte-for-byte for
// freshly initialized files.
type Provenance struct {
	SchemaVersion       int                  `json:"schema_version"`
	Repo                string               `json:"repo"`
	OriginalRepo        string               `json:"original_repo"`
	Ref                 string               `json:"ref"`
	Version             string               `json:"version"`
	Commit              string               `json:"commit"`
	AppliedMigrations   []ProvenanceMigration `json:"applied_migrations"`
	Deleted             []ProvenanceDeleted  `json:"deleted"`
	BootstrapStepsDone  []ProvenanceStep     `json:"bootstrap_steps_done"`
}

// ProvenanceMigration is one entry under applied_migrations.
type ProvenanceMigration struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// ProvenanceDeleted is one entry under deleted.
type ProvenanceDeleted struct {
	Path   string `json:"path"`
	Reason string `json:"reason,omitempty"`
}

// ProvenanceStep is one entry under bootstrap_steps_done.
type ProvenanceStep struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Reason      string `json:"reason,omitempty"`
	ContentHash string `json:"content_hash,omitempty"`
}

// NewProvenance returns a provenance record initialized with the same
// shape as `provenance.py init`.
func NewProvenance(repo, ref, version, commit string) *Provenance {
	return &Provenance{
		SchemaVersion:      SchemaVersionProvenance,
		Repo:               repo,
		OriginalRepo:       repo,
		Ref:                ref,
		Version:            version,
		Commit:             commit,
		AppliedMigrations:  []ProvenanceMigration{},
		Deleted:            []ProvenanceDeleted{},
		BootstrapStepsDone: []ProvenanceStep{},
	}
}

// LoadProvenance reads and decodes template.json. Returns an empty
// Provenance{} when the file is missing (mirrors Python `load`).
func LoadProvenance(path string) (*Provenance, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Provenance{}, nil
		}
		return nil, err
	}
	var p Provenance
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decode provenance: %w", err)
	}
	// Empty arrays must round-trip as []. Python writes `"foo": []`
	// for empty lists; Go would emit `"foo": null` for a nil slice.
	if p.AppliedMigrations == nil {
		p.AppliedMigrations = []ProvenanceMigration{}
	}
	if p.Deleted == nil {
		p.Deleted = []ProvenanceDeleted{}
	}
	if p.BootstrapStepsDone == nil {
		p.BootstrapStepsDone = []ProvenanceStep{}
	}
	return &p, nil
}

// SaveProvenance writes p to path with the same byte-for-byte format
// `provenance.py save` produces: indent=2, trailing newline.
func SaveProvenance(path string, p *Provenance) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// InitProvenance writes a fresh template.json at path. Mirrors the
// `init` subcommand.
func InitProvenance(path, repo, ref, version, commit string) error {
	return SaveProvenance(path, NewProvenance(repo, ref, version, commit))
}

// GetProvenanceField returns the printable representation of field.
// Mirrors `cmd_get`: list/dict values are JSON-encoded, scalars are
// stringified. Returns ErrFieldNotFound when the field is unknown.
var ErrFieldNotFound = errors.New("field not found")

// GetProvenanceField returns the canonical string output of `provenance.py
// get <path> <field>`. The output matches the oracle stdout (no
// trailing newline).
func GetProvenanceField(path, field string) (string, error) {
	p, err := LoadProvenance(path)
	if err != nil {
		return "", err
	}
	switch field {
	case "schema_version":
		return fmt.Sprintf("%d", p.SchemaVersion), nil
	case "repo":
		return p.Repo, nil
	case "original_repo":
		return p.OriginalRepo, nil
	case "ref":
		return p.Ref, nil
	case "version":
		return p.Version, nil
	case "commit":
		return p.Commit, nil
	case "applied_migrations":
		return jsonCompact(p.AppliedMigrations)
	case "deleted":
		return jsonCompact(p.Deleted)
	case "bootstrap_steps_done":
		return jsonCompact(p.BootstrapStepsDone)
	default:
		return "", fmt.Errorf("%w: %s", ErrFieldNotFound, field)
	}
}

// SetProvenanceField mutates a single (mutable) field on the file at
// path, persisting the result. Mirrors `cmd_set`. Refuses immutable
// fields with ErrImmutableField.
var ErrImmutableField = errors.New("field is immutable")

// SetProvenanceField writes the new value, then saves. Only string
// fields are settable from this entry point because the bash callers
// only ever set commit/ref/version/repo.
func SetProvenanceField(path, field, value string) error {
	if ProvenanceImmutableFields[field] {
		return fmt.Errorf("%w: %s", ErrImmutableField, field)
	}
	p, err := LoadProvenance(path)
	if err != nil {
		return err
	}
	switch field {
	case "repo":
		p.Repo = value
	case "ref":
		p.Ref = value
	case "version":
		p.Version = value
	case "commit":
		p.Commit = value
	default:
		// Mirror Python: `d[field] = value` would happily set unknown
		// fields. None of the callers do that today, so we reject
		// instead of silently dropping the mutation.
		return fmt.Errorf("unsupported field for set: %s", field)
	}
	return SaveProvenance(path, p)
}

// AppendProvenanceMigration appends one entry to applied_migrations.
func AppendProvenanceMigration(path, id, status, reason string) error {
	p, err := LoadProvenance(path)
	if err != nil {
		return err
	}
	p.AppliedMigrations = append(p.AppliedMigrations, ProvenanceMigration{
		ID: id, Status: status, Reason: reason,
	})
	return SaveProvenance(path, p)
}

// AppendProvenanceBootstrapStep appends one entry to bootstrap_steps_done.
func AppendProvenanceBootstrapStep(path, id, status, reason, contentHash string) error {
	p, err := LoadProvenance(path)
	if err != nil {
		return err
	}
	p.BootstrapStepsDone = append(p.BootstrapStepsDone, ProvenanceStep{
		ID: id, Status: status, Reason: reason, ContentHash: contentHash,
	})
	return SaveProvenance(path, p)
}

// HasStepApplied returns true iff the step with id has status="applied"
// and content_hash equals expectedHash.
func HasStepApplied(path, id, expectedHash string) (bool, error) {
	p, err := LoadProvenance(path)
	if err != nil {
		return false, err
	}
	for _, s := range p.BootstrapStepsDone {
		if s.ID == id && s.Status == "applied" && s.ContentHash == expectedHash {
			return true, nil
		}
	}
	return false, nil
}

// ListProvenanceSteps returns one "id status content_hash" line per
// completed step (matches `cmd_list_steps`). Each line ends in \n.
func ListProvenanceSteps(path string) (string, error) {
	p, err := LoadProvenance(path)
	if err != nil {
		return "", err
	}
	var out []byte
	for _, s := range p.BootstrapStepsDone {
		out = append(out, []byte(fmt.Sprintf("%s %s %s\n", s.ID, s.Status, s.ContentHash))...)
	}
	return string(out), nil
}

func jsonCompact(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
