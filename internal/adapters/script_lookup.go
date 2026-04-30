package adapters

import (
	"os"
	"path/filepath"
)

// resolveScript locates a bash script under the awiki scripts/ tree.
// Bash callers use `SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"`
// at script load time so `$SCRIPT_DIR/<name>` resolves to the actual
// scripts/ folder regardless of the caller's CWD. The Go binary lacks
// that anchor — it has only `repoRoot` (the data root, often
// `AWIKI_REPO_ROOT`) and the binary's own location.
//
// Resolution order, returning the first existing path:
//
//  1. <repoRoot>/scripts/<name>
//  2. <binDir>/../scripts/<name>          (binary's source-tree sibling)
//  3. scripts/<name>                       (relative to CWD)
//
// The fallback is a bare relative path so callers in unit tests that
// chdir to a fixture root keep working. exec(3) will surface a clear
// "no such file" error if none of the above resolve.
func resolveScript(repoRoot, name string) string {
	cand := filepath.Join(repoRoot, "scripts", name)
	if _, err := os.Stat(cand); err == nil {
		return cand
	}
	if exe, err := os.Executable(); err == nil {
		if abs, err := filepath.EvalSymlinks(exe); err == nil {
			binDir := filepath.Dir(abs)
			cand2 := filepath.Join(binDir, "..", "scripts", name)
			if _, err := os.Stat(cand2); err == nil {
				return cand2
			}
		}
	}
	return filepath.Join("scripts", name)
}
