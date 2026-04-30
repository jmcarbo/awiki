package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"awiki/internal/adapters"
)

// execLookPath is a thin alias so tests can stub PATH lookups.
var execLookPath = exec.LookPath

// === install-hooks ===

// InstallHooks writes .git/hooks/pre-commit. Mirrors scripts/install-hooks.sh
// (a 12-line idempotent overwrite — the bash form rewrites unconditionally).
func InstallHooks(repoRoot string, stdout, stderr io.Writer) int {
	hookDir := filepath.Join(repoRoot, ".git", "hooks")
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "install-hooks: %v\n", err)
		return 1
	}
	hook := filepath.Join(hookDir, "pre-commit")
	body := "#!/usr/bin/env bash\nset -e\njust lint\n"
	if err := os.WriteFile(hook, []byte(body), 0o755); err != nil {
		fmt.Fprintf(stderr, "install-hooks: %v\n", err)
		return 1
	}
	if err := os.Chmod(hook, 0o755); err != nil {
		fmt.Fprintf(stderr, "install-hooks: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "INSTALLED|%s\n", hook)
	return 0
}

// InstallHooksCLI parses argv and dispatches InstallHooks.
func InstallHooksCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: awiki install-hooks")
		return 2
	}
	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	return InstallHooks(repoRoot, stdout, stderr)
}

// === install-qmd ===

// InstallQmdOptions configures `awiki install-qmd`. Runner is the optional
// adapter used to shell `command`/`git`/`cargo` — defaults to ExecRunner{}.
type InstallQmdOptions struct {
	RepoRoot string
	Runner   adapters.Runner
	Home     string // override $HOME (tests)
}

// InstallQmd ports scripts/install-qmd.sh: detect existing qmd on PATH,
// otherwise clone github.com/qntx-labs/qmd, build, symlink. Side-effects:
// writes .awiki/qmd-status (ok|missing) per spec.
func InstallQmd(opts InstallQmdOptions, stdout, stderr io.Writer) int {
	if opts.RepoRoot == "" {
		opts.RepoRoot, _ = os.Getwd()
	}
	if opts.Runner == nil {
		opts.Runner = adapters.ExecRunner{}
	}
	if opts.Home == "" {
		opts.Home = os.Getenv("HOME")
	}
	statusFile := filepath.Join(opts.RepoRoot, ".awiki", "qmd-status")
	_ = os.MkdirAll(filepath.Dir(statusFile), 0o755)

	// 1. Already installed?
	if path := lookupOnPath("qmd"); path != "" {
		fmt.Fprintf(stdout, "qmd already installed at: %s\n", path)
		_ = os.WriteFile(statusFile, []byte("ok\n"), 0o644)
		return 0
	}

	// 2. cargo present?
	if lookupOnPath("cargo") == "" {
		_ = os.WriteFile(statusFile, []byte("missing\n"), 0o644)
		fmt.Fprintln(stderr, "cargo not found; install Rust (https://rustup.rs) and re-run.")
		fmt.Fprintln(stderr, "agent will use grep fallback")
		return 1
	}

	// 3. clone or pull.
	xdg := os.Getenv("XDG_DATA_HOME")
	if xdg == "" {
		xdg = filepath.Join(opts.Home, ".local", "share")
	}
	srcDir := filepath.Join(xdg, "qmd-src")
	_ = os.MkdirAll(filepath.Dir(srcDir), 0o755)
	gitDir := filepath.Join(srcDir, ".git")
	ctx := context.Background()
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		out, code, err := opts.Runner.Run(ctx, "git",
			"clone", "https://github.com/qntx-labs/qmd", srcDir)
		if err != nil || code != 0 {
			_ = os.WriteFile(statusFile, []byte("missing\n"), 0o644)
			fmt.Fprintf(stderr, "qmd clone failed: %v\n%s", err, out)
			return 1
		}
	} else {
		out, code, err := opts.Runner.RunInDir(ctx, srcDir, "git", "pull", "--ff-only")
		if err != nil || code != 0 {
			fmt.Fprintf(stderr, "qmd git pull failed (continuing): %v\n%s", err, out)
		}
	}

	// 4. build.
	out, code, err := opts.Runner.RunInDir(ctx, srcDir, "cargo", "build", "--workspace", "--release")
	if err != nil || code != 0 {
		_ = os.WriteFile(statusFile, []byte("missing\n"), 0o644)
		fmt.Fprintf(stderr, "qmd build failed; agent will use grep fallback\n%s", out)
		return 1
	}

	// 5. symlink.
	binSrc := filepath.Join(srcDir, "target", "release", "qmd")
	if _, err := os.Stat(binSrc); err != nil {
		_ = os.WriteFile(statusFile, []byte("missing\n"), 0o644)
		fmt.Fprintf(stderr, "qmd binary not found at %s after build; agent will use grep fallback\n", binSrc)
		return 1
	}
	binDir := filepath.Join(opts.Home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "install-qmd: %v\n", err)
		return 1
	}
	link := filepath.Join(binDir, "qmd")
	_ = os.Remove(link)
	if err := os.Symlink(binSrc, link); err != nil {
		fmt.Fprintf(stderr, "install-qmd: symlink: %v\n", err)
		return 1
	}

	if lookupOnPath("qmd") == "" {
		fmt.Fprintln(stdout, "Add ~/.local/bin to PATH:")
		fmt.Fprintln(stdout, `  export PATH="$HOME/.local/bin:$PATH"`)
	}

	_ = os.WriteFile(statusFile, []byte("ok\n"), 0o644)
	fmt.Fprintf(stdout, "qmd installed at %s\n", link)
	return 0
}

// InstallQmdCLI parses argv and dispatches InstallQmd.
func InstallQmdCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: awiki install-qmd")
		return 2
	}
	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	return InstallQmd(InstallQmdOptions{RepoRoot: repoRoot}, stdout, stderr)
}

// lookupOnPath is a thin wrapper over exec.LookPath that returns "" on miss.
func lookupOnPath(name string) string {
	// Use os/exec.LookPath without importing os/exec at the top of this
	// file? We need the binary; Go's stdlib LookPath is the canonical form.
	path, err := execLookPath(name)
	if err != nil {
		return ""
	}
	return path
}

// === wire-awiki-mcp ===

// WireAwikiMCPOptions configures `awiki wire-awiki-mcp`.
type WireAwikiMCPOptions struct {
	RepoRoot string
}

// WireAwikiMCP ports scripts/wire-awiki-mcp.sh: detect Claude or Codex
// harness and register the awiki MCP server entry pointing at
// mcp/awiki-server/index.js.
func WireAwikiMCP(opts WireAwikiMCPOptions, stdout, stderr io.Writer) int {
	if opts.RepoRoot == "" {
		opts.RepoRoot, _ = os.Getwd()
	}
	indexJS := filepath.Join(opts.RepoRoot, "mcp", "awiki-server", "index.js")
	if _, err := os.Stat(indexJS); os.IsNotExist(err) {
		fmt.Fprintln(stderr, "MCP server not built. Run: cd mcp/awiki-server && npm install")
		return 1
	}
	target := detectHarness(opts.RepoRoot)
	switch target {
	case "claude":
		cfg := filepath.Join(opts.RepoRoot, ".mcp.json")
		if err := mergeClaudeMCP(cfg, "awiki", map[string]any{
			"command": "node",
			"args":    []string{indexJS},
		}); err != nil {
			fmt.Fprintf(stderr, "wire-awiki-mcp: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "WIRED|claude|%s\n", "./.mcp.json")
		return 0
	case "codex":
		cfg := filepath.Join(opts.RepoRoot, ".codex", "config.toml")
		if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
			fmt.Fprintf(stderr, "wire-awiki-mcp: %v\n", err)
			return 1
		}
		entry := fmt.Sprintf(`
[mcp.servers.awiki]
command = "node"
args = ["%s"]
`, indexJS)
		if err := appendIfMissing(cfg, "[mcp.servers.awiki]", entry); err != nil {
			fmt.Fprintf(stderr, "wire-awiki-mcp: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "WIRED|codex|%s\n", cfg)
		return 0
	}
	fmt.Fprintln(stderr, "No agent harness detected (.claude or .codex). Run from a configured project.")
	return 1
}

// WireAwikiMCPCLI parses argv and dispatches WireAwikiMCP.
func WireAwikiMCPCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: awiki wire-awiki-mcp")
		return 2
	}
	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	return WireAwikiMCP(WireAwikiMCPOptions{RepoRoot: repoRoot}, stdout, stderr)
}

// === wire-qmd-mcp ===

// WireQmdMCP ports scripts/wire-qmd-mcp.sh.
func WireQmdMCP(repoRoot string, stdout, stderr io.Writer) int {
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	if lookupOnPath("qmd") == "" {
		fmt.Fprintln(stderr, "qmd not installed; cannot wire MCP server")
		return 1
	}
	target := detectHarness(repoRoot)
	switch target {
	case "claude":
		cfg := filepath.Join(repoRoot, ".mcp.json")
		if err := mergeClaudeMCP(cfg, "qmd", map[string]any{
			"command": "qmd",
			"args":    []string{"mcp", "--root", "content/"},
		}); err != nil {
			fmt.Fprintf(stderr, "wire-qmd-mcp: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "WIRED|claude|%s\n", "./.mcp.json")
		return 0
	case "codex":
		cfg := filepath.Join(repoRoot, ".codex", "config.toml")
		if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
			fmt.Fprintf(stderr, "wire-qmd-mcp: %v\n", err)
			return 1
		}
		entry := `
[mcp.servers.qmd]
command = "qmd"
args = ["mcp", "--root", "content/"]
`
		if err := appendIfMissing(cfg, "[mcp.servers.qmd]", entry); err != nil {
			fmt.Fprintf(stderr, "wire-qmd-mcp: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "WIRED|codex|%s\n", cfg)
		return 0
	}
	fmt.Fprintln(stderr, "No agent harness detected (.claude or .codex). Run from a configured project.")
	return 1
}

// WireQmdMCPCLI parses argv and dispatches WireQmdMCP.
func WireQmdMCPCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "usage: awiki wire-qmd-mcp")
		return 2
	}
	repoRoot := os.Getenv("AWIKI_REPO_ROOT")
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	return WireQmdMCP(repoRoot, stdout, stderr)
}

// === harness detection / mcp config helpers ===

// detectHarness mirrors the bash precedence: codex wins when both .claude
// and .codex are present.
func detectHarness(repoRoot string) string {
	target := ""
	if _, err := os.Stat(filepath.Join(repoRoot, ".claude")); err == nil {
		target = "claude"
	} else if _, err := os.Stat(filepath.Join(repoRoot, "CLAUDE.md")); err == nil {
		target = "claude"
	}
	if _, err := os.Stat(filepath.Join(repoRoot, ".codex")); err == nil {
		target = "codex"
	}
	return target
}

// mergeClaudeMCP reads .mcp.json (creating an empty {"mcpServers": {}} if
// missing), inserts/overwrites mcpServers.<name>, and writes back.
func mergeClaudeMCP(path, name string, entry map[string]any) error {
	data := map[string]any{"mcpServers": map[string]any{}}
	if body, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(body, &data)
	}
	servers, ok := data["mcpServers"].(map[string]any)
	if !ok {
		servers = map[string]any{}
	}
	servers[name] = entry
	data["mcpServers"] = servers
	body, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

// appendIfMissing appends <entry> to <path> when <marker> is not already
// present in the file. Creates the file if missing.
func appendIfMissing(path, marker, entry string) error {
	if data, err := os.ReadFile(path); err == nil {
		if strings.Contains(string(data), marker) {
			return nil
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(entry)
	return err
}
