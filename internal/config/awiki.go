// Package config loads the awiki .awiki/config file (shell-style
// KEY=value with `#` comments).
package config

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"strings"
)

// Load reads path and returns its key/value pairs. A missing file
// returns an empty map without error (matches `source -f` semantics
// in the bash scripts that tolerate an absent config).
func Load(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	defer f.Close()
	out := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Inline comment: split on the first ` #` (space-hash).
		if i := strings.Index(line, " #"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		out[key] = val
	}
	return out, scanner.Err()
}
