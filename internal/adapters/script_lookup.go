package adapters

import (
	"os"
	"os/exec"
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

// ResolveAwikiBin returns the path of the awiki binary so a Go caller
// can shell into another verb without recursion-by-package-import.
//
// Resolution order, returning the first hit:
//
//  1. <repoRoot>/bin/awiki                  (source-tree convention)
//  2. os.Executable()                       (the running binary itself)
//  3. exec.LookPath("awiki")                (PATH fallback)
//  4. literal "awiki"                       (bare name; PATH lookup at exec time)
//
// The bare-name fallback keeps callers in unit tests that chdir to a
// fixture root working — the missing-binary error surfaces via the
// `exec` syscall with a clear message instead of a panic here.
func ResolveAwikiBin(repoRoot string) string {
	cand := filepath.Join(repoRoot, "bin", "awiki")
	if _, err := os.Stat(cand); err == nil {
		if abs, err := filepath.Abs(cand); err == nil {
			return abs
		}
		return cand
	}
	if exe, err := os.Executable(); err == nil {
		if abs, err := filepath.EvalSymlinks(exe); err == nil {
			return abs
		}
		return exe
	}
	if path, err := exec.LookPath("awiki"); err == nil {
		return path
	}
	return "awiki"
}
