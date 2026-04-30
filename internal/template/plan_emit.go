package template

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PlanEmitGit captures the git operations plan emission needs. The bash
// + Python oracle invokes `git merge-file --diff3 -p <cur> <base>
// <new>` to test for predicted conflicts in three_way / attributes_merge
// plan rows. The merge runs against scratch copies, so we do NOT need
// in-place merging here — only the conflict count.
type PlanEmitGit interface {
	// MergeFileDryRun runs `git merge-file --diff3 -p <cur> <base>
	// <new>` against scratch copies of the three inputs. It returns
	// the merge-file exit code: 0 = clean, >0 = conflict count. A
	// non-nil err signals the binary is missing entirely.
	MergeFileDryRun(curBytes, baseBytes, newBytes []byte) (code int, err error)
}

// PlanEmitInput bundles the inputs for EmitPlan.
type PlanEmitInput struct {
	OldTree    string
	NewTree    string
	UserTree   string
	Manifest   *Manifest
	CommitOld  string
	CommitNew  string
	Provenance string // .awiki/template.json (optional)
}

// PlanLine is one PLAN|... record. Cols are field values; the wire
// format prepends "PLAN|" and joins with "|" after escaping each col.
type PlanLine struct {
	Cols []string
}

func (p PlanLine) Format() string {
	parts := make([]string, len(p.Cols))
	for i, c := range p.Cols {
		parts[i] = EscapePlanField(c)
	}
	return "PLAN|" + strings.Join(parts, "|")
}

// PlanCounts mirrors the footer counters Python tracks.
type PlanCounts struct {
	Errors        int
	Warnings      int
	Prompts       int
	Conflicts     int
	NewMigrations int
}

// EmitPlan runs the full plan emission, returning the ordered list of
// PLAN| records (header → categories → migrations → user-deleted →
// bootstrap-steps → footer) and the cumulative counts.
func EmitPlan(g PlanEmitGit, in PlanEmitInput) ([]PlanLine, PlanCounts, error) {
	if in.Manifest == nil {
		return nil, PlanCounts{}, errors.New("plan_emit: manifest is nil")
	}
	var lines []PlanLine
	counts := PlanCounts{}

	schema := in.Manifest.SchemaVersion
	if schema == 0 {
		schema = 1
	}
	lines = append(lines, PlanLine{Cols: []string{"header", fmt.Sprintf("%d", schema), in.CommitOld, in.CommitNew}})

	catLines, err := emitCategories(g, in, &counts)
	if err != nil {
		return nil, counts, err
	}
	lines = append(lines, catLines...)

	migLines, newMigs, err := emitMigrations(in)
	if err != nil {
		return nil, counts, err
	}
	counts.NewMigrations = newMigs
	lines = append(lines, migLines...)

	udLines, err := emitUserDeleted(in)
	if err != nil {
		return nil, counts, err
	}
	lines = append(lines, udLines...)

	bsLines, err := emitBootstrapSteps(in)
	if err != nil {
		return nil, counts, err
	}
	lines = append(lines, bsLines...)

	lines = append(lines, PlanLine{Cols: []string{
		"footer",
		fmt.Sprintf("errors=%d", counts.Errors),
		fmt.Sprintf("warnings=%d", counts.Warnings),
		fmt.Sprintf("prompts=%d", counts.Prompts),
		fmt.Sprintf("conflicts=%d", counts.Conflicts),
	}})
	return lines, counts, nil
}

// listFilesUnderTree mirrors Python `list_files`: every relative file
// path under root, excluding anything inside `.git/` or named `.git`.
// Returns a set-style sorted list for determinism.
func listFilesUnderTree(root string) (map[string]bool, []string, error) {
	out := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == ".git" || strings.HasPrefix(rel, ".git/") {
			return nil
		}
		out[rel] = true
		return nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]bool{}, nil, nil
		}
		return nil, nil, err
	}
	sorted := make([]string, 0, len(out))
	for k := range out {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	return out, sorted, nil
}

// fileSHA256 mirrors Python file_sha256.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// lineCount mirrors Python line_count: number of newline-separated
// chunks (trailing empty-line semantics match `for _ in f`).
func lineCount(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, nil
	}
	count := 0
	for _, b := range data {
		if b == '\n' {
			count++
		}
	}
	if data[len(data)-1] != '\n' {
		count++
	}
	return count, nil
}

func filesEqual(a, b string) (bool, error) {
	ai, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	if ai.Size() != bi.Size() {
		return false, nil
	}
	ad, err := os.ReadFile(a)
	if err != nil {
		return false, err
	}
	bd, err := os.ReadFile(b)
	if err != nil {
		return false, err
	}
	if len(ad) != len(bd) {
		return false, nil
	}
	for i := range ad {
		if ad[i] != bd[i] {
			return false, nil
		}
	}
	return true, nil
}

func emitCategories(g PlanEmitGit, in PlanEmitInput, counts *PlanCounts) ([]PlanLine, error) {
	oldSet, _, err := listFilesUnderTree(in.OldTree)
	if err != nil {
		return nil, err
	}
	newSet, _, err := listFilesUnderTree(in.NewTree)
	if err != nil {
		return nil, err
	}
	userSet, _, err := listFilesUnderTree(in.UserTree)
	if err != nil {
		return nil, err
	}

	// changed_or_new = (new - old) | {p in old∩new where bytes differ}
	pathSet := map[string]bool{}
	for p := range newSet {
		if !oldSet[p] {
			pathSet[p] = true
		} else {
			oldP := filepath.Join(in.OldTree, p)
			newP := filepath.Join(in.NewTree, p)
			if isFile(oldP) && isFile(newP) {
				eq, err := filesEqual(oldP, newP)
				if err != nil {
					return nil, err
				}
				if !eq {
					pathSet[p] = true
				}
			}
		}
	}
	deletedInTemplate := map[string]bool{}
	for p := range oldSet {
		if !newSet[p] {
			pathSet[p] = true
			deletedInTemplate[p] = true
		}
	}
	reallyNew := map[string]bool{}
	for p := range newSet {
		if !oldSet[p] {
			reallyNew[p] = true
		}
	}

	paths := make([]string, 0, len(pathSet))
	for p := range pathSet {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var lines []PlanLine
	for _, rel := range paths {
		strategy := in.Manifest.ResolveStrategy(rel)

		if deletedInTemplate[rel] {
			locallyModified := "false"
			if userSet[rel] {
				oldP := filepath.Join(in.OldTree, rel)
				userP := filepath.Join(in.UserTree, rel)
				if !isFile(oldP) || !isFile(userP) {
					locallyModified = "true"
				} else {
					eq, err := filesEqual(oldP, userP)
					if err != nil {
						return nil, err
					}
					if !eq {
						locallyModified = "true"
					}
				}
			}
			lines = append(lines, PlanLine{Cols: []string{"deletion-in-template", rel, locallyModified}})
			continue
		}

		switch strategy {
		case "overwrite":
			lines = append(lines, PlanLine{Cols: []string{"overwrite", rel, ""}})
		case "preserve":
			lines = append(lines, PlanLine{Cols: []string{"preserve", rel, ""}})
		case "template_only":
			lines = append(lines, PlanLine{Cols: []string{"template_only", rel, ""}})
		case "three_way", "attributes_merge":
			status, conflicts, err := testMerge(g, in.OldTree, in.NewTree, in.UserTree, rel)
			if err != nil {
				return nil, err
			}
			lines = append(lines, PlanLine{Cols: []string{strategy, rel, status, fmt.Sprintf("%d", conflicts)}})
			if status == "conflict-predicted" {
				counts.Conflicts += conflicts
			}
		case "prompt":
			if reallyNew[rel] {
				lines = append(lines, PlanLine{Cols: []string{"new_file", rel, "prompt"}})
				counts.Prompts++
			} else {
				lines = append(lines, PlanLine{Cols: []string{"overwrite", rel, "fallback"}})
			}
		default:
			lines = append(lines, PlanLine{Cols: []string{"template_only", rel, fmt.Sprintf("unknown-strategy:%s", strategy)}})
			counts.Warnings++
		}
	}
	return lines, nil
}

func testMerge(g PlanEmitGit, oldTree, newTree, userTree, rel string) (status string, conflicts int, err error) {
	user := filepath.Join(userTree, rel)
	if _, err := os.Stat(user); err != nil {
		// Mirror Python: not exists treated as clean (no merge needed).
		if errors.Is(err, os.ErrNotExist) {
			return "clean", 0, nil
		}
		return "", 0, err
	}
	base := filepath.Join(oldTree, rel)
	if _, err := os.Stat(base); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "clean", 0, nil
		}
		return "", 0, err
	}
	if g == nil {
		return "clean", 0, nil
	}
	curBytes, err := os.ReadFile(user)
	if err != nil {
		return "", 0, err
	}
	baseBytes, err := os.ReadFile(base)
	if err != nil {
		return "", 0, err
	}
	newBytes, err := os.ReadFile(filepath.Join(newTree, rel))
	if err != nil {
		return "", 0, err
	}
	code, err := g.MergeFileDryRun(curBytes, baseBytes, newBytes)
	if err != nil {
		return "", 0, err
	}
	if code == 0 {
		return "clean", 0, nil
	}
	return "conflict-predicted", code, nil
}

func emitMigrations(in PlanEmitInput) ([]PlanLine, int, error) {
	newMigDir := filepath.Join(in.NewTree, "migrations")
	oldMigDir := filepath.Join(in.OldTree, "migrations")
	if !isDir(newMigDir) {
		return nil, 0, nil
	}
	newEntries, err := os.ReadDir(newMigDir)
	if err != nil {
		return nil, 0, err
	}
	var newNames []string
	for _, e := range newEntries {
		if !e.IsDir() {
			newNames = append(newNames, e.Name())
		}
	}
	sort.Strings(newNames)
	old := map[string]bool{}
	if isDir(oldMigDir) {
		oldEntries, err := os.ReadDir(oldMigDir)
		if err != nil {
			return nil, 0, err
		}
		for _, e := range oldEntries {
			if !e.IsDir() {
				old[e.Name()] = true
			}
		}
	}
	added := []string{}
	for _, n := range newNames {
		if !old[n] {
			added = append(added, n)
		}
	}
	var lines []PlanLine
	count := 0
	for _, fname := range added {
		if fname == "README.md" || fname == ".gitkeep" {
			continue
		}
		full := filepath.Join(newMigDir, fname)
		if strings.HasSuffix(fname, ".sh") {
			mid := strings.TrimSuffix(fname, ".sh")
			count++
			lines, err = appendShMigrationLines(lines, full, fname, mid)
			if err != nil {
				return nil, 0, err
			}
		} else if strings.HasSuffix(fname, ".prompt.md") {
			mid := strings.TrimSuffix(fname, ".prompt.md")
			count++
			pl, err := buildPromptMigrationLine(in, full, fname, mid)
			if err != nil {
				return nil, 0, err
			}
			lines = append(lines, pl)
		}
	}
	return lines, count, nil
}

func appendShMigrationLines(lines []PlanLine, full, fname, mid string) ([]PlanLine, error) {
	lc, err := lineCount(full)
	if err != nil {
		// Python treats `wc -l` failure as "0"; mirror that.
		lc = 0
	}
	lines = append(lines, PlanLine{Cols: []string{"migration", mid, "migrations/" + fname, fmt.Sprintf("%d-lines", lc)}})
	sha, err := fileSHA256(full)
	if err != nil {
		return nil, err
	}
	lines = append(lines, PlanLine{Cols: []string{"migration-content", mid, "sha256:" + sha, fmt.Sprintf("%d", lc)}})
	return lines, nil
}

func buildPromptMigrationLine(in PlanEmitInput, full, fname, mid string) (PlanLine, error) {
	data, err := os.ReadFile(full)
	if err != nil {
		return PlanLine{}, err
	}
	scope := ""
	risk := "medium"
	inFM := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "---" {
			inFM = !inFM
			if !inFM {
				break
			}
			continue
		}
		if inFM {
			if strings.HasPrefix(line, "scope_glob:") {
				v := strings.TrimSpace(strings.TrimPrefix(line, "scope_glob:"))
				v = strings.Trim(v, `"'`)
				scope = v
			} else if strings.HasPrefix(line, "risk:") {
				risk = strings.TrimSpace(strings.TrimPrefix(line, "risk:"))
			}
		}
	}
	matched := 0
	if scope != "" {
		_, files, err := listFilesUnderTree(in.UserTree)
		if err != nil {
			return PlanLine{}, err
		}
		for _, f := range files {
			if fnmatch(scope, f) {
				matched++
			}
		}
	}
	return PlanLine{Cols: []string{
		"migration-prompt", mid, "migrations/" + fname, scope,
		fmt.Sprintf("%d", matched), risk,
	}}, nil
}

func emitUserDeleted(in PlanEmitInput) ([]PlanLine, error) {
	if in.Provenance == "" || !isFile(in.Provenance) {
		return nil, nil
	}
	prov, err := LoadProvenance(in.Provenance)
	if err != nil {
		return nil, err
	}
	var lines []PlanLine
	for _, d := range prov.Deleted {
		if d.Path != "" {
			lines = append(lines, PlanLine{Cols: []string{"user-deleted", d.Path, d.Reason}})
		}
	}
	return lines, nil
}

func emitBootstrapSteps(in PlanEmitInput) ([]PlanLine, error) {
	bsMD := filepath.Join(in.NewTree, "BOOTSTRAP.md")
	if !isFile(bsMD) {
		return nil, nil
	}
	data, err := os.ReadFile(bsMD)
	if err != nil {
		return nil, err
	}
	bodies := ParseBootstrapBodies(string(data))
	ordered := in.Manifest.Bootstrap.OrderedSteps
	dangerous := map[string]bool{}
	for _, id := range in.Manifest.Bootstrap.Dangerous.IDs {
		dangerous[id] = true
	}

	pinSteps := map[string]ProvenanceStep{}
	if in.Provenance != "" && isFile(in.Provenance) {
		prov, err := LoadProvenance(in.Provenance)
		if err != nil {
			return nil, err
		}
		for _, s := range prov.BootstrapStepsDone {
			pinSteps[s.ID] = s
		}
	}

	var lines []PlanLine
	for _, sid := range ordered {
		body, ok := bodies[sid]
		if !ok {
			continue
		}
		norm := normalizeWhitespace(body)
		sum := sha256.Sum256([]byte(norm))
		newHash := "sha256:" + hex.EncodeToString(sum[:])
		existing, hasExisting := pinSteps[sid]
		if !hasExisting {
			if dangerous[sid] {
				lines = append(lines, PlanLine{Cols: []string{"bootstrap-step-dangerous", sid, "marked-dangerous-new"}})
			} else {
				lines = append(lines, PlanLine{Cols: []string{"bootstrap-step-new", sid}})
			}
			continue
		}
		if existing.ContentHash == newHash && existing.Status == "applied" {
			continue
		}
		if dangerous[sid] {
			lines = append(lines, PlanLine{Cols: []string{"bootstrap-step-dangerous", sid, "marked-dangerous-changed"}})
		} else {
			lines = append(lines, PlanLine{Cols: []string{"bootstrap-step-content-changed", sid, existing.ContentHash, newHash}})
		}
	}
	return lines, nil
}
