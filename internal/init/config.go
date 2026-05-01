package initverb

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// LoadConfig reads a YAML-ish config file. To avoid pulling a YAML
// dependency for one verb, we accept a deliberately constrained subset
// — flat `key: value` lines, `# ...` comments, no nesting, no flow
// style. That covers the schema documented in BOOTSTRAP.md / the spec
// for `awiki init --config`. Unknown keys are reported as errors so
// typos surface immediately.
func LoadConfig(path string) (Answers, string, error) {
	var ans Answers
	agent := ""
	f, err := os.Open(path)
	if err != nil {
		return ans, "", fmt.Errorf("open config: %w", err)
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			return ans, "", fmt.Errorf("config %s:%d: malformed line %q", path, lineNo, raw)
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// Strip an inline `# comment` suffix that comes after the value.
		if hash := strings.Index(val, " #"); hash >= 0 {
			val = strings.TrimSpace(val[:hash])
		}
		// Strip surrounding quotes if present.
		if len(val) >= 2 {
			c0, cN := val[0], val[len(val)-1]
			if (c0 == '"' && cN == '"') || (c0 == '\'' && cN == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if err := applyConfigKV(&ans, &agent, key, val); err != nil {
			return ans, "", fmt.Errorf("config %s:%d: %w", path, lineNo, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return ans, "", err
	}
	return ans, agent, nil
}

// applyConfigKV maps one key into the Answers struct (or the side-band
// agent string).
func applyConfigKV(a *Answers, agent *string, key, val string) error {
	switch key {
	case "domain":
		a.Domain = val
	case "wiki_name":
		a.WikiName = val
	case "purpose":
		a.Purpose = val
	case "privacy":
		a.Privacy = val
	case "track_processed":
		b, err := parseBool(val)
		if err != nil {
			return err
		}
		a.TrackProcessed = b
	case "theme":
		a.Theme = val
	case "publish_log":
		b, err := parseBool(val)
		if err != nil {
			return err
		}
		a.PublishLog = b
	case "wire_qmd_mcp":
		b, err := parseBool(val)
		if err != nil {
			return err
		}
		a.WireQmdMCP = b
	case "wire_awiki_mcp":
		b, err := parseBool(val)
		if err != nil {
			return err
		}
		a.WireAwikiMCP = b
	case "stage_commit":
		b, err := parseBool(val)
		if err != nil {
			return err
		}
		a.StageCommit = b
	case "agent":
		*agent = val
	default:
		return fmt.Errorf("unknown key %q", key)
	}
	return nil
}

// parseBool accepts the YAML 1.1 truthy literals + the conventional
// y/n shortcuts, since users hand-edit this file.
func parseBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true", "yes", "y", "on", "1":
		return true, nil
	case "false", "no", "n", "off", "0", "":
		return false, nil
	}
	if b, err := strconv.ParseBool(s); err == nil {
		return b, nil
	}
	return false, fmt.Errorf("invalid bool %q", s)
}
