#!/usr/bin/env bash
set -euo pipefail

# wire-qmd-mcp.sh — register qmd as an MCP server for the local agent
# harness (Claude Code or Codex). Detects which harness is configured
# in the project root; if both are present, codex wins (last-write).
#
# Notes:
# - Claude Code reads project-level MCP servers from `./.mcp.json`
#   at the repository root.
# - Codex reads `./.codex/config.toml` (project-local).
# - The `qntx-labs/qmd` v0.5.0 CLI does not yet expose an `mcp`
#   subcommand (see docs/decisions/qmd-install.md). The wiring written
#   here matches the spec (`qmd mcp --root content/`) so it starts
#   working as soon as upstream ships qmd-mcp. Until then, the harness
#   will simply fail to start the server — that is non-fatal for awiki.

if ! command -v qmd >/dev/null 2>&1; then
  echo "qmd not installed; cannot wire MCP server" >&2
  exit 1
fi

# Detect harness — last write wins, so codex takes precedence when both
# are present (matches the spec's documented preference).
TARGET=""
if [[ -d .claude || -f CLAUDE.md ]]; then TARGET="claude"; fi
if [[ -d .codex ]]; then TARGET="codex"; fi

case "$TARGET" in
  claude)
    CONFIG="./.mcp.json"
    if [[ ! -f "$CONFIG" ]]; then echo '{"mcpServers": {}}' > "$CONFIG"; fi
    python3 - "$CONFIG" <<'PY'
import json, sys
path = sys.argv[1]
data = json.load(open(path))
data.setdefault("mcpServers", {})["qmd"] = {
    "command": "qmd",
    "args": ["mcp", "--root", "content/"],
}
json.dump(data, open(path, "w"), indent=2)
PY
    echo "WIRED|claude|$CONFIG"
    exit 0
    ;;
  codex)
    CONFIG=".codex/config.toml"
    mkdir -p .codex
    if ! grep -q '^\[mcp.servers.qmd\]' "$CONFIG" 2>/dev/null; then
      cat >> "$CONFIG" <<'TOML'

[mcp.servers.qmd]
command = "qmd"
args = ["mcp", "--root", "content/"]
TOML
    fi
    echo "WIRED|codex|$CONFIG"
    exit 0
    ;;
  *)
    echo "No agent harness detected (.claude or .codex). Run from a configured project." >&2
    exit 1
    ;;
esac
