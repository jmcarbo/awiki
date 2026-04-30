package template

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RetrofitInput bundles the args `template-retrofit.sh` accepts.
type RetrofitInput struct {
	RepoRoot        string
	Repo            string
	Ref             string
	Version         string
	Commit          string
	NonInteractive  bool
	HeuristicsOnly  bool
}

// RetrofitResult captures the output of a retrofit run for callers /
// test assertions.
type RetrofitResult struct {
	// AlreadyHasProvenance is true when .awiki/template.json was
	// already present (no-op path).
	AlreadyHasProvenance bool
	// HeuristicHits lists the bootstrap-step IDs (and "privacy")
	// detected as already-completed.
	HeuristicHits []string
	// HeuristicLines is the human-readable detection summary, ready
	// to print line-by-line.
	HeuristicLines []string
	// InitRan is true when InitTemplate was actually invoked.
	InitRan bool
}

// Retrofit mirrors scripts/template-retrofit.sh. The driver:
//  1. Bails (info-level) if .awiki/template.json already exists.
//  2. Detects heuristic completion of bootstrap steps from on-disk
//     artefacts (.awiki/qmd-status, .gitattributes, secrets/age-key.txt,
//     .mcp.json, .cursor/mcp.json).
//  3. If --heuristics-only, returns without seeding.
//  4. Otherwise calls InitTemplate (the bash equivalent of running
//     template-init.sh).
//
// Interactive correction of step status is left to callers — non-
// interactive callers are unaffected; interactive callers can call
// MarkStepSkipped per-step.
func Retrofit(arch InitArchiver, in RetrofitInput) (*RetrofitResult, error) {
	res := &RetrofitResult{}
	pj := filepath.Join(in.RepoRoot, ".awiki", "template.json")
	if isFile(pj) {
		res.AlreadyHasProvenance = true
		return res, nil
	}

	res.HeuristicLines = append(res.HeuristicLines, "Detecting already-completed bootstrap steps:")
	hits := []string{}

	if isFile(filepath.Join(in.RepoRoot, ".awiki", "qmd-status")) {
		res.HeuristicLines = append(res.HeuristicLines, "  - install-qmd: yes (.awiki/qmd-status exists)")
		hits = append(hits, "install-qmd")
	}
	if gitAttrs := filepath.Join(in.RepoRoot, ".gitattributes"); isFile(gitAttrs) {
		data, err := os.ReadFile(gitAttrs)
		if err == nil && strings.Contains(string(data), "filter=git-crypt") {
			res.HeuristicLines = append(res.HeuristicLines, "  - privacy=git-crypt: yes (.gitattributes references git-crypt)")
			hits = append(hits, "privacy")
		}
	}
	if isFile(filepath.Join(in.RepoRoot, "secrets", "age-key.txt")) {
		res.HeuristicLines = append(res.HeuristicLines, "  - privacy=age: yes (secrets/age-key.txt exists)")
		hits = append(hits, "privacy")
	}
	if mcpDetected(in.RepoRoot) {
		res.HeuristicLines = append(res.HeuristicLines, "  - wire-awiki-mcp: yes (mcp config references awiki-server)")
		hits = append(hits, "wire-awiki-mcp")
	}
	if len(hits) == 0 {
		res.HeuristicLines = append(res.HeuristicLines, "  (none detected)")
	}
	res.HeuristicHits = hits

	if in.HeuristicsOnly {
		return res, nil
	}
	if in.Repo == "" || in.Version == "" || in.Commit == "" {
		return res, errors.New("retrofit: --repo, --version, --commit all required (unless --heuristics-only)")
	}

	if _, err := InitTemplate(arch, InitInput{
		RepoRoot: in.RepoRoot,
		Repo:     in.Repo,
		Ref:      in.Ref,
		Version:  in.Version,
		Commit:   in.Commit,
	}); err != nil {
		return res, err
	}
	res.InitRan = true
	return res, nil
}

// mcpDetected returns true iff .mcp.json or .cursor/mcp.json contain
// the substring "awiki-server".
func mcpDetected(repoRoot string) bool {
	for _, p := range []string{
		filepath.Join(repoRoot, ".mcp.json"),
		filepath.Join(repoRoot, ".cursor", "mcp.json"),
	} {
		data, err := os.ReadFile(p)
		if err == nil && strings.Contains(string(data), "awiki-server") {
			return true
		}
	}
	return false
}

// MarkStepSkipped sets bootstrap_steps_done[id].status = "skipped" with
// a fixed reason ("retrofit: user said no"). Mirrors the inline Python
// heredoc in the bash retrofit interactive flow.
func MarkStepSkipped(pj, id string) error {
	data, err := os.ReadFile(pj)
	if err != nil {
		return err
	}
	var d map[string]any
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}
	steps, _ := d["bootstrap_steps_done"].([]any)
	for i, sRaw := range steps {
		s, ok := sRaw.(map[string]any)
		if !ok {
			continue
		}
		if v, _ := s["id"].(string); v == id {
			s["status"] = "skipped"
			s["reason"] = "retrofit: user said no"
			steps[i] = s
			break
		}
	}
	d["bootstrap_steps_done"] = steps
	out, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(pj, out, 0o644)
}

// MergeFiles is the Go entry point for `awiki template merge`. It
// shells `git merge-file --diff3 -L current -L base -L new <cur> <base>
// <new>` via the supplied SyncGit adapter. Returns the merge-file exit
// code.
func MergeFiles(g SyncGit, cur, base, newFile string) (int, error) {
	if g == nil {
		return 0, errors.New("merge: git adapter is nil")
	}
	if !isFile(base) {
		return 0, fmt.Errorf("base file not found: %s", base)
	}
	if !isFile(cur) {
		return 0, fmt.Errorf("current file not found: %s", cur)
	}
	if !isFile(newFile) {
		return 0, fmt.Errorf("new file not found: %s", newFile)
	}
	return g.MergeFile(cur, base, newFile)
}
