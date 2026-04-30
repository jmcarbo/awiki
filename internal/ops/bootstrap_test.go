package ops

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallHooksWritesPreCommit(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".git", "hooks"))
	var stdout bytes.Buffer
	rc := InstallHooks(dir, &stdout, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	body := mustRead(t, filepath.Join(dir, ".git", "hooks", "pre-commit"))
	if !strings.Contains(body, "just lint") {
		t.Fatalf("hook missing just lint: %s", body)
	}
	info, err := os.Stat(filepath.Join(dir, ".git", "hooks", "pre-commit"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("hook not executable: %v", info.Mode())
	}
	if !strings.Contains(stdout.String(), "INSTALLED|") {
		t.Fatalf("missing INSTALLED record: %s", stdout.String())
	}
}

func TestInstallQmdWritesStatusOkWhenAlreadyOnPath(t *testing.T) {
	// Stub execLookPath to simulate qmd already present.
	dir := t.TempDir()
	prev := execLookPath
	execLookPath = func(name string) (string, error) {
		if name == "qmd" {
			return "/usr/local/bin/qmd", nil
		}
		return prev(name)
	}
	t.Cleanup(func() { execLookPath = prev })
	rc := InstallQmd(InstallQmdOptions{RepoRoot: dir, Home: dir}, &bytes.Buffer{}, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	got := strings.TrimSpace(mustRead(t, filepath.Join(dir, ".awiki", "qmd-status")))
	if got != "ok" {
		t.Fatalf("status=%q want ok", got)
	}
}

func TestInstallQmdWritesStatusMissingWhenCargoUnavailable(t *testing.T) {
	dir := t.TempDir()
	prev := execLookPath
	execLookPath = func(name string) (string, error) {
		// Both qmd and cargo missing.
		return "", os.ErrNotExist
	}
	t.Cleanup(func() { execLookPath = prev })
	var stderr bytes.Buffer
	rc := InstallQmd(InstallQmdOptions{RepoRoot: dir, Home: dir}, &bytes.Buffer{}, &stderr)
	if rc != 1 {
		t.Fatalf("rc=%d want 1", rc)
	}
	got := strings.TrimSpace(mustRead(t, filepath.Join(dir, ".awiki", "qmd-status")))
	if got != "missing" {
		t.Fatalf("status=%q want missing", got)
	}
	if !strings.Contains(stderr.String(), "cargo not found") {
		t.Fatalf("missing diag: %s", stderr.String())
	}
}

func TestWireAwikiMCPRequiresMCPServer(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".claude"))
	var stderr bytes.Buffer
	rc := WireAwikiMCP(WireAwikiMCPOptions{RepoRoot: dir}, &bytes.Buffer{}, &stderr)
	if rc != 1 {
		t.Fatalf("rc=%d want 1", rc)
	}
	if !strings.Contains(stderr.String(), "MCP server not built") {
		t.Fatalf("missing diag: %s", stderr.String())
	}
}

func TestWireAwikiMCPClaudeWritesMcpJSON(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".claude"))
	mustMkdir(t, filepath.Join(dir, "mcp", "awiki-server"))
	mustWrite(t, filepath.Join(dir, "mcp", "awiki-server", "index.js"), "// stub\n")
	var stdout bytes.Buffer
	rc := WireAwikiMCP(WireAwikiMCPOptions{RepoRoot: dir}, &stdout, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	body := mustRead(t, filepath.Join(dir, ".mcp.json"))
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, body)
	}
	servers, ok := parsed["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("missing mcpServers: %s", body)
	}
	awiki, ok := servers["awiki"].(map[string]any)
	if !ok {
		t.Fatalf("missing awiki entry: %s", body)
	}
	if awiki["command"] != "node" {
		t.Fatalf("wrong command: %v", awiki["command"])
	}
	if !strings.Contains(stdout.String(), "WIRED|claude") {
		t.Fatalf("missing WIRED record: %s", stdout.String())
	}
}

func TestWireAwikiMCPCodexAppendsToTOML(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".codex"))
	mustMkdir(t, filepath.Join(dir, "mcp", "awiki-server"))
	mustWrite(t, filepath.Join(dir, "mcp", "awiki-server", "index.js"), "// stub\n")
	var stdout bytes.Buffer
	rc := WireAwikiMCP(WireAwikiMCPOptions{RepoRoot: dir}, &stdout, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	body := mustRead(t, filepath.Join(dir, ".codex", "config.toml"))
	if !strings.Contains(body, "[mcp.servers.awiki]") {
		t.Fatalf("missing section: %s", body)
	}
	// Idempotent — second run does not duplicate.
	WireAwikiMCP(WireAwikiMCPOptions{RepoRoot: dir}, &bytes.Buffer{}, &bytes.Buffer{})
	body2 := mustRead(t, filepath.Join(dir, ".codex", "config.toml"))
	if strings.Count(body2, "[mcp.servers.awiki]") != 1 {
		t.Fatalf("duplicated entry: %s", body2)
	}
	if !strings.Contains(stdout.String(), "WIRED|codex") {
		t.Fatalf("missing record: %s", stdout.String())
	}
}

func TestWireQmdMCPRequiresQmdOnPath(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".claude"))
	prev := execLookPath
	execLookPath = func(name string) (string, error) { return "", os.ErrNotExist }
	t.Cleanup(func() { execLookPath = prev })
	var stderr bytes.Buffer
	rc := WireQmdMCP(dir, &bytes.Buffer{}, &stderr)
	if rc != 1 {
		t.Fatalf("rc=%d want 1", rc)
	}
	if !strings.Contains(stderr.String(), "qmd not installed") {
		t.Fatalf("missing diag: %s", stderr.String())
	}
}

func TestWireQmdMCPClaudeAppendsServer(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".claude"))
	prev := execLookPath
	execLookPath = func(name string) (string, error) {
		if name == "qmd" {
			return "/usr/local/bin/qmd", nil
		}
		return "", os.ErrNotExist
	}
	t.Cleanup(func() { execLookPath = prev })
	rc := WireQmdMCP(dir, &bytes.Buffer{}, &bytes.Buffer{})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	body := mustRead(t, filepath.Join(dir, ".mcp.json"))
	if !strings.Contains(body, `"qmd"`) {
		t.Fatalf("qmd entry missing: %s", body)
	}
}

func TestDetectHarnessCodexWinsOverClaude(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".claude"))
	mustMkdir(t, filepath.Join(dir, ".codex"))
	if got := detectHarness(dir); got != "codex" {
		t.Fatalf("got=%q want codex", got)
	}
}

func TestDetectHarnessClaudeMdAlone(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "CLAUDE.md"), "# claude\n")
	if got := detectHarness(dir); got != "claude" {
		t.Fatalf("got=%q want claude", got)
	}
}
