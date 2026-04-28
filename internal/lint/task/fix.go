package task

import (
	"hash/fnv"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func ApplyFixes(opts Options) ([]FixRecord, error) {
	scan, err := ScanContent(opts.ContentDir)
	if err != nil {
		return nil, err
	}
	skip := map[string]bool{}
	for _, continuation := range scan.Continuations {
		skip[continuation.File] = true
	}
	used := map[string]bool{}
	for _, action := range scan.Actions {
		if action.ID != "" {
			used[action.ID] = true
		}
	}
	var fixes []FixRecord
	err = filepath.WalkDir(opts.ContentDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		rel, _ := filepath.Rel(opts.ContentDir, path)
		rel = filepath.ToSlash(rel)
		if skip[rel] {
			return nil
		}
		changed, err := fixOneFile(path, rel, today(opts).Format("2006-01-02"), used)
		if err != nil {
			return err
		}
		if changed {
			fixes = append(fixes, FixRecord{File: path, Message: "normalized task actions"})
		}
		return nil
	})
	return fixes, err
}

func fixOneFile(path, rel, today string, used map[string]bool) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	lines := strings.Split(string(data), "\n")
	inFM := false
	fmDone := false
	inFence := false
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" && !fmDone {
			if !inFM {
				inFM = true
			} else {
				inFM = false
				fmDone = true
			}
			continue
		}
		if inFM {
			continue
		}
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence || strings.HasPrefix(trimmed, ">") || !actionLinePattern.MatchString(line) {
			continue
		}
		next := fixActionLine(line, rel, i+1, today, used)
		if next != line {
			lines[i] = next
			changed = true
		}
	}
	if !changed {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func fixActionLine(line, rel string, lineNo int, today string, used map[string]bool) string {
	action, ok := ParseActionLine(line)
	if !ok || action.BadStatus != "" || action.HasBadID || action.BadKey != "" {
		return line
	}
	prefix := actionPrefix(line)
	tokens := strings.Fields(strings.TrimPrefix(line, prefix))
	var text, tail []string
	ctx := ""
	id := ""
	status := action.Status
	for _, tok := range tokens {
		switch {
		case contextPattern.MatchString(tok) && ctx == "":
			ctx = tok
		case blockIDPattern.MatchString(tok):
			id = tok
		case tokenKeyPattern.MatchString(tok):
			key, val, _ := strings.Cut(tok, ":")
			val = normalizeDateToken(val)
			if key == "done" || key == "since" {
				// emitted in canonical order below
			}
			action.TailKeys[key] = val
		default:
			if ctx == "" && id == "" && len(action.TailKeys) == 0 {
				text = append(text, tok)
			}
		}
	}
	if status == "x" && action.TailKeys["done"] == "" {
		action.TailKeys["done"] = today
	}
	if status == "?" && action.TailKeys["since"] == "" {
		action.TailKeys["since"] = today
	}
	for _, key := range []string{"due", "defer", "wait", "since", "every", "priority", "est", "done"} {
		if val := action.TailKeys[key]; val != "" {
			tail = append(tail, key+":"+val)
		}
	}
	if id == "" {
		id = "^" + mintID(rel, lineNo, used)
	}
	parts := append([]string{}, text...)
	if ctx != "" {
		parts = append(parts, ctx)
	}
	parts = append(parts, tail...)
	if id != "" {
		parts = append(parts, id)
	}
	if len(parts) == 0 {
		return line
	}
	return prefix + strings.Join(parts, " ")
}

var prefixPattern = regexp.MustCompile(`^\s*[-*]\s+\[.\]\s+`)

func actionPrefix(line string) string {
	return prefixPattern.FindString(line)
}

func mintID(rel string, lineNo int, used map[string]bool) string {
	alphabet := "abcdefghijklmnopqrstuvwxyz0123456789"
	for salt := 0; salt < 100; salt++ {
		h := fnv.New64a()
		h.Write([]byte(rel))
		h.Write([]byte{0})
		h.Write([]byte(string(rune(lineNo))))
		h.Write([]byte{byte(salt)})
		n := h.Sum64()
		var b strings.Builder
		for i := 0; i < 8; i++ {
			b.WriteByte(alphabet[n%uint64(len(alphabet))])
			n /= uint64(len(alphabet))
		}
		id := b.String()
		if !used[id] {
			used[id] = true
			return id
		}
	}
	return "taskfix1"
}
