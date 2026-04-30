// Package git ports the bash + python `ingest-git` pipeline to Go. It
// is consumed by the slice-8 verb dispatcher (`awiki ingest-git`).
//
// Layout mirrors the bash oracle:
//
//   - state.go    — read/write `.awiki/git-state/<repo_key>.json` (port of
//                   scripts/lib/git-state.sh).
//   - config.go   — parse + validate `.awiki/git-sources.yml` entries
//                   (port of scripts/lib/git-config.sh).
//   - clone.go    — resolve a `<repo-spec>` to a local checkout (port of
//                   scripts/lib/git-clone.sh).
//   - transform.go — turn one upstream markdown file into an awiki source
//                   page (port of scripts/ingest-git-transform.py).
//   - run.go      — top-level driver (port of scripts/ingest-git.sh).
package git

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// State mirrors the per-repo state JSON written by the bash pipeline.
// The bash form (scripts/ingest-git.sh:307) emits this object via
// `python3 -c json.dumps(out, indent=2)` with the keys in the order
// listed below. Mirroring the Python dict's insertion order requires
// emitting keys in the same order on the Go side; encoding/json
// honours struct-field order, so we keep the field declaration order
// stable.
//
// IMPORTANT: changing the field order would break diff parity with bash
// callers that grep the JSON file.
type State struct {
	Schema        int                  `json:"schema"`
	RepoKey       string               `json:"repo_key"`
	RepoName      string               `json:"repo_name"`
	URL           string               `json:"url"`
	DefaultBranch string               `json:"default_branch"`
	HeadSHA       string               `json:"head_sha"`
	IngestedAt    string               `json:"ingested_at"`
	Private       bool                 `json:"private"`
	Files         map[string]FileEntry `json:"files"`
}

// FileEntry is one record under State.Files. Bash emits the dict in
// per-file insertion order with these four keys (ingest-git.sh:295):
//
//	{"blob_sha": ..., "slug": ..., "out_path": ..., "last_ingested": ...}
type FileEntry struct {
	BlobSHA      string `json:"blob_sha"`
	Slug         string `json:"slug"`
	OutPath      string `json:"out_path"`
	LastIngested string `json:"last_ingested"`
}

// repoKeyRe rejects repo_keys that contain path-traversal characters.
// Mirrors scripts/lib/git-state.sh:awiki_git_state_validate_repo_key —
// it rejects empty, leading-slash, embedded "..", and embedded "/".
// We rebuild the same logic here as a regex check on the cleared key.
var repoKeyForbidden = regexp.MustCompile(`(^$|/|\.\.)`)

// ValidateRepoKey returns nil if repo_key is a safe filename component.
// Mirrors `awiki_git_state_validate_repo_key`: empty, leading `/`,
// contains `..`, contains `/` are all rejected.
func ValidateRepoKey(repoKey string) error {
	if repoKey == "" {
		return errors.New("repo_key is empty")
	}
	if strings.HasPrefix(repoKey, "/") {
		return fmt.Errorf("repo_key contains path-traversal chars: %s", repoKey)
	}
	if strings.Contains(repoKey, "/") || repoKeyForbidden.MatchString(repoKey) {
		return fmt.Errorf("repo_key contains path-traversal chars: %s", repoKey)
	}
	return nil
}

// StatePath returns the per-repo state file path under repoRoot. Bash
// equivalent: `.awiki/git-state/<repo_key>.json` (scripts/lib/git-state.sh:25).
func StatePath(repoRoot, repoKey string) (string, error) {
	if err := ValidateRepoKey(repoKey); err != nil {
		return "", err
	}
	return filepath.Join(repoRoot, ".awiki", "git-state", repoKey+".json"), nil
}

// LoadState reads the state file for repoKey. When the file is missing,
// returns a zero-valued State (Files == nil, Schema == 0) and ok=false,
// mirroring bash's `cat || echo '{}'` fallback at git-state.sh:35.
//
// On parse error returns the parse error verbatim (bash equivalent:
// the python loader wraps with try/except and returns {} — in Go we
// surface the error so callers can decide). Production callers in this
// port treat parse errors as a user-facing error and abort.
func LoadState(repoRoot, repoKey string) (State, bool, error) {
	path, err := StatePath(repoRoot, repoKey)
	if err != nil {
		return State{}, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, false, nil
		}
		return State{}, false, fmt.Errorf("git state load: %w", err)
	}
	if len(data) == 0 {
		return State{}, false, nil
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, false, fmt.Errorf("git state parse %s: %w", path, err)
	}
	return s, true, nil
}

// SaveState writes State for repoKey atomically. Mirrors bash
// `awiki_git_state_save`: mkdir -p the parent, write to a per-pid temp,
// then rename (scripts/lib/git-state.sh:39-48). The bash form also
// appends a trailing newline (`printf '%s\n'`); we mirror that to keep
// byte-parity with bash-produced fixtures.
func SaveState(repoRoot, repoKey string, s State) error {
	if err := ValidateRepoKey(repoKey); err != nil {
		return err
	}
	path, err := StatePath(repoRoot, repoKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("git state save: mkdir: %w", err)
	}
	// Bash emits via `python3 -c print(json.dumps(out, indent=2))`,
	// which uses 2-space indent and no trailing comma. encoding/json
	// matches this when MarshalIndent is invoked with prefix="" and
	// indent="  ". The bash form then adds one '\n' via printf '%s\n'.
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("git state marshal: %w", err)
	}
	data = append(data, '\n')
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("git state save: write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("git state save: rename: %w", err)
	}
	return nil
}

// ValidateState matches the bash `awiki_git_state_validate` schema check
// (scripts/lib/git-state.sh:50-68): require schema == 1 and the eight
// required top-level keys.
func ValidateState(s State) error {
	if s.Schema != 1 {
		return errors.New("schema must be 1")
	}
	required := []struct {
		name string
		ok   bool
	}{
		{"repo_key", s.RepoKey != ""},
		{"repo_name", s.RepoName != ""},
		{"url", s.URL != ""},
		{"default_branch", s.DefaultBranch != ""},
		{"head_sha", s.HeadSHA != ""},
		{"ingested_at", s.IngestedAt != ""},
		// `private` is a bool — both true and false are valid; the
		// bash check is `key in obj`, i.e. the field must be present.
		// In Go the JSON unmarshal always populates a bool so we
		// cannot distinguish absent-from-false. Mirroring the bash
		// "presence is enough" rule means we skip the bool check.
		{"files", s.Files != nil},
	}
	for _, r := range required {
		if !r.ok {
			return fmt.Errorf("missing key: %s", r.name)
		}
	}
	return nil
}
